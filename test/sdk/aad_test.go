package sdk

import (
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"

	"localaz.dev/internal/aadserver"
)

// newAAD spins up an in-process Entra ID emulator over TLS (azidentity refuses
// to send credentials over plain HTTP) and returns the test server.
func newAAD(t *testing.T) *httptest.Server {
	t.Helper()
	srv, err := aadserver.New()
	if err != nil {
		t.Fatalf("create aad server: %v", err)
	}
	ts := httptest.NewTLSServer(srv)
	t.Cleanup(ts.Close)
	return ts
}

// TestAADClientSecretCredential drives the emulator with the real azidentity
// ClientSecretCredential: discovery, token request and parsing all run through
// the same MSAL code path a production application uses.
func TestAADClientSecretCredential(t *testing.T) {
	ts := newAAD(t)

	opts := azcore.ClientOptions{
		Cloud:     cloud.Configuration{ActiveDirectoryAuthorityHost: ts.URL},
		Transport: ts.Client(),
	}
	cred, err := azidentity.NewClientSecretCredential(
		"adfs",
		"11111111-1111-1111-1111-111111111111",
		"localaz-secret",
		&azidentity.ClientSecretCredentialOptions{
			ClientOptions:            opts,
			DisableInstanceDiscovery: true,
		},
	)
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}

	tok, err := cred.GetToken(ctx(t), policy.TokenRequestOptions{
		Scopes: []string{"https://management.azure.com/.default"},
	})
	if err != nil {
		t.Fatalf("get token: %v", err)
	}
	if tok.Token == "" {
		t.Fatal("expected a non-empty access token")
	}

	claims := decodeJWTClaims(t, tok.Token)
	if got := claims["aud"]; got != "https://management.azure.com" {
		t.Fatalf("token audience = %v, want https://management.azure.com", got)
	}
	if got := claims["tid"]; got != "adfs" {
		t.Fatalf("token tenant = %v, want adfs", got)
	}
	if claims["iss"] == "" || claims["iss"] == nil {
		t.Fatal("token is missing an issuer claim")
	}
}

// decodeJWTClaims decodes the payload segment of a compact JWT.
func decodeJWTClaims(t *testing.T, token string) map[string]any {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token is not a compact JWT: %d segments", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode token payload: %v", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("unmarshal token payload: %v", err)
	}
	return claims
}
