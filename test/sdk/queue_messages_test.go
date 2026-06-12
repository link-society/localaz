package sdk

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azqueue"
)

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
