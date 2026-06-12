// Package egstore is an in-memory pub/sub store backing the localaz Event Grid
// namespace emulator. Topics fan out published CloudEvents to per-subscription
// queues, and subscriptions hand out lock tokens for the pull-delivery model
// (receive / acknowledge / release / reject / renew).
package egstore

import (
	"encoding/json"
	"sync"
	"time"
)

// defaultLockDuration is how long a Receive holds an event under its lock token
// before the lock expires and the event becomes eligible for redelivery. It
// mirrors the Event Grid pull-delivery default visibility window.
const defaultLockDuration = 5 * time.Minute

// Store holds all Event Grid topics for a single namespace. It is safe for
// concurrent use.
type Store struct {
	mu     sync.Mutex
	topics map[string]*topic

	// now is the clock used for lock deadlines; injectable for tests.
	now func() time.Time
	// lockDuration is how long a locked delivery is held before its lock
	// expires and it is returned to the available queue for redelivery.
	lockDuration time.Duration
}

// New constructs an empty Store.
func New() *Store {
	return &Store{
		topics:       make(map[string]*topic),
		now:          func() time.Time { return time.Now().UTC() },
		lockDuration: defaultLockDuration,
	}
}

// getTopic returns the named topic, creating it on first reference.
func (s *Store) getTopic(name string) *topic {
	t, ok := s.topics[name]
	if !ok {
		t = &topic{subs: make(map[string]*subscription)}
		s.topics[name] = t
	}
	return t
}

// Publish appends each raw CloudEvent to the topic backlog and to every
// existing subscription's queue.
func (s *Store) Publish(topicName string, events []json.RawMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t := s.getTopic(topicName)
	for _, raw := range events {
		stored := append(json.RawMessage(nil), raw...)
		t.backlog = append(t.backlog, stored)
		for _, sub := range t.subs {
			sub.available = append(sub.available, &delivery{event: stored})
		}
	}
}

// Receive locks up to max events from the subscription and returns them. A
// subscription is created on first reference, seeded with the topic backlog so
// that events published before the subscription existed are still delivered.
func (s *Store) Receive(topicName, subName string, max int) []Received {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub := s.getTopic(topicName).getSub(subName)
	if max <= 0 {
		max = 1
	}

	now := s.now()
	s.sweepExpired(sub, now)

	out := make([]Received, 0, max)
	for len(out) < max && len(sub.available) > 0 {
		d := sub.available[0]
		sub.available = sub.available[1:]
		d.deliveryCount++
		d.lockDeadline = now.Add(s.lockDuration)
		token := newLockToken()
		sub.locked[token] = d
		out = append(out, Received{
			LockToken:     token,
			DeliveryCount: d.deliveryCount,
			Event:         d.event,
		})
	}
	return out
}

// Acknowledge removes locked events permanently. Unknown tokens are reported as
// failed.
func (s *Store) Acknowledge(topicName, subName string, tokens []string) LockResult {
	return s.resolve(topicName, subName, tokens, func(*subscription, *delivery) {})
}

// Reject drops locked events (dead-letter). Unknown tokens are reported as
// failed.
func (s *Store) Reject(topicName, subName string, tokens []string) LockResult {
	return s.resolve(topicName, subName, tokens, func(*subscription, *delivery) {})
}

// Release returns locked events to the available queue for redelivery.
func (s *Store) Release(topicName, subName string, tokens []string) LockResult {
	return s.resolve(topicName, subName, tokens, func(sub *subscription, d *delivery) {
		sub.available = append(sub.available, d)
	})
}

// RenewLocks extends each lock's deadline by lockDuration from now, keeping the
// events locked past their original expiry. Unknown tokens are reported as
// failed.
func (s *Store) RenewLocks(topicName, subName string, tokens []string) LockResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub := s.getTopic(topicName).getSub(subName)
	deadline := s.now().Add(s.lockDuration)
	var res LockResult
	for _, tok := range tokens {
		if d, ok := sub.locked[tok]; ok {
			d.lockDeadline = deadline
			res.Succeeded = append(res.Succeeded, tok)
		} else {
			res.Failed = append(res.Failed, tok)
		}
	}
	return res
}

// resolve removes the given locked tokens, invoking onRemove for each found
// token after unlocking it.
func (s *Store) resolve(topicName, subName string, tokens []string, onRemove func(*subscription, *delivery)) LockResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub := s.getTopic(topicName).getSub(subName)
	var res LockResult
	for _, tok := range tokens {
		d, ok := sub.locked[tok]
		if !ok {
			res.Failed = append(res.Failed, tok)
			continue
		}
		delete(sub.locked, tok)
		onRemove(sub, d)
		res.Succeeded = append(res.Succeeded, tok)
	}
	return res
}

// sweepExpired returns every locked delivery whose lock deadline has passed to
// the subscription's available queue, freeing strands left by consumers that
// received but never acknowledged or released. Called lazily on Receive so no
// background goroutine is needed.
func (s *Store) sweepExpired(sub *subscription, now time.Time) {
	for tok, d := range sub.locked {
		if now.After(d.lockDeadline) {
			delete(sub.locked, tok)
			d.lockDeadline = time.Time{}
			sub.available = append(sub.available, d)
		}
	}
}
