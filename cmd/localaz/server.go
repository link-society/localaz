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
//
// Every listener is bound up front (so a bind failure is reported before the
// emulator is declared ready), the plain-HTTP management server is served first
// so it can answer 503 on its health endpoint during startup, and the
// management server is flipped to ready only once every other listener is bound
// and serving.
func run(ctx context.Context, logger *slog.Logger, services []service, amqp amqpService, mgmt *managementServer, tlsCert *tls.Certificate) error {
	var wg sync.WaitGroup

	// Bind every listener before serving so a bind failure is surfaced before
	// the emulator declares itself ready. Listeners are closed on early failure.
	var listeners []net.Listener
	closeListeners := func() {
		for _, ln := range listeners {
			_ = ln.Close()
		}
	}

	mgmtListener, err := net.Listen("tcp", mgmt.addr)
	if err != nil {
		return fmt.Errorf("listen management: %w", err)
	}
	listeners = append(listeners, mgmtListener)

	svcListeners := make([]net.Listener, len(services))
	for i, svc := range services {
		ln, err := net.Listen("tcp", svc.addr)
		if err != nil {
			closeListeners()
			return fmt.Errorf("listen %s: %w", svc.name, err)
		}
		svcListeners[i] = ln
		listeners = append(listeners, ln)
	}

	amqpListener, err := net.Listen("tcp", amqp.addr)
	if err != nil {
		closeListeners()
		return fmt.Errorf("listen servicebus: %w", err)
	}
	listeners = append(listeners, amqpListener)

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

	httpServers := make([]*http.Server, 0, len(services)+1)

	// Serve the management server first so its health endpoint can report 503
	// while the rest comes up; it stays plain HTTP so a probe needs no TLS
	// trust material.
	mgmtSrv := &http.Server{
		Handler:           logRequests(logger.With("service", "management"), mgmt.handler()),
		ReadHeaderTimeout: 30 * time.Second,
	}
	httpServers = append(httpServers, mgmtSrv)
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("service listening", "service", "management", "addr", mgmt.addr, "tls", false)
		if err := mgmtSrv.Serve(mgmtListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			reportErr("management", err)
		}
	}()

	for i, svc := range services {
		srv := &http.Server{
			Handler:           logRequests(logger.With("service", svc.name), svc.handler),
			ReadHeaderTimeout: 30 * time.Second,
			TLSConfig:         &tls.Config{Certificates: []tls.Certificate{*tlsCert}},
		}
		httpServers = append(httpServers, srv)

		wg.Add(1)
		go func(svc service, srv *http.Server, ln net.Listener) {
			defer wg.Done()
			logger.Info("service listening", "service", svc.name, "addr", svc.addr, "tls", true)
			if err := srv.ServeTLS(ln, "", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
				reportErr(svc.name, err)
			}
		}(svc, srv, svcListeners[i])
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("service listening", "service", "servicebus", "addr", amqp.addr)
		if err := amqp.server.Serve(amqpListener); err != nil && !errors.Is(err, net.ErrClosed) {
			reportErr("servicebus", err)
		}
	}()

	// Every listener is bound and serving: the emulator is ready, so the
	// management server's health endpoint flips from 503 to 200.
	mgmt.ready.Store(true)
	logger.Info("all services ready")

	// Shut down on either a cancelled context or the first serve failure.
	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-errc:
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, srv := range httpServers {
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "err", err)
		}
	}
	_ = amqpListener.Close()
	wg.Wait()
	logger.Info("stopped")
	return runErr
}
