package sdk

import (
	"net/http/httptest"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azqueue"

	"localaz.dev/internal/queueserver"
	"localaz.dev/internal/queuestore"
)

// newQueueClient spins up an in-process Queue emulator and returns a service
// client pointed at it.
func newQueueClient(t *testing.T) *azqueue.ServiceClient {
	t.Helper()
	store, err := queuestore.New(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	ts := httptest.NewServer(queueserver.New(store))
	t.Cleanup(ts.Close)

	client, err := azqueue.NewServiceClientWithNoCredential(ts.URL+"/devstoreaccount1", nil)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	return client
}

func TestQueueSendReceiveDelete(t *testing.T) {
	svc := newQueueClient(t)
	c := ctx(t)

	qc := svc.NewQueueClient("work-items")
	if _, err := qc.Create(c, nil); err != nil {
		t.Fatalf("create queue: %v", err)
	}

	if _, err := qc.EnqueueMessage(c, "hello queue", nil); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	resp, err := qc.DequeueMessage(c, nil)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if len(resp.Messages) != 1 {
		t.Fatalf("dequeued %d messages, want 1", len(resp.Messages))
	}
	got := resp.Messages[0]
	if got.MessageText == nil || *got.MessageText != "hello queue" {
		t.Fatalf("message text = %v, want %q", got.MessageText, "hello queue")
	}

	if _, err := qc.DeleteMessage(c, *got.MessageID, *got.PopReceipt, nil); err != nil {
		t.Fatalf("delete message: %v", err)
	}

	// The queue must now be empty.
	peek, err := qc.PeekMessage(c, nil)
	if err != nil {
		t.Fatalf("peek: %v", err)
	}
	if len(peek.Messages) != 0 {
		t.Fatalf("peeked %d messages after delete, want 0", len(peek.Messages))
	}
}

func TestQueueVisibilityTimeout(t *testing.T) {
	svc := newQueueClient(t)
	c := ctx(t)

	qc := svc.NewQueueClient("visibility")
	if _, err := qc.Create(c, nil); err != nil {
		t.Fatalf("create queue: %v", err)
	}
	if _, err := qc.EnqueueMessage(c, "later", nil); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// Dequeue with a long visibility timeout so the message is hidden.
	vis := int32(30)
	if _, err := qc.DequeueMessage(c, &azqueue.DequeueMessageOptions{VisibilityTimeout: &vis}); err != nil {
		t.Fatalf("dequeue: %v", err)
	}

	// A second dequeue must see nothing while the first lease is held.
	resp, err := qc.DequeueMessage(c, nil)
	if err != nil {
		t.Fatalf("second dequeue: %v", err)
	}
	if len(resp.Messages) != 0 {
		t.Fatalf("second dequeue returned %d messages, want 0 (still invisible)", len(resp.Messages))
	}
}

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

func ptr(s string) *string { return &s }
