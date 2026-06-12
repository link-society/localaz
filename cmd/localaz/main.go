// Command localaz runs the localaz Azure emulator. A single process exposes
// each emulated Azure service on its own listener (mirroring Azurite's
// multi-port layout) so that users only ever run one container.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"localaz.dev/internal/servers/aadserver"
	"localaz.dev/internal/servers/armserver"
	"localaz.dev/internal/servers/blobserver"
	"localaz.dev/internal/servers/egserver"
	"localaz.dev/internal/servers/monitorserver"
	"localaz.dev/internal/servers/queueserver"
	"localaz.dev/internal/servers/sbserver"
	"localaz.dev/internal/servers/tableserver"
	"localaz.dev/internal/servers/wpsserver"
	"localaz.dev/internal/stores/armstore"
	"localaz.dev/internal/stores/blobstore/fsstore"
	"localaz.dev/internal/stores/egstore"
	"localaz.dev/internal/stores/monitorstore"
	"localaz.dev/internal/stores/queuestore"
	"localaz.dev/internal/stores/sbstore"
	"localaz.dev/internal/stores/tablestore"
	"localaz.dev/internal/utils/devcert"
)

func main() {
	blobAddr := flag.String("addr", envOr("LOCALAZ_BLOB_ADDR", ":10000"), "blob service listen address")
	queueAddr := flag.String("queue-addr", envOr("LOCALAZ_QUEUE_ADDR", ":10001"), "queue service listen address")
	tableAddr := flag.String("table-addr", envOr("LOCALAZ_TABLE_ADDR", ":10002"), "table service listen address")
	eventGridAddr := flag.String("eventgrid-addr", envOr("LOCALAZ_EVENTGRID_ADDR", ":10003"), "event grid service listen address")
	webPubSubAddr := flag.String("webpubsub-addr", envOr("LOCALAZ_WEBPUBSUB_ADDR", ":10004"), "web pubsub service listen address")
	monitorAddr := flag.String("monitor-addr", envOr("LOCALAZ_MONITOR_ADDR", ":10005"), "monitor logs service listen address")
	aadAddr := flag.String("aad-addr", envOr("LOCALAZ_AAD_ADDR", ":10006"), "entra id (aad) service listen address")
	armAddr := flag.String("arm-addr", envOr("LOCALAZ_ARM_ADDR", ":10007"), "resource manager (arm) service listen address")
	serviceBusAddr := flag.String("servicebus-addr", envOr("LOCALAZ_SERVICEBUS_ADDR", ":5672"), "service bus AMQP listen address")
	dataDir := flag.String("data", envOr("LOCALAZ_DATA_DIR", "/data"), "directory for persisted state")
	cloudName := flag.String("arm-cloud-name", envOr("LOCALAZ_ARM_CLOUD_NAME", "localaz"), "cloud name advertised by the ARM metadata document")
	advertiseHost := flag.String("advertise-host", envOr("LOCALAZ_ADVERTISE_HOST", "127.0.0.1"), "host clients use to reach the control-plane services")
	tlsCertFile := flag.String("tls-cert", envOr("LOCALAZ_TLS_CERT", ""), "PEM certificate for the bearer/control-plane services")
	tlsKeyFile := flag.String("tls-key", envOr("LOCALAZ_TLS_KEY", ""), "PEM private key for the bearer/control-plane services")
	tlsAuto := flag.Bool("tls-auto", envOr("LOCALAZ_TLS_AUTO", "") != "", "generate a self-signed certificate for the bearer/control-plane services")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	tlsCert, err := loadTLS(logger, *dataDir, *tlsCertFile, *tlsKeyFile, *tlsAuto)
	fatal(logger, "init tls", err)
	blobStore, err := fsstore.New(*dataDir)
	fatal(logger, "init blob store", err)
	queueStore, err := queuestore.New(*dataDir)
	fatal(logger, "init queue store", err)
	tableStore, err := tablestore.New(*dataDir)
	fatal(logger, "init table store", err)
	aadServer, err := aadserver.New()
	fatal(logger, "init aad server", err)

	scheme := "http"
	if tlsCert != nil {
		scheme = "https"
	}
	armStore := armstore.New(armstore.Config{
		CloudName:            *cloudName,
		TenantID:             "adfs",
		LoginEndpoint:        controlURL(scheme, *advertiseHost, *aadAddr) + "/",
		ResourceManager:      controlURL(scheme, *advertiseHost, *armAddr),
		LogAnalyticsEndpoint: controlURL(scheme, *advertiseHost, *monitorAddr),
		StorageSuffix:        *advertiseHost,
	})

	services := []service{
		{name: "blob", addr: *blobAddr, handler: blobserver.New(blobStore)},
		{name: "queue", addr: *queueAddr, handler: queueserver.New(queueStore)},
		{name: "table", addr: *tableAddr, handler: tableserver.New(tableStore)},
		{name: "eventgrid", addr: *eventGridAddr, handler: egserver.New(egstore.New())},
		{name: "webpubsub", addr: *webPubSubAddr, handler: wpsserver.New()},
		{name: "monitor", addr: *monitorAddr, handler: monitorserver.New(monitorstore.New()), secure: true},
		{name: "aad", addr: *aadAddr, handler: aadServer, secure: true},
		{name: "arm", addr: *armAddr, handler: armserver.New(armStore), secure: true},
	}

	amqp := amqpService{addr: *serviceBusAddr, server: sbserver.New(sbstore.New())}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	run(ctx, logger, services, amqp, tlsCert)
}

// service describes one emulated Azure service mounted on its own HTTP listener.
type service struct {
	name    string
	addr    string
	handler http.Handler
	// secure marks services that carry bearer tokens (Entra ID, ARM, Monitor)
	// and therefore listen over TLS when certificate material is available.
	secure bool
}

// amqpService describes the Service Bus listener, which speaks raw AMQP over
// TCP rather than HTTP.
type amqpService struct {
	addr   string
	server *sbserver.Server
}

// run starts every service and blocks until the context is cancelled, then
// gracefully shuts them all down.
func run(ctx context.Context, logger *slog.Logger, services []service, amqp amqpService, tlsCert *tls.Certificate) {
	var wg sync.WaitGroup
	servers := make([]*http.Server, 0, len(services))

	for _, svc := range services {
		srv := &http.Server{
			Addr:              svc.addr,
			Handler:           logRequests(logger.With("service", svc.name), svc.handler),
			ReadHeaderTimeout: 30 * time.Second,
		}
		useTLS := svc.secure && tlsCert != nil
		if useTLS {
			srv.TLSConfig = &tls.Config{Certificates: []tls.Certificate{*tlsCert}}
		}
		servers = append(servers, srv)

		wg.Add(1)
		go func(svc service, srv *http.Server, useTLS bool) {
			defer wg.Done()
			logger.Info("service listening", "service", svc.name, "addr", svc.addr, "tls", useTLS)
			var err error
			if useTLS {
				err = srv.ListenAndServeTLS("", "")
			} else {
				err = srv.ListenAndServe()
			}
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Error("serve", "service", svc.name, "err", err)
				os.Exit(1)
			}
		}(svc, srv, useTLS)
	}

	amqpListener, err := net.Listen("tcp", amqp.addr)
	if err != nil {
		logger.Error("listen", "service", "servicebus", "err", err)
		os.Exit(1)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("service listening", "service", "servicebus", "addr", amqp.addr)
		if err := amqp.server.Serve(amqpListener); err != nil && !errors.Is(err, net.ErrClosed) {
			logger.Error("serve", "service", "servicebus", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, srv := range servers {
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "err", err)
		}
	}
	_ = amqpListener.Close()
	wg.Wait()
	logger.Info("stopped")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// fatal logs what and exits when err is non-nil. It collapses the repeated
// "initialise dependency or die" wiring in main.
func fatal(logger *slog.Logger, what string, err error) {
	if err != nil {
		logger.Error(what, "err", err)
		os.Exit(1)
	}
}

// loadTLS resolves the certificate used by the bearer/control-plane services.
// An explicit cert/key pair wins; otherwise -tls-auto generates a throwaway
// self-signed certificate and writes its PEM under <data>/tls so clients can
// trust it (for example via REQUESTS_CA_BUNDLE). Returns nil when TLS is off.
func loadTLS(logger *slog.Logger, dataDir, certFile, keyFile string, auto bool) (*tls.Certificate, error) {
	if certFile != "" && keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("load tls key pair: %w", err)
		}
		return &cert, nil
	}
	if !auto {
		return nil, nil
	}

	certPEM, keyPEM, err := devcert.Generate()
	if err != nil {
		return nil, err
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse generated certificate: %w", err)
	}

	tlsDir := filepath.Join(dataDir, "tls")
	if err := os.MkdirAll(tlsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create tls dir: %w", err)
	}
	certPath := filepath.Join(tlsDir, "localaz.crt")
	keyPath := filepath.Join(tlsDir, "localaz.key")
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return nil, fmt.Errorf("write certificate: %w", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return nil, fmt.Errorf("write key: %w", err)
	}
	logger.Info("generated self-signed certificate", "cert", certPath, "key", keyPath)
	return &cert, nil
}

// controlURL builds the externally reachable base URL of a control-plane
// service from the advertise host and the port portion of its listen address.
func controlURL(scheme, host, addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		port = "0"
	}
	return scheme + "://" + net.JoinHostPort(host, port)
}

// logRequests is a minimal access log so users can see SDK/CLI traffic hitting
// the emulator.
func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path+querySuffix(r),
			"duration", time.Since(start).Round(time.Millisecond),
		)
	})
}

func querySuffix(r *http.Request) string {
	if r.URL.RawQuery == "" {
		return ""
	}
	return "?" + r.URL.RawQuery
}
