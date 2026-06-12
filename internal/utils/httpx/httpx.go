// Package httpx holds small HTTP helpers shared by the JSON-speaking services
// (the AAD/ARM control plane, Event Grid and Monitor Logs). It keeps the
// response-writing boilerplate in one place without coupling those services to
// each other.
package httpx

import (
	"encoding/json"
	"net/http"
)

// WriteJSON encodes body as JSON and writes it with the given status code and
// an application/json content type.
func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// errorEnvelope is the {"error":{"code","message"}} shape used by Azure
// Resource Manager, Event Grid and Monitor for failures.
type errorEnvelope struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteError writes the standard Azure JSON error envelope.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, errorEnvelope{Error: errorDetail{Code: code, Message: message}})
}
