package sdk

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azqueue"
)

func TestQueueListAndMetadata(t *testing.T) {
	svc := newQueueClient(t)
	c := ctx(t)

	qc := svc.NewQueueClient("metaq")
	if _, err := qc.Create(c, &azqueue.CreateOptions{Metadata: map[string]*string{"team": ptr("storage")}}); err != nil {
		t.Fatalf("create queue: %v", err)
	}

	props, err := qc.GetProperties(c, nil)
	if err != nil {
		t.Fatalf("get properties: %v", err)
	}
	if metadataValue(props.Metadata, "team") != "storage" {
		t.Fatalf("metadata team = %q, want storage", metadataValue(props.Metadata, "team"))
	}

	found := false
	pager := svc.NewListQueuesPager(nil)
	for pager.More() {
		page, err := pager.NextPage(c)
		if err != nil {
			t.Fatalf("list queues: %v", err)
		}
		for _, q := range page.Queues {
			if q.Name != nil && *q.Name == "metaq" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("queue metaq not found in listing")
	}
}
