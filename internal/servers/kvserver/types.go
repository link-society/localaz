package kvserver

import (
	"time"

	"localaz.dev/internal/stores/keyvaultstore"
)

// recoveryLevel is reported on every secret. The emulator does soft-delete-free
// hard deletes, so secrets are purgeable and not recoverable.
const recoveryLevel = "Purgeable"

// secretAttributes is the Key Vault attributes object. Timestamps are Unix
// epoch seconds, as the secrets SDK expects on the wire.
type secretAttributes struct {
	Enabled       bool   `json:"enabled"`
	Created       int64  `json:"created,omitempty"`
	Updated       int64  `json:"updated,omitempty"`
	NotBefore     *int64 `json:"nbf,omitempty"`
	Expires       *int64 `json:"exp,omitempty"`
	RecoveryLevel string `json:"recoveryLevel,omitempty"`
}

// secretBundle is the JSON body returned for a single secret. Value is omitted
// for responses that do not carry it (list, update).
type secretBundle struct {
	ID          string            `json:"id,omitempty"`
	Value       string            `json:"value,omitempty"`
	ContentType string            `json:"contentType,omitempty"`
	Attributes  *secretAttributes `json:"attributes,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
}

// deletedSecretBundle is the body returned by Delete Secret. It embeds the
// secret fields and adds the deletion/recovery metadata.
type deletedSecretBundle struct {
	secretBundle
	RecoveryID         string `json:"recoveryId,omitempty"`
	DeletedDate        int64  `json:"deletedDate,omitempty"`
	ScheduledPurgeDate int64  `json:"scheduledPurgeDate,omitempty"`
}

// secretListResult is one page of a list operation.
type secretListResult struct {
	Value    []secretBundle `json:"value"`
	NextLink *string        `json:"nextLink"`
}

// setSecretRequest is the Set Secret request body.
type setSecretRequest struct {
	Value       string                `json:"value"`
	ContentType string                `json:"contentType"`
	Tags        map[string]string     `json:"tags"`
	Attributes  *secretAttributesBody `json:"attributes"`
}

// updateSecretRequest is the Update Secret request body. ContentType and Tags
// are pointers/maps so an absent field leaves the stored value untouched.
type updateSecretRequest struct {
	ContentType *string               `json:"contentType"`
	Tags        map[string]string     `json:"tags"`
	Attributes  *secretAttributesBody `json:"attributes"`
}

// secretAttributesBody is the attributes object accepted on set/update.
type secretAttributesBody struct {
	Enabled   *bool  `json:"enabled"`
	NotBefore *int64 `json:"nbf"`
	Expires   *int64 `json:"exp"`
}

func (b *secretAttributesBody) toAttributes() keyvaultstore.Attributes {
	var a keyvaultstore.Attributes
	if b == nil {
		return a
	}
	a.Enabled = b.Enabled
	if b.NotBefore != nil {
		t := time.Unix(*b.NotBefore, 0).UTC()
		a.NotBefore = &t
	}
	if b.Expires != nil {
		t := time.Unix(*b.Expires, 0).UTC()
		a.Expires = &t
	}
	return a
}

func attributesOf(sec keyvaultstore.Secret) *secretAttributes {
	a := &secretAttributes{
		Enabled:       sec.Enabled,
		Created:       sec.Created.Unix(),
		Updated:       sec.Updated.Unix(),
		RecoveryLevel: recoveryLevel,
	}
	if sec.NotBefore != nil {
		nb := sec.NotBefore.Unix()
		a.NotBefore = &nb
	}
	if sec.Expires != nil {
		exp := sec.Expires.Unix()
		a.Expires = &exp
	}
	return a
}

func bundleOf(base string, sec keyvaultstore.Secret, withValue bool) secretBundle {
	b := secretBundle{
		ID:          secretID(base, sec.Name, sec.Version),
		ContentType: sec.ContentType,
		Attributes:  attributesOf(sec),
		Tags:        sec.Tags,
	}
	if withValue {
		b.Value = sec.Value
	}
	return b
}

func secretID(base, name, version string) string {
	return base + "/secrets/" + name + "/" + version
}
