package aadserver

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
)

// signer mints and signs the JWTs handed out by the token endpoint. A single
// RSA key is generated per process and published through the JWKS endpoint so
// that clients which validate token signatures (the Azure SDKs) accept them.
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
	// A stable, content-derived key id keeps the JWKS and token headers aligned.
	sum := sha256.Sum256(key.PublicKey.N.Bytes())
	return &signer{key: key, kid: base64.RawURLEncoding.EncodeToString(sum[:8])}, nil
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
