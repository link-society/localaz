package sdk

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

// TestARMResourceGroupLifecycle exercises create/get/list/delete on resource
// groups through the official armresources SDK client.
func TestARMResourceGroupLifecycle(t *testing.T) {
	_, opts := newARM(t)

	client, err := armresources.NewResourceGroupsClient(testSubscriptionID, fakeCredential{}, opts)
	if err != nil {
		t.Fatalf("create resource groups client: %v", err)
	}

	created, err := client.CreateOrUpdate(ctx(t), "rg1", armresources.ResourceGroup{
		Location: to.Ptr("localaz"),
		Tags:     map[string]*string{"env": to.Ptr("test")},
	}, nil)
	if err != nil {
		t.Fatalf("create resource group: %v", err)
	}
	if created.Name == nil || *created.Name != "rg1" {
		t.Fatalf("created group name = %v, want rg1", created.Name)
	}
	if created.Properties == nil || created.Properties.ProvisioningState == nil ||
		*created.Properties.ProvisioningState != "Succeeded" {
		t.Fatal("created group provisioning state is not Succeeded")
	}

	got, err := client.Get(ctx(t), "rg1", nil)
	if err != nil {
		t.Fatalf("get resource group: %v", err)
	}
	if got.Tags["env"] == nil || *got.Tags["env"] != "test" {
		t.Fatalf("group tags = %v, want env=test", got.Tags)
	}

	pager := client.NewListPager(nil)
	var names []string
	for pager.More() {
		page, err := pager.NextPage(ctx(t))
		if err != nil {
			t.Fatalf("list resource groups: %v", err)
		}
		for _, rg := range page.Value {
			if rg.Name != nil {
				names = append(names, *rg.Name)
			}
		}
	}
	if len(names) != 1 || names[0] != "rg1" {
		t.Fatalf("resource groups = %v, want [rg1]", names)
	}

	poller, err := client.BeginDelete(ctx(t), "rg1", nil)
	if err != nil {
		t.Fatalf("begin delete resource group: %v", err)
	}
	if _, err := poller.PollUntilDone(ctx(t), nil); err != nil {
		t.Fatalf("delete resource group: %v", err)
	}

	if _, err := client.Get(ctx(t), "rg1", nil); err == nil {
		t.Fatal("expected error getting deleted resource group")
	}
}
