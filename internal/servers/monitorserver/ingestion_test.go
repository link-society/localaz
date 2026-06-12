package monitorserver

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"localaz.dev/internal/stores/monitorstore"
)

// TestIngestRejectsOversizedRawBody verifies that a raw (uncompressed) upload
// larger than maxIngestBytes is rejected with a 4xx and that nothing lands in
// the store. Guards the http.MaxBytesReader bound: without it the body is read
// into memory unbounded and ingested, so this test fails.
func TestIngestRejectsOversizedRawBody(t *testing.T) {
	defer restoreLimits(maxIngestBytes, maxDecompressedBytes)
	maxIngestBytes = 1 << 10 // 1 KiB cap for a focused, fast test.

	store := monitorstore.New()
	srv := httptest.NewServer(New(store))
	defer srv.Close()

	// A valid JSON array that is comfortably larger than the lowered cap.
	body := []byte("[" + strings.Repeat(`{"x":"yyyyyyyy"},`, 200) + `{"x":"z"}]`)
	if int64(len(body)) <= maxIngestBytes {
		t.Fatalf("test body (%d bytes) must exceed cap (%d bytes)", len(body), maxIngestBytes)
	}

	resp, err := http.Post(srv.URL+"/dataCollectionRules/dcr/streams/Custom-Big", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 400 || resp.StatusCode >= 500 {
		t.Fatalf("expected 4xx rejection, got %d", resp.StatusCode)
	}
	if rows, ok := store.Rows("Big"); ok {
		t.Fatalf("oversized body was ingested: %d rows", len(rows))
	}
}

// TestIngestRejectsGzipBomb verifies that a gzip-encoded body whose
// DECOMPRESSED size exceeds maxDecompressedBytes is rejected with a 4xx and
// not ingested. The compressed payload is tiny but inflates past the lowered
// cap. Guards the io.LimitReader bound on the decompressed stream: without it
// the inflated bytes are read unbounded and ingested, so this test fails.
func TestIngestRejectsGzipBomb(t *testing.T) {
	defer restoreLimits(maxIngestBytes, maxDecompressedBytes)
	maxDecompressedBytes = 1 << 10 // 1 KiB decompressed cap.

	store := monitorstore.New()
	srv := httptest.NewServer(New(store))
	defer srv.Close()

	// Highly compressible JSON array that inflates well past the cap.
	plain := []byte("[" + strings.Repeat(`{"a":"aaaaaaaaaaaaaaaa"},`, 2000) + `{"a":"a"}]`)
	if int64(len(plain)) <= maxDecompressedBytes {
		t.Fatalf("decompressed payload (%d bytes) must exceed cap (%d bytes)", len(plain), maxDecompressedBytes)
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(plain); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/dataCollectionRules/dcr/streams/Custom-Bomb", &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 400 || resp.StatusCode >= 500 {
		t.Fatalf("expected 4xx rejection, got %d", resp.StatusCode)
	}
	if rows, ok := store.Rows("Bomb"); ok {
		t.Fatalf("gzip bomb was ingested: %d rows", len(rows))
	}
}

func restoreLimits(rawLimit, decompressedLimit int64) {
	maxIngestBytes = rawLimit
	maxDecompressedBytes = decompressedLimit
}
