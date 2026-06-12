package queuestore_test

import (
	"testing"

	"localaz.dev/internal/stores/queuestore"
)

// TestQueueSurvivesRestart enqueues a message, then opens a brand new store over
// the same directory and asserts the queue and message are read back.
func TestQueueSurvivesRestart(t *testing.T) {
	root := t.TempDir()

	s1, err := queuestore.New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := s1.CreateQueue("acct", "q1", map[string]string{"k": "v"}); err != nil {
		t.Fatalf("CreateQueue: %v", err)
	}
	if _, err := s1.Enqueue("acct", "q1", "hello", 0, -1); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	s2, err := queuestore.New(root)
	if err != nil {
		t.Fatalf("reopen New: %v", err)
	}
	info, err := s2.GetMetadata("acct", "q1")
	if err != nil {
		t.Fatalf("GetMetadata after restart: %v", err)
	}
	if info.ApproximateCount != 1 {
		t.Fatalf("message count = %d, want 1", info.ApproximateCount)
	}
	msgs, err := s2.Peek("acct", "q1", 10)
	if err != nil {
		t.Fatalf("Peek after restart: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Text != "hello" {
		t.Fatalf("peeked = %+v, want a single 'hello' message", msgs)
	}
}
