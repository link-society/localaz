package monitorserver

import (
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"time"

	"localaz.dev/internal/monitorstore"
)

// handleQuery implements the Log Analytics query API:
//
//	POST /v1/workspaces/{workspaceId}/query
//
// The body is {"query": "<KQL>", "timespan": "..."}. The workspace id and
// timespan are accepted but not validated; the query is evaluated against the
// ingested tables and the result is returned as a single PrimaryResult table.
func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request, _ string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "Only POST is supported.")
		return
	}

	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "BadRequest", "Could not read request body.")
		return
	}
	var body queryRequest
	if err := json.Unmarshal(raw, &body); err != nil || body.Query == "" {
		writeError(w, http.StatusBadRequest, "BadArgumentError", "Request body must contain a 'query' string.")
		return
	}

	result, err := evalKQL(s.store, body.Query)
	if err != nil {
		writeError(w, http.StatusBadRequest, "BadArgumentError", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, queryResponse{Tables: []table{materialize(result)}})
}

// materialize converts the evaluated pipeline result into the wire table:
// ordered columns (with inferred KQL types) and rows as positional value
// arrays.
func materialize(res resultTable) table {
	cols := res.columns
	if len(cols) == 0 {
		cols = inferColumns(res.rows)
	}

	out := table{Name: "PrimaryResult", Columns: make([]column, len(cols)), Rows: make([][]any, len(res.rows))}
	for i, name := range cols {
		out.Columns[i] = column{Name: name, Type: inferType(name, res.rows)}
	}
	for i, row := range res.rows {
		values := make([]any, len(cols))
		for j, name := range cols {
			values[j] = row[name]
		}
		out.Rows[i] = values
	}
	return out
}

// inferColumns collects the union of column names across all rows, placing
// TimeGenerated first (when present) and the rest in alphabetical order for a
// stable response.
func inferColumns(rows []monitorstore.Row) []string {
	seen := make(map[string]struct{})
	for _, r := range rows {
		for k := range r {
			seen[k] = struct{}{}
		}
	}
	cols := make([]string, 0, len(seen))
	hasTime := false
	for k := range seen {
		if k == "TimeGenerated" {
			hasTime = true
			continue
		}
		cols = append(cols, k)
	}
	sort.Strings(cols)
	if hasTime {
		cols = append([]string{"TimeGenerated"}, cols...)
	}
	return cols
}

// inferType maps the first non-nil value found in a column to a KQL column
// type.
func inferType(name string, rows []monitorstore.Row) string {
	for _, r := range rows {
		v, ok := r[name]
		if !ok || v == nil {
			continue
		}
		switch val := v.(type) {
		case bool:
			return "bool"
		case float64:
			if val == float64(int64(val)) {
				return "long"
			}
			return "real"
		case string:
			if name == "TimeGenerated" || isRFC3339(val) {
				return "datetime"
			}
			return "string"
		case map[string]any, []any:
			return "dynamic"
		}
	}
	if name == "TimeGenerated" {
		return "datetime"
	}
	return "string"
}

// isRFC3339 reports whether s parses as an RFC 3339 timestamp.
func isRFC3339(s string) bool {
	if _, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return true
	}
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}
