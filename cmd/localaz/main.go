// Command localaz runs the localaz Azure emulator. The current build exposes
// the Azure Blob service on a single HTTP listener; additional services will be
// mounted on the same process so that users only ever run one container.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"localaz.dev/internal/blobserver"
	"localaz.dev/internal/blobstore/fsstore"
)

func main() {
	addr := flag.String("addr", envOr("LOCALAZ_BLOB_ADDR", ":10000"), "blob service listen address")
	dataDir := flag.String("data", envOr("LOCALAZ_DATA_DIR", "/data"), "directory for persisted state")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	store, err := fsstore.New(*dataDir)
	if err != nil {
		logger.Error("init store", "err", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           logRequests(logger, blobserver.New(store)),
		ReadHeaderTimeout: 30 * time.Second,
	}

	go func() {
		logger.Info("blob service listening", "addr", *addr, "data", *dataDir)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("serve", "err", err)
			os.Exit(1)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "err", err)
	}
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
