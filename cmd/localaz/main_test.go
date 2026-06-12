package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"localaz.dev/internal/servers/sbserver"
	"localaz.dev/internal/stores/sbstore"
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
		done <- run(ctx, logger, services, amqp, nil)
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

// nopHandler is a do-nothing http.Handler for wiring services in tests.
type nopHandler struct{}

func (nopHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}

// testWriter funnels run's slog output into the test log.
type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Logf("%s", p)
	return len(p), nil
}

// TestLogRequestsRecoversPanic verifies the access-log middleware recovers a
// panic from the wrapped handler instead of letting it propagate, and writes a
// 500 response when the handler wrote nothing.
func TestLogRequestsRecoversPanic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	panicking := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	h := logRequests(logger, panicking)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/explode", nil)

	// (a) ServeHTTP must not propagate the panic (no crash).
	h.ServeHTTP(rec, req)

	// (b) the response status must be 500.
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d after panic, got %d", http.StatusInternalServerError, rec.Code)
	}
}

// TestLogRequestsPassesThroughStatus verifies a non-panicking handler still
// returns its normal status through the middleware.
func TestLogRequestsPassesThroughStatus(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	h := logRequests(logger, ok)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Fatalf("expected status %d, got %d", http.StatusTeapot, rec.Code)
	}
}
