package main

import (
	"log/slog"
	"net/http"
	"time"
)

// statusRecorder wraps an http.ResponseWriter to capture the status code and
// whether anything has been written yet, so logRequests can log the status and
// safely emit a fallback 500 on panic.
type statusRecorder struct {
	http.ResponseWriter
	status  int
	written bool
}

func (sr *statusRecorder) WriteHeader(code int) {
	if !sr.written {
		sr.status = code
		sr.written = true
		sr.ResponseWriter.WriteHeader(code)
	}
}

func (sr *statusRecorder) Write(b []byte) (int, error) {
	if !sr.written {
		sr.status = http.StatusOK
		sr.written = true
	}
	return sr.ResponseWriter.Write(b)
}

// logRequests is a minimal access log so users can see SDK/CLI traffic hitting
// the emulator. It also recovers panics from the wrapped handler: it logs them
// and, if the handler wrote nothing, returns a 500 so a single handler panic
// cannot drop the request without a response.
func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sr := &statusRecorder{ResponseWriter: w}
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error("handler panic",
					"method", r.Method,
					"path", r.URL.Path+querySuffix(r),
					"panic", rec,
				)
				if !sr.written {
					sr.WriteHeader(http.StatusInternalServerError)
				}
			}
			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path+querySuffix(r),
				"status", sr.status,
				"duration", time.Since(start).Round(time.Millisecond),
			)
		}()
		next.ServeHTTP(sr, r)
	})
}

func querySuffix(r *http.Request) string {
	if r.URL.RawQuery == "" {
		return ""
	}
	return "?" + r.URL.RawQuery
}
