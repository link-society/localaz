package aadserver

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// claims is the minimal set of JWT claims the emulator emits. It is a faithful
// shape (issuer, audience, subject, tenant, expiry) without being a real,
// verifiable Entra ID assertion.
type claims struct {
	Issuer    string `json:"iss"`
	Audience  string `json:"aud"`
	Subject   string `json:"sub"`
	Tenant    string `json:"tid"`
	AppID     string `json:"appid"`
	ObjectID  string `json:"oid"`
	Name      string `json:"name,omitempty"`
	UPN       string `json:"upn,omitempty"`
	Version   string `json:"ver"`
	IssuedAt  int64  `json:"iat"`
	NotBefore int64  `json:"nbf"`
	Expiry    int64  `json:"exp"`
}

// newClaims builds a claims set for the given audience, tenant and identity.
//
// On a user/delegated flow (ROPC, auth code) a username is supplied via upn;
// the subject and object id must then identify the USER, not the application —
// otherwise an OIDC consumer keying on sub/oid always sees the client id. We
// derive those from the username deterministically so they are stable across
// calls and clearly distinct from the client id. On client_credentials (no
// username) the subject and object id remain the app/client id.
func newClaims(issuer, audience, tenant, appID, name, upn string) claims {
	now := time.Now()
	subject, objectID := appID, appID
	if upn != "" {
		subject = upn
		objectID = userObjectID(upn)
	}
	return claims{
		Issuer:    issuer,
		Audience:  audience,
		Subject:   subject,
		Tenant:    tenant,
		AppID:     appID,
		ObjectID:  objectID,
		Name:      name,
		UPN:       upn,
		Version:   "1.0",
		IssuedAt:  now.Unix(),
		NotBefore: now.Unix(),
		Expiry:    now.Add(time.Hour).Unix(),
	}
}

// userObjectID maps a username to a stable, GUID-shaped object id derived from
// sha256(username). It is deterministic across calls and never collides with a
// real client id.
func userObjectID(upn string) string {
	sum := sha256.Sum256([]byte(upn))
	h := hex.EncodeToString(sum[:16])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}
