package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
