// Package aadserver implements a minimal Microsoft Entra ID (Azure Active
// Directory) emulator: the OpenID Connect discovery document, a JWKS endpoint
// and an OAuth2 token endpoint. It exists so the Azure CLI and SDKs can
// complete a real sign-in flow against localaz instead of the public cloud.
//
// The authority is exposed in ADFS shape (a literal "adfs" tenant segment),
// which is the one mode where MSAL skips public instance discovery and talks
// only to the configured authority. Tokens are signed with a per-process RSA
// key published via JWKS, but their claims are synthetic: like every localaz
// service, sign-in is emulated, not verified.
package aadserver

import (
	"net/http"
	"strings"
)

// Server implements the Entra ID HTTP surface. It holds the signing key used
// to mint tokens.
type Server struct {
	signer *signer
}

// New constructs a Server with a freshly generated signing key.
func New() (*Server, error) {
	s, err := newSigner()
	if err != nil {
		return nil, err
	}
	return &Server{signer: s}, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
	segments := strings.Split(path, "/")

	switch {
	case strings.HasSuffix(path, ".well-known/openid-configuration"):
		s.handleOpenIDConfig(w, r, segments[0])
	case strings.Contains(path, "/discovery/") && strings.HasSuffix(path, "keys"):
		s.handleJWKS(w, r)
	case strings.HasSuffix(path, "/oauth2/token") || strings.HasSuffix(path, "/oauth2/v2.0/token"):
		s.handleToken(w, r, segments[0])
	default:
		writeOAuthError(w, http.StatusNotFound, "invalid_request", "Unknown endpoint: "+r.URL.Path)
	}
}

// baseURL reconstructs this authority's externally reachable origin from the
// inbound request, so discovery documents advertise the right scheme, host and
// port regardless of how the listener was bound.
func baseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
