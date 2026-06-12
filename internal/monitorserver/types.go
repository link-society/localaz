package monitorserver

import (
	"encoding/json"
	"net/http"
)

// queryRequest is the Log Analytics query request body.
type queryRequest struct {
	Query    string `json:"query"`
	Timespan string `json:"timespan,omitempty"`
}

// queryResponse is the Log Analytics query response body.
type queryResponse struct {
	Tables []table `json:"tables"`
}

// table is one result table in a query response.
type table struct {
	Name    string   `json:"name"`
	Columns []column `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

// column describes one column of a result table.
type column struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// errorBody is the generic Azure Monitor error envelope.
type errorBody struct {
	Error errorDetail `json:"error"`
}

// errorDetail is the inner error object.
type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message}})
}
