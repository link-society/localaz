// Package monitorserver implements the HTTP surface of the Azure Monitor Logs
// data plane: the Logs Ingestion API (data-collection rule streams) and the
// Log Analytics query API. It is faithful enough that the official
// ingestion/azlogs and query/azlogs SDKs work against it unmodified.
package monitorserver

import (
	"net/http"
	"strings"

	"localaz.dev/internal/stores/monitorstore"
	"localaz.dev/internal/utils/httpx"
)

// Server routes Azure Monitor Logs requests onto a monitorstore.Store. Both
// the ingestion and query data planes share a single port, distinguished by
// their URL path prefix.
type Server struct {
	store *monitorstore.Store
}

// New constructs a Server backed by the given store.
func New(store *monitorstore.Store) *Server {
	return &Server{store: store}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	segments := strings.Split(path, "/")

	switch {
	// POST dataCollectionRules/{ruleId}/streams/{stream}
	case len(segments) >= 4 && segments[0] == "dataCollectionRules" && segments[2] == "streams":
		s.handleIngest(w, r, segments[3])
	// POST v1/workspaces/{workspaceId}/query
	case len(segments) >= 4 && segments[0] == "v1" && segments[1] == "workspaces" && segments[3] == "query":
		s.handleQuery(w, r, segments[2])
	default:
		httpx.WriteError(w, http.StatusNotFound, "NotFound", "Unknown operation.")
	}
}
