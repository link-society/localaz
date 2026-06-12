package armstore

// Config holds the static facts an emulated ARM control plane advertises: the
// single subscription and tenant it owns, and the cloud endpoints it publishes
// through the metadata document so the Azure CLI can discover them.
type Config struct {
	CloudName        string
	SubscriptionID   string
	SubscriptionName string
	TenantID         string
	Location         string

	// LoginEndpoint is the Entra ID authority base URL (trailing slash).
	LoginEndpoint string
	// ResourceManager is this ARM service's own externally reachable base URL.
	ResourceManager string
	// LogAnalyticsEndpoint is the Monitor Logs query endpoint advertised to the
	// log-analytics CLI extension via the metadata document.
	LogAnalyticsEndpoint string
	// StorageSuffix is the storage endpoint suffix advertised to the CLI.
	StorageSuffix string
}

// ResourceGroup is a minimal ARM resource group record.
type ResourceGroup struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Location   string            `json:"location"`
	Tags       map[string]string `json:"tags,omitempty"`
	Properties groupProperties   `json:"properties"`
}

// groupProperties carries the provisioning state of a resource group.
type groupProperties struct {
	ProvisioningState string `json:"provisioningState"`
}
