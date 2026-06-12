package aadserver

import (
	"net/http"

	"localaz.dev/internal/utils/httpx"
)

// handleOpenIDConfig serves the OpenID Connect discovery document for the given
// tenant. Endpoint URLs are derived from the request so the document is valid
// no matter which host or port the authority is reached on.
func (s *Server) handleOpenIDConfig(w http.ResponseWriter, r *http.Request, tenant string) {
	base := baseURL(r) + "/" + tenant
	httpx.WriteJSON(w, http.StatusOK, openIDConfig{
		Issuer:                            base,
		AuthorizationEndpoint:             base + "/oauth2/authorize",
		TokenEndpoint:                     base + "/oauth2/token",
		DeviceAuthorizationEndpoint:       base + "/oauth2/devicecode",
		JWKSURI:                           base + "/discovery/keys",
		EndSessionEndpoint:                base + "/oauth2/logout",
		ResponseTypesSupported:            []string{"code", "id_token", "token", "code id_token"},
		SubjectTypesSupported:             []string{"public"},
		IDTokenSigningAlgValuesSupported:  []string{"RS256"},
		TokenEndpointAuthMethodsSupported: []string{"client_secret_post", "client_secret_basic", "private_key_jwt"},
		ScopesSupported:                   []string{"openid", "profile", "email", "offline_access"},
		GrantTypesSupported: []string{
			"authorization_code", "client_credentials", "refresh_token",
			"password", "urn:ietf:params:oauth:grant-type:device_code",
		},
	})
}

// handleJWKS publishes the public signing key.
func (s *Server) handleJWKS(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, s.signer.jwks())
}
