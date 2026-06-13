package keyvaultstore

import "time"

// Secret is one stored version of a Key Vault secret.
type Secret struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Value       string            `json:"value"`
	ContentType string            `json:"contentType,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	Enabled     bool              `json:"enabled"`
	NotBefore   *time.Time        `json:"notBefore,omitempty"`
	Expires     *time.Time        `json:"expires,omitempty"`
	Created     time.Time         `json:"created"`
	Updated     time.Time         `json:"updated"`
}

// Attributes carries the mutable secret attributes accepted on set and update.
// A nil field leaves the corresponding attribute unchanged on update.
type Attributes struct {
	Enabled   *bool
	NotBefore *time.Time
	Expires   *time.Time
}

// vault holds every secret for a single vault host.
type vault struct {
	Secrets map[string]*secretEntry `json:"secrets"`
}

// secretEntry is the version history of one named secret.
type secretEntry struct {
	Current  string             `json:"current"`
	Order    []string           `json:"order"`
	Versions map[string]*Secret `json:"versions"`
}
