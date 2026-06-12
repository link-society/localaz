package monitorserver

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"localaz.dev/internal/stores/monitorstore"
	"localaz.dev/internal/utils/httpx"
)

// Ingestion body size caps. They guard against unbounded memory growth from a
// large upload or a gzip bomb (a tiny Content-Encoding: gzip body that inflates
// to gigabytes). maxIngestBytes bounds the raw request body via
// http.MaxBytesReader; maxDecompressedBytes bounds the inflated stream via an
// io.LimitReader. They are vars rather than consts only so tests can lower
// them; production code never reassigns them.
var (
	maxIngestBytes       int64 = 64 << 20  // 64 MiB raw upload.
	maxDecompressedBytes int64 = 256 << 20 // 256 MiB after gzip inflation.
)

// errBodyTooLarge signals that the request body (raw or decompressed) exceeded
// its size cap, so handleIngest can answer 413 rather than 400.
var errBodyTooLarge = errors.New("request body exceeds size limit")

// handleIngest implements the Logs Ingestion API:
//
//	POST /dataCollectionRules/{ruleId}/streams/{stream}?api-version=2023-01-01
//
// The body is a JSON array of log records (optionally gzip-encoded). The data
// collection rule id is accepted but not validated; the stream name selects
// the destination table. A successful upload returns 204 No Content.
func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request, stream string) {
	if r.Method != http.MethodPost {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "Only POST is supported.")
		return
	}

	body, err := readBody(w, r)
	if err != nil {
		if errors.Is(err, errBodyTooLarge) {
			httpx.WriteError(w, http.StatusRequestEntityTooLarge, "RequestEntityTooLarge", "Request body exceeds the maximum allowed size.")
			return
		}
		httpx.WriteError(w, http.StatusBadRequest, "BadRequest", "Could not read request body.")
		return
	}

	var records []monitorstore.Row
	if err := json.Unmarshal(body, &records); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "InvalidContent", "Request body must be a JSON array of log records.")
		return
	}

	s.store.Ingest(stream, records)
	w.WriteHeader(http.StatusNoContent)
}

// readBody reads the request body, transparently decompressing it when the
// client set Content-Encoding: gzip. Both the raw upload and the decompressed
// stream are size-bounded (see maxIngestBytes / maxDecompressedBytes) so an
// oversized body or a gzip bomb cannot expand unbounded in memory. An
// over-limit body returns errBodyTooLarge.
func readBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxIngestBytes)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return nil, errBodyTooLarge
		}
		return nil, err
	}
	if r.Header.Get("Content-Encoding") != "gzip" {
		return raw, nil
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	// Read one byte past the cap so we can detect an over-limit inflation.
	decompressed, err := io.ReadAll(io.LimitReader(zr, maxDecompressedBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(decompressed)) > maxDecompressedBytes {
		return nil, errBodyTooLarge
	}
	return decompressed, nil
}
