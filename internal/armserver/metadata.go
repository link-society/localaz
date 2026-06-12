package armserver

import "net/http"

// handleMetadata serves the ARM cloud-endpoint discovery document. The CLI
// reads this (via ARM_CLOUD_METADATA_URL) to resolve service endpoints such as
// the Log Analytics query host. The entry's name must match the custom cloud
// the CLI was registered with.
func (s *Server) handleMetadata(w http.ResponseWriter, _ *http.Request) {
	cfg := s.store.Config()
	writeJSON(w, http.StatusOK, []cloudMetadata{{
		Portal:          cfg.ResourceManager,
		Name:            cfg.CloudName,
		ResourceManager: cfg.ResourceManager,
		Authentication: metadataAuth{
			LoginEndpoint: cfg.LoginEndpoint,
			Audiences:     []string{cfg.ResourceManager},
		},
		Suffixes:               metadataSuffixes{StorageEndpoint: cfg.StorageSuffix},
		LogAnalyticsResourceID: cfg.LogAnalyticsEndpoint,
	}})
}
