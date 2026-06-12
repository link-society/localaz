package sdk

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
)

func TestContainerLifecycle(t *testing.T) {
	client := newClient(t)
	c := ctx(t)

	if _, err := client.CreateContainer(c, "lifecycle", nil); err != nil {
		t.Fatalf("create container: %v", err)
	}

	// Creating the same container again must report the Azure conflict code.
	_, err := client.CreateContainer(c, "lifecycle", nil)
	if !bloberror.HasCode(err, bloberror.ContainerAlreadyExists) {
		t.Fatalf("expected ContainerAlreadyExists, got %v", err)
	}

	// The container must show up in the account listing.
	found := false
	pager := client.NewListContainersPager(nil)
	for pager.More() {
		page, err := pager.NextPage(c)
		if err != nil {
			t.Fatalf("list containers: %v", err)
		}
		for _, item := range page.ContainerItems {
			if item.Name != nil && *item.Name == "lifecycle" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("created container not present in listing")
	}

	if _, err := client.DeleteContainer(c, "lifecycle", nil); err != nil {
		t.Fatalf("delete container: %v", err)
	}
}

func TestOperationsOnMissingContainer(t *testing.T) {
	client := newClient(t)
	c := ctx(t)
	_, err := client.UploadBuffer(c, "ghost", "x", []byte("y"), nil)
	if !bloberror.HasCode(err, bloberror.ContainerNotFound) {
		t.Fatalf("expected ContainerNotFound, got %v", err)
	}
}
