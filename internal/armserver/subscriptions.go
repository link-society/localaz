package armserver

import (
	"fmt"
	"net/http"
)

// handleSubscriptions lists the single subscription owned by the emulator.
func (s *Server) handleSubscriptions(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, listEnvelope{Value: []subscription{s.subscription()}})
}

// handleSubscription returns the subscription by id (any id resolves to the
// single emulated subscription).
func (s *Server) handleSubscription(w http.ResponseWriter, _ *http.Request, _ string) {
	writeJSON(w, http.StatusOK, s.subscription())
}

// handleTenants lists the single tenant owned by the emulator.
func (s *Server) handleTenants(w http.ResponseWriter, _ *http.Request) {
	cfg := s.store.Config()
	writeJSON(w, http.StatusOK, listEnvelope{Value: []tenant{{
		ID:       fmt.Sprintf("/tenants/%s", cfg.TenantID),
		TenantID: cfg.TenantID,
	}}})
}

func (s *Server) subscription() subscription {
	cfg := s.store.Config()
	return subscription{
		ID:                  fmt.Sprintf("/subscriptions/%s", cfg.SubscriptionID),
		SubscriptionID:      cfg.SubscriptionID,
		DisplayName:         cfg.SubscriptionName,
		State:               "Enabled",
		TenantID:            cfg.TenantID,
		AuthorizationSource: "RoleBased",
	}
}
