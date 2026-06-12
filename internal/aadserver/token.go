package aadserver

import (
	"net/http"
	"strings"
)

// handleToken implements the OAuth2 token endpoint. Every supported grant type
// resolves to the same outcome: a freshly signed bearer token for the
// requested resource. Client secrets and codes are accepted without
// verification, matching localaz's "auth is emulated, not enforced" stance.
func (s *Server) handleToken(w http.ResponseWriter, r *http.Request, tenant string) {
	if r.Method != http.MethodPost {
		writeOAuthError(w, http.StatusMethodNotAllowed, "invalid_request", "token endpoint requires POST")
		return
	}
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "malformed form body")
		return
	}

	grant := r.PostForm.Get("grant_type")
	clientID := r.PostForm.Get("client_id")
	if clientID == "" {
		clientID = "00000000-0000-0000-0000-000000000000"
	}
	username := r.PostForm.Get("username")

	audience := resourceFromScope(r.PostForm.Get("scope"))
	if audience == "" {
		audience = r.PostForm.Get("resource")
	}
	if audience == "" {
		audience = baseURL(r)
	}

	issuer := baseURL(r) + "/" + tenant + "/v2.0"
	token, err := s.signer.sign(newClaims(issuer, audience, tenant, clientID, username, username))
	if err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}

	resp := tokenResponse{
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		ExtExpiresIn: 3600,
		AccessToken:  token,
		Scope:        r.PostForm.Get("scope"),
	}
	// Interactive and delegated grants expect a refresh token; openid scopes
	// expect an id token.
	if grant != "client_credentials" {
		resp.RefreshToken = "localaz-refresh-token"
	}
	if strings.Contains(r.PostForm.Get("scope"), "openid") {
		idToken, err := s.signer.sign(newClaims(issuer, clientID, tenant, clientID, username, username))
		if err == nil {
			resp.IDToken = idToken
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// resourceFromScope turns an OAuth2 scope list ("https://host/.default openid")
// into the resource audience that scope targets.
func resourceFromScope(scope string) string {
	for _, s := range strings.Fields(scope) {
		if s == "openid" || s == "profile" || s == "email" || s == "offline_access" {
			continue
		}
		s = strings.TrimSuffix(s, "/.default")
		s = strings.TrimSuffix(s, ".default")
		return strings.TrimRight(s, "/")
	}
	return ""
}
