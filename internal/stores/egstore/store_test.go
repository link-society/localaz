package egstore

import (
	"encoding/json"
	"testing"
	"time"
)

// TestReceiveRedeliversExpiredLock verifies that a locked delivery whose lock
// deadline has passed is returned to the available queue on the next Receive
// and handed out again with an incremented DeliveryCount.
func TestReceiveRedeliversExpiredLock(t *testing.T) {
	s := New()

	base := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	now := base
	s.now = func() time.Time { return now }

	s.Publish("topic", []json.RawMessage{json.RawMessage(`{"id":"e1"}`)})

	first := s.Receive("topic", "sub", 1)
	if len(first) != 1 {
		t.Fatalf("first Receive: got %d events, want 1", len(first))
	}
	if first[0].DeliveryCount != 1 {
		t.Fatalf("first DeliveryCount = %d, want 1", first[0].DeliveryCount)
	}

	// Before the lock expires, the event must not be redelivered.
	now = base.Add(s.lockDuration / 2)
	if got := s.Receive("topic", "sub", 1); len(got) != 0 {
		t.Fatalf("Receive before expiry: got %d events, want 0 (still locked)", len(got))
	}

	// After the lock expires, the same event is redelivered with a higher count.
	now = base.Add(s.lockDuration + time.Second)
	second := s.Receive("topic", "sub", 1)
	if len(second) != 1 {
		t.Fatalf("Receive after expiry: got %d events, want 1 (redelivery)", len(second))
	}
	if string(second[0].Event) != string(first[0].Event) {
		t.Fatalf("redelivered event = %s, want %s", second[0].Event, first[0].Event)
	}
	if second[0].DeliveryCount != 2 {
		t.Fatalf("redelivered DeliveryCount = %d, want 2", second[0].DeliveryCount)
	}
}

// TestRenewLocksExtendsDeadline verifies that RenewLocks moves the lock
// deadline forward so the event is not redelivered at what would have been the
// original expiry.
func TestRenewLocksExtendsDeadline(t *testing.T) {
	s := New()

	base := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	now := base
	s.now = func() time.Time { return now }

	s.Publish("topic", []json.RawMessage{json.RawMessage(`{"id":"e1"}`)})

	first := s.Receive("topic", "sub", 1)
	if len(first) != 1 {
		t.Fatalf("first Receive: got %d events, want 1", len(first))
	}
	token := first[0].LockToken

	// Renew just before the original deadline; the new deadline is now+lockDuration.
	now = base.Add(s.lockDuration - time.Second)
	res := s.RenewLocks("topic", "sub", []string{token})
	if len(res.Succeeded) != 1 || len(res.Failed) != 0 {
		t.Fatalf("RenewLocks = %+v, want 1 succeeded / 0 failed", res)
	}

	// At what would have been the original expiry, the renewed lock still holds.
	now = base.Add(s.lockDuration + time.Second)
	if got := s.Receive("topic", "sub", 1); len(got) != 0 {
		t.Fatalf("Receive after renew (pre-renewed-deadline): got %d events, want 0", len(got))
	}

	// Past the renewed deadline, the event is finally redelivered.
	now = base.Add(2*s.lockDuration + 2*time.Second)
	again := s.Receive("topic", "sub", 1)
	if len(again) != 1 {
		t.Fatalf("Receive past renewed deadline: got %d events, want 1", len(again))
	}
	if again[0].DeliveryCount != 2 {
		t.Fatalf("DeliveryCount = %d, want 2", again[0].DeliveryCount)
	}
}

// TestRenewLocksFailsUnknownToken verifies the validation behavior is retained.
func TestRenewLocksFailsUnknownToken(t *testing.T) {
	s := New()
	s.Publish("topic", []json.RawMessage{json.RawMessage(`{"id":"e1"}`)})
	_ = s.Receive("topic", "sub", 1)

	res := s.RenewLocks("topic", "sub", []string{"deadbeef"})
	if len(res.Succeeded) != 0 || len(res.Failed) != 1 {
		t.Fatalf("RenewLocks unknown token = %+v, want 0 succeeded / 1 failed", res)
	}
}
