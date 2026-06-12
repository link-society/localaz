package aadserver

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// decodeJWTPayload splits a compact JWT, base64url-decodes the middle segment
// and json-unmarshals it into a generic claims map.
func decodeJWTPayload(t *testing.T, token string) map[string]any {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token is not a compact JWT (got %d segments): %q", len(parts), token)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode JWT payload: %v", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("unmarshal JWT payload: %v", err)
	}
	return claims
}

// postToken drives the token endpoint through the full HTTP surface and returns
// the decoded token response.
func postToken(t *testing.T, srv *httptest.Server, form url.Values) tokenResponse {
	t.Helper()
	resp, err := http.PostForm(srv.URL+"/adfs/oauth2/v2.0/token", form)
	if err != nil {
		t.Fatalf("POST token endpoint: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("token endpoint status = %d, want 200", resp.StatusCode)
	}
	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	return tr
}

// TestTokenUserFlowSubject covers the ROPC (password) grant: when a username is
// supplied the id_token's sub/oid must reflect the USER, not the client id, and
// must be stable across identical requests.
func TestTokenUserFlowSubject(t *testing.T) {
	s, err := New("")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	srv := httptest.NewServer(s)
	defer srv.Close()

	const clientID = "11111111-1111-1111-1111-111111111111"
	const username = "alice@contoso.test"

	form := url.Values{
		"grant_type": {"password"},
		"client_id":  {clientID},
		"username":   {username},
		"password":   {"any"},
		"scope":      {"openid https://management.azure.com/.default"},
	}

	tr := postToken(t, srv, form)
	if tr.IDToken == "" {
		t.Fatalf("expected an id_token for an openid scope, got none")
	}
	c := decodeJWTPayload(t, tr.IDToken)

	if got := c["sub"]; got == clientID {
		t.Errorf("id_token sub = %v, must NOT equal client_id %q for a user flow", got, clientID)
	}
	if got := c["oid"]; got == clientID {
		t.Errorf("id_token oid = %v, must NOT equal client_id %q for a user flow", got, clientID)
	}
	if got := c["upn"]; got != username {
		t.Errorf("id_token upn = %v, want %q", got, username)
	}
	// appid stays the client id even on a user flow.
	if got := c["appid"]; got != clientID {
		t.Errorf("id_token appid = %v, want client_id %q", got, clientID)
	}

	// A second identical request must yield identical (deterministic) sub/oid.
	tr2 := postToken(t, srv, form)
	c2 := decodeJWTPayload(t, tr2.IDToken)
	if c["sub"] != c2["sub"] {
		t.Errorf("sub not deterministic across requests: %v vs %v", c["sub"], c2["sub"])
	}
	if c["oid"] != c2["oid"] {
		t.Errorf("oid not deterministic across requests: %v vs %v", c["oid"], c2["oid"])
	}
}

// TestTokenClientCredentialsSubject covers the client_credentials grant: with no
// username the access_token's sub stays the client id (unchanged behavior).
func TestTokenClientCredentialsSubject(t *testing.T) {
	s, err := New("")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	srv := httptest.NewServer(s)
	defer srv.Close()

	const clientID = "22222222-2222-2222-2222-222222222222"

	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientID},
		"client_secret": {"any"},
		"scope":         {"https://management.azure.com/.default"},
	}

	tr := postToken(t, srv, form)
	if tr.AccessToken == "" {
		t.Fatalf("expected an access_token, got none")
	}
	c := decodeJWTPayload(t, tr.AccessToken)
	if got := c["sub"]; got != clientID {
		t.Errorf("client_credentials access_token sub = %v, want client_id %q", got, clientID)
	}
	if got := c["oid"]; got != clientID {
		t.Errorf("client_credentials access_token oid = %v, want client_id %q", got, clientID)
	}
}
