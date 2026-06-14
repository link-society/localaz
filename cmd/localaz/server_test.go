package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"localaz.dev/internal/servers/sbserver"
	"localaz.dev/internal/stores/sbstore"
	"localaz.dev/internal/utils/devcert"
)

// TestRunReturnsErrorOnBindFailure guards the graceful-shutdown fix: when one
// listener fails to bind (its address is already in use), run must NOT hard-kill
// the process with os.Exit. Instead it must detect the bind failure, run the
// graceful-shutdown path for every other listener, and RETURN a non-nil error
// promptly. Before the fix run() never returns on a bind failure (it os.Exits),
// so this test would hang until the deadline and then fail — genuinely guarding
// the change.
func TestRunReturnsErrorOnBindFailure(t *testing.T) {
	// Occupy a TCP port and keep it open for the whole test so a later bind to
	// the same address fails with "address already in use".
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("occupy port: %v", err)
	}
	defer occupied.Close()
	busyAddr := occupied.Addr().String()

	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))

	// One HTTP service deliberately points at the occupied address; the others
	// (and the AMQP listener) use ephemeral ports that bind fine.
	services := []service{
		{name: "ok-1", addr: "127.0.0.1:0", handler: nopHandler{}},
		{name: "clash", addr: busyAddr, handler: nopHandler{}},
		{name: "ok-2", addr: "127.0.0.1:0", handler: nopHandler{}},
	}
	amqp := amqpService{addr: "127.0.0.1:0", server: sbserver.New(sbstore.New())}

	// A context that is NOT cancelled: the only way run can return is via the
	// bind-failure path. If the fix is absent, run blocks forever (or os.Exits).
	ctx := context.Background()

	done := make(chan error, 1)
	go func() {
		done <- run(ctx, logger, services, amqp, newManagementServer("127.0.0.1:0", nil, nil), testCert(t))
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("run returned nil error despite a listener bind failure; expected it to surface the error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run did not return within 5s on a bind failure (it hung or os.Exited instead of graceful shutdown)")
	}
}

// TestRunManagementEndpoints verifies the plain-HTTP management server: its
// health endpoint answers 200 once run has brought every listener up, and it
// serves the certificate and key PEM it was given. The health probe is the
// signal a container HEALTHCHECK and the test suites wait on, so a
// wait-with-timeout for "healthcheck OK" mirrors how the suites gate on
// readiness.
func TestRunManagementEndpoints(t *testing.T) {
	mgmtLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve management port: %v", err)
	}
	mgmtAddr := mgmtLn.Addr().String()
	mgmtLn.Close()

	certPEM, keyPEM, err := devcert.Generate()
	if err != nil {
		t.Fatalf("generate cert: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))
	services := []service{{name: "ok", addr: "127.0.0.1:0", handler: nopHandler{}}}
	amqp := amqpService{addr: "127.0.0.1:0", server: sbserver.New(sbstore.New())}
	mgmt := newManagementServer(mgmtAddr, certPEM, keyPEM)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, logger, services, amqp, mgmt, testCert(t))
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	base := "http://" + mgmtAddr

	// Wait (with timeout) for "healthcheck OK".
	if err := waitForHealth(base+"/health", 5*time.Second); err != nil {
		t.Fatalf("health endpoint never reported ready: %v", err)
	}

	// The certificate and key are served verbatim.
	for path, want := range map[string][]byte{
		"/certs/pubkey":  certPEM,
		"/certs/privkey": keyPEM,
	} {
		resp, err := http.Get(base + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200", path, resp.StatusCode)
		}
		if !bytes.Equal(body, want) {
			t.Fatalf("GET %s body does not match the served PEM", path)
		}
	}
}

// waitForHealth polls url until it returns 200 or the timeout elapses.
func waitForHealth(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("health endpoint not ready within %s: %s", timeout, url)
}

// nopHandler is a do-nothing http.Handler for wiring services in tests.
type nopHandler struct{}

func (nopHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}

// testCert builds a self-signed TLS certificate for run, which always serves
// HTTPS.
func testCert(t *testing.T) *tls.Certificate {
	t.Helper()
	certPEM, keyPEM, err := devcert.Generate()
	if err != nil {
		t.Fatalf("generate cert: %v", err)
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	return &cert
}

// testWriter funnels run's slog output into the test log.
type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Logf("%s", p)
	return len(p), nil
}
