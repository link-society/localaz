// Package sbstore is an in-memory broker for the Azure Service Bus emulator. It
// holds queues and topic/subscription fan-out. Messages are stored as opaque
// AMQP payloads so the protocol layer can relay them to receivers verbatim.
package sbstore

import (
	"strings"
	"sync"
)

// Broker holds queues and topic subscriptions in memory.
type Broker struct {
	mu        sync.Mutex
	queues    map[string][][]byte
	topicSubs map[string]map[string]struct{}
	waiters   map[string][]chan struct{}
}

// New constructs an empty Broker.
func New() *Broker {
	return &Broker{
		queues:    make(map[string][][]byte),
		topicSubs: make(map[string]map[string]struct{}),
		waiters:   make(map[string][]chan struct{}),
	}
}

const subscriptionInfix = "/subscriptions/"

// RegisterSubscription records that subAddress (of the form
// "<topic>/Subscriptions/<name>") exists so sends to its topic fan out to it.
func (b *Broker) RegisterSubscription(subAddress string) {
	idx := strings.Index(strings.ToLower(subAddress), subscriptionInfix)
	if idx < 0 {
		return
	}
	topic := subAddress[:idx]

	b.mu.Lock()
	defer b.mu.Unlock()
	subs, ok := b.topicSubs[topic]
	if !ok {
		subs = make(map[string]struct{})
		b.topicSubs[topic] = subs
	}
	subs[subAddress] = struct{}{}
}

// Send enqueues msg to address. If address is a topic with registered
// subscriptions, the message is copied to each subscription; otherwise address
// is treated as a queue.
func (b *Broker) Send(address string, msg []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if subs := b.topicSubs[address]; len(subs) > 0 {
		for sub := range subs {
			b.enqueueLocked(sub, msg)
		}
		return
	}
	b.enqueueLocked(address, msg)
}

func (b *Broker) enqueueLocked(address string, msg []byte) {
	cp := make([]byte, len(msg))
	copy(cp, msg)
	b.queues[address] = append(b.queues[address], cp)
	b.notifyLocked(address)
}

// Dequeue removes and returns the next message for address, or nil if empty.
func (b *Broker) Dequeue(address string) []byte {
	b.mu.Lock()
	defer b.mu.Unlock()

	q := b.queues[address]
	if len(q) == 0 {
		return nil
	}
	msg := q[0]
	b.queues[address] = q[1:]
	return msg
}

// Requeue returns a previously dequeued message to the front of address, used
// when a receiver link closes before the message could be delivered.
func (b *Broker) Requeue(address string, msg []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.queues[address] = append([][]byte{msg}, b.queues[address]...)
	b.notifyLocked(address)
}

// WaitChan returns a channel that is closed when a message becomes available
// for address. It must be called while the queue is observed empty.
func (b *Broker) WaitChan(address string) <-chan struct{} {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan struct{}, 1)
	if len(b.queues[address]) > 0 {
		// Already has data; fire immediately.
		close(ch)
		return ch
	}
	b.waiters[address] = append(b.waiters[address], ch)
	return ch
}

func (b *Broker) notifyLocked(address string) {
	for _, ch := range b.waiters[address] {
		close(ch)
	}
	delete(b.waiters, address)
}
