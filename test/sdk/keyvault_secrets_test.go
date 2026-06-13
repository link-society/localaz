package sdk

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
)

func TestKeyVaultSecretSetGet(t *testing.T) {
	client := newKeyVault(t)
	c := ctx(t)

	if _, err := client.SetSecret(c, "db-password", azsecrets.SetSecretParameters{
		Value:       to.Ptr("s3cr3t"),
		ContentType: to.Ptr("text/plain"),
		Tags:        map[string]*string{"env": to.Ptr("dev")},
	}, nil); err != nil {
		t.Fatalf("set secret: %v", err)
	}

	got, err := client.GetSecret(c, "db-password", "", nil)
	if err != nil {
		t.Fatalf("get secret: %v", err)
	}
	if got.Value == nil || *got.Value != "s3cr3t" {
		t.Fatalf("value = %v, want s3cr3t", got.Value)
	}
	if got.ContentType == nil || *got.ContentType != "text/plain" {
		t.Fatalf("contentType = %v, want text/plain", got.ContentType)
	}
	if got.ID.Name() != "db-password" {
		t.Fatalf("id name = %q, want db-password", got.ID.Name())
	}
	if v := got.Tags["env"]; v == nil || *v != "dev" {
		t.Fatalf("tag env = %v, want dev", v)
	}
}

func TestKeyVaultSecretUpdate(t *testing.T) {
	client := newKeyVault(t)
	c := ctx(t)

	if _, err := client.SetSecret(c, "api-key", azsecrets.SetSecretParameters{
		Value: to.Ptr("v1"),
	}, nil); err != nil {
		t.Fatalf("set secret: %v", err)
	}

	updated, err := client.UpdateSecretProperties(c, "api-key", "", azsecrets.UpdateSecretPropertiesParameters{
		ContentType:      to.Ptr("application/json"),
		SecretAttributes: &azsecrets.SecretAttributes{Enabled: to.Ptr(false)},
	}, nil)
	if err != nil {
		t.Fatalf("update secret: %v", err)
	}
	if updated.ContentType == nil || *updated.ContentType != "application/json" {
		t.Fatalf("contentType = %v, want application/json", updated.ContentType)
	}
	if updated.Attributes == nil || updated.Attributes.Enabled == nil || *updated.Attributes.Enabled {
		t.Fatalf("enabled = %v, want false", updated.Attributes)
	}

	got, err := client.GetSecret(c, "api-key", "", nil)
	if err != nil {
		t.Fatalf("get secret: %v", err)
	}
	if got.Value == nil || *got.Value != "v1" {
		t.Fatalf("value = %v, want v1 (value is immutable)", got.Value)
	}
}

func TestKeyVaultListSecretsAndVersions(t *testing.T) {
	client := newKeyVault(t)
	c := ctx(t)

	if _, err := client.SetSecret(c, "alpha", azsecrets.SetSecretParameters{Value: to.Ptr("a1")}, nil); err != nil {
		t.Fatalf("set alpha v1: %v", err)
	}
	if _, err := client.SetSecret(c, "alpha", azsecrets.SetSecretParameters{Value: to.Ptr("a2")}, nil); err != nil {
		t.Fatalf("set alpha v2: %v", err)
	}
	if _, err := client.SetSecret(c, "beta", azsecrets.SetSecretParameters{Value: to.Ptr("b1")}, nil); err != nil {
		t.Fatalf("set beta: %v", err)
	}

	names := map[string]bool{}
	pager := client.NewListSecretPropertiesPager(nil)
	for pager.More() {
		page, err := pager.NextPage(c)
		if err != nil {
			t.Fatalf("list secrets: %v", err)
		}
		for _, item := range page.Value {
			names[item.ID.Name()] = true
		}
	}
	if !names["alpha"] || !names["beta"] || len(names) != 2 {
		t.Fatalf("listed secrets = %v, want alpha and beta", names)
	}

	versions := 0
	vpager := client.NewListSecretPropertiesVersionsPager("alpha", nil)
	for vpager.More() {
		page, err := vpager.NextPage(c)
		if err != nil {
			t.Fatalf("list versions: %v", err)
		}
		versions += len(page.Value)
	}
	if versions != 2 {
		t.Fatalf("alpha versions = %d, want 2", versions)
	}
}

func TestKeyVaultDeleteSecret(t *testing.T) {
	client := newKeyVault(t)
	c := ctx(t)

	if _, err := client.SetSecret(c, "temp", azsecrets.SetSecretParameters{Value: to.Ptr("x")}, nil); err != nil {
		t.Fatalf("set secret: %v", err)
	}
	if _, err := client.DeleteSecret(c, "temp", nil); err != nil {
		t.Fatalf("delete secret: %v", err)
	}
	if _, err := client.GetSecret(c, "temp", "", nil); err == nil {
		t.Fatal("expected error getting deleted secret, got nil")
	}
}
