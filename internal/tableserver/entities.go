package tableserver

import (
	"encoding/json"
	"io"
	"net/http"

	"localaz.dev/internal/azerr"
	"localaz.dev/internal/tablestore"
)

func (s *Server) insertEntity(w http.ResponseWriter, r *http.Request, account, table string) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4<<20))
	if err != nil {
		s.writeError(w, r, azerr.Internal(err.Error()))
		return
	}
	var props map[string]json.RawMessage
	if err := json.Unmarshal(body, &props); err != nil {
		s.writeError(w, r, azerr.New(http.StatusBadRequest, azerr.CodeInvalidInput, "Invalid entity payload."))
		return
	}
	pk := decodeKey(props["PartitionKey"])
	rk := decodeKey(props["RowKey"])

	ent, err := s.store.InsertEntity(account, table, pk, rk, props)
	if s.mapStoreErr(w, r, err) {
		return
	}

	w.Header().Set("ETag", ent.ETag)
	w.Header().Set("Content-Type", contentType)
	if preferNoContent(r.Header.Get("Prefer")) {
		w.Header().Set("Preference-Applied", "return-no-content")
		w.Header().Set("Location", s.entityLocation(r, account, table, pk, rk))
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Location", s.entityLocation(r, account, table, pk, rk))
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(s.entityResponse(r, account, table, ent))
}

func (s *Server) getEntity(w http.ResponseWriter, r *http.Request, account, table, pk, rk string) {
	ent, err := s.store.GetEntity(account, table, pk, rk)
	if s.mapStoreErr(w, r, err) {
		return
	}
	w.Header().Set("ETag", ent.ETag)
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(s.entityResponse(r, account, table, ent))
}

func (s *Server) listEntities(w http.ResponseWriter, r *http.Request, account, table string) {
	ents, err := s.store.ListEntities(account, table)
	if s.mapStoreErr(w, r, err) {
		return
	}

	q := r.URL.Query()
	var filter filterFunc = func(map[string]json.RawMessage) bool { return true }
	if expr := q.Get("$filter"); expr != "" {
		fn, ferr := parseFilter(expr)
		if ferr != nil {
			s.writeError(w, r, azerr.New(http.StatusBadRequest, azerr.CodeInvalidInput, "Unsupported $filter expression."))
			return
		}
		filter = fn
	}
	top := parseInt(q.Get("$top"), -1)
	selectClause := q.Get("$select")

	value := make([]map[string]json.RawMessage, 0, len(ents))
	for _, ent := range ents {
		rendered := renderEntity(ent)
		if !filter(rendered) {
			continue
		}
		value = append(value, projectSelect(rendered, selectClause))
		if top >= 0 && len(value) >= top {
			break
		}
	}

	out, err := json.Marshal(map[string]any{
		"odata.metadata": s.metadataBase(r, account) + "#" + table,
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

func (s *Server) updateEntity(w http.ResponseWriter, r *http.Request, account, table, pk, rk string, merge bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4<<20))
	if err != nil {
		s.writeError(w, r, azerr.Internal(err.Error()))
		return
	}
	var props map[string]json.RawMessage
	if len(body) > 0 {
		if err := json.Unmarshal(body, &props); err != nil {
			s.writeError(w, r, azerr.New(http.StatusBadRequest, azerr.CodeInvalidInput, "Invalid entity payload."))
			return
		}
	}
	ifMatch := r.Header.Get("If-Match")

	var ent *tablestore.Entity
	if merge {
		ent, err = s.store.MergeEntity(account, table, pk, rk, props, ifMatch)
	} else {
		ent, err = s.store.ReplaceEntity(account, table, pk, rk, props, ifMatch)
	}
	if s.mapStoreErr(w, r, err) {
		return
	}
	w.Header().Set("ETag", ent.ETag)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteEntity(w http.ResponseWriter, r *http.Request, account, table, pk, rk string) {
	ifMatch := r.Header.Get("If-Match")
	if s.mapStoreErr(w, r, s.store.DeleteEntity(account, table, pk, rk, ifMatch)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// entityResponse renders a single entity with its odata.metadata wrapper.
func (s *Server) entityResponse(r *http.Request, account, table string, ent *tablestore.Entity) []byte {
	m := renderEntity(ent)
	m2 := make(map[string]json.RawMessage, len(m)+1)
	meta, _ := json.Marshal(s.metadataBase(r, account) + "#" + table + "/@Element")
	m2["odata.metadata"] = meta
	for k, v := range m {
		m2[k] = v
	}
	out, _ := json.Marshal(m2)
	return out
}

func decodeKey(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}
