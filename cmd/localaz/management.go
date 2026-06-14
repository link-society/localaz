package main

import (
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

// managementServer is the one listener served over plain HTTP (no TLS). It is
// not an emulated Azure service: it hosts cross-cutting management endpoints —
// today a readiness probe and the self-signed TLS material every other service
// is served with, and a natural home for future endpoints such as Prometheus
// metrics.
//
// The readiness probe reports 503 until every other listener is bound and
// serving, then 200, so a container HEALTHCHECK or a test harness can wait for
// the emulator to be ready. The certificate and its private key are served so
// clients can fetch the trust material they need to reach the TLS services. The
// key is a throwaway, self-signed development certificate, so exposing it here
// is a deliberate local-development convenience, not a production pattern.
type managementServer struct {
	addr    string
	ready   atomic.Bool
	certPEM []byte
	keyPEM  []byte
}

// newManagementServer builds a management server that serves the given
// certificate and key PEM. It starts not-ready; run flips it to ready once
// everything is up.
func newManagementServer(addr string, certPEM, keyPEM []byte) *managementServer {
	return &managementServer{addr: addr, certPEM: certPEM, keyPEM: keyPEM}
}

// handler builds the routes the management server exposes.
func (m *managementServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", m.handleHealth)
	mux.HandleFunc("GET /certs/pubkey", m.servePEM(m.certPEM))
	mux.HandleFunc("GET /certs/privkey", m.servePEM(m.keyPEM))
	return mux
}

// handleHealth answers 200 once the emulator is ready and 503 until then.
func (m *managementServer) handleHealth(w http.ResponseWriter, _ *http.Request) {
	if !m.ready.Load() {
		http.Error(w, "starting", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// servePEM returns a handler that serves the given PEM bytes, or 404 when the
// emulator has no PEM material to hand out.
func (m *managementServer) servePEM(pem []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if len(pem) == 0 {
			http.Error(w, "certificate not available", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/x-pem-file")
		_, _ = w.Write(pem)
	}
}

// runHealthcheck probes the management server's health endpoint at addr and
// returns a process exit code: 0 when the emulator reports ready, 1 otherwise.
// It backs the container HEALTHCHECK, which cannot rely on a shell or curl in
// the distroless image.
func runHealthcheck(addr string) int {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 1
	}
	if host == "" {
		host = "127.0.0.1"
	}
	url := fmt.Sprintf("http://%s/health", net.JoinHostPort(host, port))

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return 0
	}
	return 1
}
