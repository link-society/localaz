package egstore

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
)

// topic is a named Event Grid topic. backlog retains every published event so
// that subscriptions created later still receive earlier events; subs holds the
// per-subscription delivery queues.
type topic struct {
	backlog []json.RawMessage
	subs    map[string]*subscription
}

// getSub returns the named subscription, creating it (seeded from the topic
// backlog) on first reference.
func (t *topic) getSub(name string) *subscription {
	sub, ok := t.subs[name]
	if !ok {
		sub = &subscription{locked: make(map[string]*delivery)}
		for _, raw := range t.backlog {
			sub.available = append(sub.available, &delivery{event: raw})
		}
		t.subs[name] = sub
	}
	return sub
}

// subscription is a pull-delivery cursor over a topic. available holds events
// ready to be received; locked holds events handed out under a lock token,
// awaiting acknowledge / release / reject.
type subscription struct {
	available []*delivery
	locked    map[string]*delivery
}

// delivery couples a stored event with its current delivery count.
type delivery struct {
	event         json.RawMessage
	deliveryCount int
}

// Received is a single locked event returned from Receive.
type Received struct {
	LockToken     string
	DeliveryCount int
	Event         json.RawMessage
}

// LockResult reports the outcome of a token-based operation (acknowledge,
// release, reject, renew).
type LockResult struct {
	Succeeded []string
	Failed    []string
}

// newLockToken returns a random opaque lock token.
func newLockToken() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
