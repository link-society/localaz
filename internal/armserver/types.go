package armserver

import (
	"encoding/json"
	"net/http"
)

// listEnvelope is the standard ARM collection wrapper.
type listEnvelope struct {
	Value any `json:"value"`
}

// subscription is an ARM subscription record.
type subscription struct {
	ID                  string `json:"id"`
	SubscriptionID      string `json:"subscriptionId"`
	DisplayName         string `json:"displayName"`
	State               string `json:"state"`
	TenantID            string `json:"tenantId"`
	AuthorizationSource string `json:"authorizationSource"`
}

// tenant is an ARM tenant record.
type tenant struct {
	ID       string `json:"id"`
	TenantID string `json:"tenantId"`
}

// cloudMetadata is one entry of the ARM /metadata/endpoints document that the
// Azure CLI consumes for cloud-endpoint discovery.
type cloudMetadata struct {
	Portal          string           `json:"portal"`
	Authentication  metadataAuth     `json:"authentication"`
	Name            string           `json:"name"`
	Suffixes        metadataSuffixes `json:"suffixes"`
	ResourceManager string           `json:"resourceManager"`
	// LogAnalyticsResourceID uses the doubled-prefix key the log-analytics CLI
	// extension looks up verbatim in the metadata document.
	LogAnalyticsResourceID string `json:"logAnalyticslogAnalyticsResourceId"`
}

// metadataAuth is the authentication block of a cloud metadata entry.
type metadataAuth struct {
	LoginEndpoint string   `json:"loginEndpoint"`
	Audiences     []string `json:"audiences"`
}

// metadataSuffixes is the suffixes block of a cloud metadata entry.
type metadataSuffixes struct {
	StorageEndpoint string `json:"storageEndpoint"`
}

// armError is the standard ARM error envelope.
type armError struct {
	Error armErrorBody `json:"error"`
}

type armErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, armError{Error: armErrorBody{Code: code, Message: message}})
}
