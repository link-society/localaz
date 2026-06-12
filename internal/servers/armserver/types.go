package armserver

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
