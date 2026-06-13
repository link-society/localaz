// Package kvserver implements the HTTP surface of the Azure Key Vault secrets
// data plane. It is faithful enough that the official azsecrets SDK works
// against it unmodified, including the challenge-based authentication handshake
// (a 401 carrying a WWW-Authenticate header) the SDK requires before it will
// attach a bearer token and a request body.
package kvserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"localaz.dev/internal/stores/keyvaultstore"
	"localaz.dev/internal/utils/httpx"
)

// maxBodyBytes caps a request body. Key Vault secret values top out at 25 KiB;
// this leaves generous room while bounding memory per request.
const maxBodyBytes = 256 * 1024

// Server routes Key Vault secret requests onto a keyvaultstore.Store.
type Server struct {
	store     *keyvaultstore.Store
	authority string
}

// New constructs a Server backed by store. authority is the AAD authority URL
// advertised in the authentication challenge so a client's credential knows
// where to obtain a token.
func New(store *keyvaultstore.Store, authority string) *Server {
	return &Server{store: store, authority: authority}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Challenge-based auth: an unauthenticated request is answered with a 401
	// and a WWW-Authenticate header. The azsecrets SDK relies on this to learn
	// the token scope/tenant and to (re)attach a stripped request body before
	// retrying with a bearer token. The token itself is accepted but never
	// validated, like every other localaz credential.
	if r.Header.Get("Authorization") == "" {
		w.Header().Set("WWW-Authenticate",
			fmt.Sprintf(`Bearer authorization="%s", resource="https://vault.azure.net"`, s.authority))
		httpx.WriteError(w, http.StatusUnauthorized, "Unauthorized", "Request is missing a bearer token.")
		return
	}
	s.route(w, r)
}

func (s *Server) route(w http.ResponseWriter, r *http.Request) {
	base := "https://" + r.Host
	host := r.Host

	segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(segments) == 0 || segments[0] != "secrets" {
		httpx.WriteError(w, http.StatusNotFound, "NotFound", "Unknown operation.")
		return
	}

	name := ""
	if len(segments) >= 2 {
		decoded, err := url.PathUnescape(segments[1])
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "BadParameter", "Invalid secret name.")
			return
		}
		name = decoded
	}

	switch {
	case len(segments) == 1 && r.Method == http.MethodGet:
		s.listSecrets(w, host, base)
	case len(segments) == 2 && r.Method == http.MethodPut:
		s.setSecret(w, r, host, base, name)
	case len(segments) == 2 && r.Method == http.MethodGet:
		s.getSecret(w, host, base, name, "")
	case len(segments) == 2 && r.Method == http.MethodPatch:
		s.updateSecret(w, r, host, base, name, "")
	case len(segments) == 2 && r.Method == http.MethodDelete:
		s.deleteSecret(w, host, base, name)
	case len(segments) == 3 && segments[2] == "versions" && r.Method == http.MethodGet:
		s.listVersions(w, host, base, name)
	case len(segments) == 3 && r.Method == http.MethodGet:
		s.getSecret(w, host, base, name, segments[2])
	case len(segments) == 3 && r.Method == http.MethodPatch:
		s.updateSecret(w, r, host, base, name, segments[2])
	default:
		httpx.WriteError(w, http.StatusNotFound, "NotFound", "Unknown operation.")
	}
}

func (s *Server) setSecret(w http.ResponseWriter, r *http.Request, host, base, name string) {
	var req setSecretRequest
	if !decodeBody(w, r, &req) {
		return
	}
	sec := s.store.SetSecret(host, name, req.Value, req.ContentType, req.Tags, req.Attributes.toAttributes())
	httpx.WriteJSON(w, http.StatusOK, bundleOf(base, sec, true))
}

func (s *Server) getSecret(w http.ResponseWriter, host, base, name, version string) {
	sec, err := s.store.GetSecret(host, name, version)
	if err != nil {
		writeSecretNotFound(w, name)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, bundleOf(base, sec, true))
}

func (s *Server) updateSecret(w http.ResponseWriter, r *http.Request, host, base, name, version string) {
	var req updateSecretRequest
	if !decodeBody(w, r, &req) {
		return
	}
	sec, err := s.store.UpdateSecret(host, name, version, req.ContentType, req.Tags, req.Attributes.toAttributes())
	if err != nil {
		writeSecretNotFound(w, name)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, bundleOf(base, sec, false))
}

func (s *Server) deleteSecret(w http.ResponseWriter, host, base, name string) {
	sec, err := s.store.DeleteSecret(host, name)
	if err != nil {
		writeSecretNotFound(w, name)
		return
	}
	body := deletedSecretBundle{
		secretBundle: bundleOf(base, sec, true),
		RecoveryID:   base + "/deletedsecrets/" + name,
	}
	httpx.WriteJSON(w, http.StatusOK, body)
}

func (s *Server) listSecrets(w http.ResponseWriter, host, base string) {
	secrets := s.store.ListSecrets(host)
	items := make([]secretBundle, 0, len(secrets))
	for _, sec := range secrets {
		items = append(items, bundleOf(base, sec, false))
	}
	httpx.WriteJSON(w, http.StatusOK, secretListResult{Value: items})
}

func (s *Server) listVersions(w http.ResponseWriter, host, base, name string) {
	versions, err := s.store.ListVersions(host, name)
	if err != nil {
		writeSecretNotFound(w, name)
		return
	}
	items := make([]secretBundle, 0, len(versions))
	for _, sec := range versions {
		items = append(items, bundleOf(base, sec, false))
	}
	httpx.WriteJSON(w, http.StatusOK, secretListResult{Value: items})
}

// decodeBody reads and unmarshals a JSON request body, bounding its size. It
// writes a 400 and returns false on failure.
func decodeBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			httpx.WriteError(w, http.StatusRequestEntityTooLarge, "RequestEntityTooLarge", "Request body is too large.")
			return false
		}
		httpx.WriteError(w, http.StatusBadRequest, "BadParameter", "Request body is not valid JSON.")
		return false
	}
	return true
}

func writeSecretNotFound(w http.ResponseWriter, name string) {
	httpx.WriteError(w, http.StatusNotFound, "SecretNotFound",
		fmt.Sprintf("A secret with (name/id) %s was not found in this key vault.", name))
}
