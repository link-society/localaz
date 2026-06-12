package monitorserver

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"

	"localaz.dev/internal/stores/monitorstore"
	"localaz.dev/internal/utils/httpx"
)

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

	body, err := readBody(r)
	if err != nil {
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
// client set Content-Encoding: gzip.
func readBody(r *http.Request) ([]byte, error) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
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
	return io.ReadAll(zr)
}
