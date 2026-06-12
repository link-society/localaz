package armserver

import (
	"encoding/json"
	"net/http"
)

// groupRequest is the body of a resource-group create/update call.
type groupRequest struct {
	Location string            `json:"location"`
	Tags     map[string]string `json:"tags,omitempty"`
}

// handleResourceGroups dispatches resource-group collection and item requests.
func (s *Server) handleResourceGroups(w http.ResponseWriter, r *http.Request, segments []string) {
	// Collection: /subscriptions/{id}/resourcegroups
	if len(segments) == 3 {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "unsupported method")
			return
		}
		writeJSON(w, http.StatusOK, listEnvelope{Value: s.store.ListResourceGroups()})
		return
	}

	name := segments[3]
	switch r.Method {
	case http.MethodPut:
		var req groupRequest
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&req)
		}
		writeJSON(w, http.StatusCreated, s.store.PutResourceGroup(name, req.Location, req.Tags))
	case http.MethodGet:
		rg, ok := s.store.GetResourceGroup(name)
		if !ok {
			writeError(w, http.StatusNotFound, "ResourceGroupNotFound", "Resource group '"+name+"' could not be found.")
			return
		}
		writeJSON(w, http.StatusOK, rg)
	case http.MethodDelete:
		if !s.store.DeleteResourceGroup(name) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
	default:
		writeError(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "unsupported method")
	}
}
