package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"localaz.dev/internal/servers/sbserver"
)

// service describes one emulated Azure service mounted on its own HTTP listener.
// Every HTTP service is served over TLS so the Azure SDKs and CLI will attach
// credentials (they refuse to over plain HTTP); only Service Bus, which speaks
// AMQP with UseDevelopmentEmulator (plain TCP + anonymous SASL), stays cleartext.
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

// run starts every service and blocks until the context is cancelled or a
// listener fails, then gracefully shuts them all down. It returns the listener
// error (if any) so main can decide the process exit code; a serve failure on
// one listener no longer hard-kills the others via os.Exit.
func run(ctx context.Context, logger *slog.Logger, services []service, amqp amqpService, tlsCert *tls.Certificate) error {
	var wg sync.WaitGroup
	servers := make([]*http.Server, 0, len(services))

	// Bind the AMQP listener before starting any serve goroutine so a bind
	// failure here is returned cleanly without leaving HTTP servers running.
	amqpListener, err := net.Listen("tcp", amqp.addr)
	if err != nil {
		return fmt.Errorf("listen servicebus: %w", err)
	}

	// errc carries the first non-graceful serve error from any goroutine. It is
	// buffered so a goroutine never blocks on send, and we only read one value.
	errc := make(chan error, 1)
	reportErr := func(svc string, err error) {
		logger.Error("serve", "service", svc, "err", err)
		select {
		case errc <- fmt.Errorf("serve %s: %w", svc, err):
		default:
		}
	}

	for _, svc := range services {
		srv := &http.Server{
			Addr:              svc.addr,
			Handler:           logRequests(logger.With("service", svc.name), svc.handler),
			ReadHeaderTimeout: 30 * time.Second,
		}
		srv.TLSConfig = &tls.Config{Certificates: []tls.Certificate{*tlsCert}}
		servers = append(servers, srv)

		wg.Add(1)
		go func(svc service, srv *http.Server) {
			defer wg.Done()
			logger.Info("service listening", "service", svc.name, "addr", svc.addr, "tls", true)
			if err := srv.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
				reportErr(svc.name, err)
			}
		}(svc, srv)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("service listening", "service", "servicebus", "addr", amqp.addr)
		if err := amqp.server.Serve(amqpListener); err != nil && !errors.Is(err, net.ErrClosed) {
			reportErr("servicebus", err)
		}
	}()

	// Shut down on either a cancelled context or the first serve failure.
	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-errc:
	}

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
	return runErr
}
