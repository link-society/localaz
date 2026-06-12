package aadserver

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNewPersistsSigningKey verifies that constructing a Server twice over the
// same data directory reuses the same signing key: the kid stays stable and a
// token minted by the first instance verifies against the second instance's
// JWKS. A per-process key (the old behaviour) would change the kid and break
// signature validation across restarts.
func TestNewPersistsSigningKey(t *testing.T) {
	dir := t.TempDir()

	first, err := New(dir)
	if err != nil {
		t.Fatalf("New(dir) first: %v", err)
	}
	firstKid := first.signer.kid
	token, err := first.signer.sign(newClaims("iss", "aud", "tenant", "app", "name", "upn"))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	second, err := New(dir)
	if err != nil {
		t.Fatalf("New(dir) second: %v", err)
	}
	secondKid := second.signer.kid

	if firstKid != secondKid {
		t.Fatalf("kid changed across restarts: %q != %q", firstKid, secondKid)
	}

	// The second server must be able to verify a token minted by the first,
	// rebuilding the public key from its published JWKS.
	pub := publicKeyFromJWKS(t, second)
	verifyRS256(t, pub, token)

	// A persisted key file must exist on disk.
	if _, err := os.Stat(filepath.Join(dir, "aad", "signing-key.pem")); err != nil {
		t.Fatalf("expected persisted key file: %v", err)
	}
}

// TestNewWithoutDataDirGeneratesEphemeralKey verifies that New("") still works
// and does not attempt to persist anything.
func TestNewWithoutDataDirGeneratesEphemeralKey(t *testing.T) {
	s, err := New("")
	if err != nil {
		t.Fatalf("New(\"\"): %v", err)
	}
	if s.signer == nil || s.signer.key == nil {
		t.Fatalf("expected a generated signer")
	}
}

// publicKeyFromJWKS reconstructs the RSA public key advertised by the server's
// JWKS endpoint.
func publicKeyFromJWKS(t *testing.T, s *Server) *rsa.PublicKey {
	t.Helper()
	keys := s.signer.jwks()["keys"]
	if len(keys) != 1 {
		t.Fatalf("expected one JWK, got %d", len(keys))
	}
	k := keys[0]
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		t.Fatalf("decode n: %v", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		t.Fatalf("decode e: %v", err)
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(new(big.Int).SetBytes(eBytes).Int64()),
	}
}

// verifyRS256 fails the test if token does not carry a valid RS256 signature
// over its header.payload made by pub.
func verifyRS256(t *testing.T, pub *rsa.PublicKey, token string) {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT segments, got %d", len(parts))
	}
	signingInput := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	digest := sha256.Sum256([]byte(signingInput))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sig); err != nil {
		t.Fatalf("token from first server failed verification against second server JWKS: %v", err)
	}
}
