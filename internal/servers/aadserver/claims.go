package aadserver

import "time"

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
func newClaims(issuer, audience, tenant, appID, name, upn string) claims {
	now := time.Now()
	return claims{
		Issuer:    issuer,
		Audience:  audience,
		Subject:   appID,
		Tenant:    tenant,
		AppID:     appID,
		ObjectID:  appID,
		Name:      name,
		UPN:       upn,
		Version:   "1.0",
		IssuedAt:  now.Unix(),
		NotBefore: now.Unix(),
		Expiry:    now.Add(time.Hour).Unix(),
	}
}
