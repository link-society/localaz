package sdk

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
)

// TestARMListSubscriptions exercises the subscriptions endpoint through the
// official armsubscriptions SDK client.
func TestARMListSubscriptions(t *testing.T) {
	_, opts := newARM(t)

	client, err := armsubscriptions.NewClient(fakeCredential{}, opts)
	if err != nil {
		t.Fatalf("create subscriptions client: %v", err)
	}

	pager := client.NewListPager(nil)
	var ids []string
	for pager.More() {
		page, err := pager.NextPage(ctx(t))
		if err != nil {
			t.Fatalf("list subscriptions: %v", err)
		}
		for _, sub := range page.Value {
			if sub.SubscriptionID != nil {
				ids = append(ids, *sub.SubscriptionID)
			}
		}
	}
	if len(ids) != 1 || ids[0] != testSubscriptionID {
		t.Fatalf("subscriptions = %v, want [%s]", ids, testSubscriptionID)
	}
}
