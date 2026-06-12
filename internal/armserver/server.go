// Package armserver implements a minimal Azure Resource Manager emulator: the
// cloud-metadata discovery document, the subscription and tenant listings used
// during sign-in, and resource-group CRUD. Together with aadserver it lets the
// Azure CLI register localaz as a custom cloud, sign in, and resolve the
// data-plane endpoints of the other emulated services.
package armserver

import (
	"net/http"
	"strings"

	"localaz.dev/internal/armstore"
)

// Server routes ARM requests onto an armstore.Store.
type Server struct {
	store *armstore.Store
}

// New constructs a Server backed by the given store.
func New(store *armstore.Store) *Server {
	return &Server{store: store}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
	segments := strings.Split(path, "/")
	lower := strings.ToLower(path)

	switch {
	case lower == "metadata/endpoints":
		s.handleMetadata(w, r)
	case lower == "tenants":
		s.handleTenants(w, r)
	case lower == "subscriptions":
		s.handleSubscriptions(w, r)
	// Any path that names a resource provider (e.g. .../providers/Microsoft.ServiceBus/...).
	case providerIndex(segments) >= 0:
		s.handleProvider(w, r, segments)
	// /subscriptions/{id}/resourcegroups[/{name}]
	case len(segments) >= 3 && strings.EqualFold(segments[0], "subscriptions") && strings.EqualFold(segments[2], "resourcegroups"):
		s.handleResourceGroups(w, r, segments)
	// /subscriptions/{id}
	case len(segments) == 2 && strings.EqualFold(segments[0], "subscriptions"):
		s.handleSubscription(w, r, segments[1])
	default:
		writeError(w, http.StatusNotFound, "NotFound", "Unknown operation: "+r.URL.Path)
	}
}

// providerIndex returns the index of the "providers" segment, or -1 when the
// path does not address a resource provider.
func providerIndex(segments []string) int {
	for i, seg := range segments {
		if strings.EqualFold(seg, "providers") {
			return i
		}
	}
	return -1
}
