// Command localaz runs the localaz Azure emulator. A single process exposes
// each emulated Azure service on its own listener (mirroring Azurite's
// multi-port layout) so that users only ever run one container.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"localaz.dev/internal/blobserver"
	"localaz.dev/internal/blobstore/fsstore"
	"localaz.dev/internal/egserver"
	"localaz.dev/internal/egstore"
	"localaz.dev/internal/sbserver"
	"localaz.dev/internal/sbstore"
	"localaz.dev/internal/wpsserver"
)

func main() {
	blobAddr := flag.String("addr", envOr("LOCALAZ_BLOB_ADDR", ":10000"), "blob service listen address")
	eventGridAddr := flag.String("eventgrid-addr", envOr("LOCALAZ_EVENTGRID_ADDR", ":10001"), "event grid service listen address")
	webPubSubAddr := flag.String("webpubsub-addr", envOr("LOCALAZ_WEBPUBSUB_ADDR", ":10002"), "web pubsub service listen address")
	serviceBusAddr := flag.String("servicebus-addr", envOr("LOCALAZ_SERVICEBUS_ADDR", ":5672"), "service bus AMQP listen address")
	dataDir := flag.String("data", envOr("LOCALAZ_DATA_DIR", "/data"), "directory for persisted state")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	blobStore, err := fsstore.New(*dataDir)
	if err != nil {
		logger.Error("init blob store", "err", err)
		os.Exit(1)
	}

	services := []service{
		{name: "blob", addr: *blobAddr, handler: blobserver.New(blobStore)},
		{name: "eventgrid", addr: *eventGridAddr, handler: egserver.New(egstore.New())},
		{name: "webpubsub", addr: *webPubSubAddr, handler: wpsserver.New()},
	}

	amqp := amqpService{addr: *serviceBusAddr, server: sbserver.New(sbstore.New())}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	run(ctx, logger, services, amqp)
}

// service describes one emulated Azure service mounted on its own HTTP listener.
type service struct {
	name    string
	addr    string
	handler http.Handler
}

// amqpService describes the Service Bus listener, which speaks raw AMQP over
// TCP rather than HTTP.
type amqpService struct {
	addr   string
	server *sbserver.Server
}

// run starts every service and blocks until the context is cancelled, then
// gracefully shuts them all down.
func run(ctx context.Context, logger *slog.Logger, services []service, amqp amqpService) {
	var wg sync.WaitGroup
	servers := make([]*http.Server, 0, len(services))

	for _, svc := range services {
		srv := &http.Server{
			Addr:              svc.addr,
			Handler:           logRequests(logger.With("service", svc.name), svc.handler),
			ReadHeaderTimeout: 30 * time.Second,
		}
		servers = append(servers, srv)

		wg.Add(1)
		go func(svc service, srv *http.Server) {
			defer wg.Done()
			logger.Info("service listening", "service", svc.name, "addr", svc.addr)
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Error("serve", "service", svc.name, "err", err)
				os.Exit(1)
			}
		}(svc, srv)
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
