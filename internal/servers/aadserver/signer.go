package aadserver

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// signer mints and signs the JWTs handed out by the token endpoint. A single
// RSA key (persisted across restarts when a data dir is configured) is
// published through the JWKS endpoint so that clients which validate token
// signatures (the Azure SDKs) accept them.
type signer struct {
	key *rsa.PrivateKey
	kid string
}

// newSigner generates a fresh RSA signing key.
func newSigner() (*signer, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate signing key: %w", err)
	}
	return signerFromKey(key), nil
}

// signerFromKey wraps an RSA key in a signer, deriving the content-addressed
// key id from the modulus so the JWKS and token headers stay aligned (and so a
// persisted key always yields the same kid).
func signerFromKey(key *rsa.PrivateKey) *signer {
	sum := sha256.Sum256(key.PublicKey.N.Bytes())
	return &signer{key: key, kid: base64.RawURLEncoding.EncodeToString(sum[:8])}
}

// signerFromFileOrNew loads the PKCS#8 PEM signing key at path; if it does not
// exist, it generates a new key and persists it there (creating the parent
// directory) so the kid stays stable across restarts.
func signerFromFileOrNew(path string) (*signer, error) {
	pemBytes, err := os.ReadFile(path)
	switch {
	case err == nil:
		block, _ := pem.Decode(pemBytes)
		if block == nil {
			return nil, fmt.Errorf("parse signing key %s: no PEM block", path)
		}
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse signing key %s: %w", path, err)
		}
		key, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("signing key %s is not RSA", path)
		}
		return signerFromKey(key), nil
	case errors.Is(err, os.ErrNotExist):
		s, err := newSigner()
		if err != nil {
			return nil, err
		}
		der, err := x509.MarshalPKCS8PrivateKey(s.key)
		if err != nil {
			return nil, fmt.Errorf("marshal signing key: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create signing key dir: %w", err)
		}
		pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
		if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
			return nil, fmt.Errorf("write signing key: %w", err)
		}
		return s, nil
	default:
		return nil, fmt.Errorf("read signing key %s: %w", path, err)
	}
}

// sign serialises and RS256-signs the given claims, returning a compact JWT.
func (s *signer) sign(c claims) (string, error) {
	header := map[string]string{"typ": "JWT", "alg": "RS256", "kid": s.kid}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	payloadJSON, err := json.Marshal(c)
	if err != nil {
		return "", err
	}

	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(payloadJSON)

	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// jwk is one key in a JSON Web Key Set.
type jwk struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// jwks renders the public half of the signing key as a JSON Web Key Set.
func (s *signer) jwks() map[string][]jwk {
	pub := s.key.PublicKey
	eBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(eBytes, uint32(pub.E))
	for len(eBytes) > 1 && eBytes[0] == 0 {
		eBytes = eBytes[1:]
	}
	return map[string][]jwk{"keys": {{
		Kty: "RSA",
		Use: "sig",
		Kid: s.kid,
		Alg: "RS256",
		N:   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(eBytes),
	}}}
}
