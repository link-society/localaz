package tableserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"localaz.dev/internal/azerr"
)

// contentType is the response media type the Table service uses.
const contentType = "application/json;odata=minimalmetadata;streaming=true;charset=utf-8"

type createTableBody struct {
	TableName string `json:"TableName"`
}

func (s *Server) createTable(w http.ResponseWriter, r *http.Request, account string) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		s.writeError(w, r, azerr.Internal(err.Error()))
		return
	}
	var req createTableBody
	if err := json.Unmarshal(body, &req); err != nil || req.TableName == "" {
		s.writeError(w, r, azerr.New(http.StatusBadRequest, azerr.CodeInvalidInput, "Invalid table creation payload."))
		return
	}
	if s.mapStoreErr(w, r, s.store.CreateTable(account, req.TableName)) {
		return
	}

	w.Header().Set("Content-Type", contentType)
	if preferNoContent(r.Header.Get("Prefer")) {
		w.Header().Set("Preference-Applied", "return-no-content")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	out, _ := json.Marshal(map[string]any{
		"odata.metadata": s.metadataBase(r, account) + "#Tables/@Element",
		"TableName":      req.TableName,
	})
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(out)
}

func (s *Server) deleteTable(w http.ResponseWriter, r *http.Request, account, name string) {
	if s.mapStoreErr(w, r, s.store.DeleteTable(account, name)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listTables(w http.ResponseWriter, r *http.Request, account string) {
	names := s.store.ListTables(account)
	value := make([]map[string]string, 0, len(names))
	for _, n := range names {
		value = append(value, map[string]string{"TableName": n})
	}
	out, err := json.Marshal(map[string]any{
		"odata.metadata": s.metadataBase(r, account) + "#Tables",
		"value":          value,
	})
	if err != nil {
		s.writeError(w, r, azerr.Internal(err.Error()))
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out)
}

func (s *Server) entityLocation(r *http.Request, account, table, pk, rk string) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/%s/%s(PartitionKey='%s',RowKey='%s')",
		scheme, r.Host, account, table, escapeKey(pk), escapeKey(rk))
}

func escapeKey(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
