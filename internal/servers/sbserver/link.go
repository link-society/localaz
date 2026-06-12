package sbserver

import "sync"

const (
	// senderInitialCredit is the link-credit granted to a client-sender on
	// attach (and restored on each replenishing flow): the size of the client's
	// outstanding-transfer window.
	senderInitialCredit = 5000

	// senderCreditReplenishThreshold is the number of incoming transfers a
	// client-sender link may consume before the server emits a replenishing
	// flow that returns the window to senderInitialCredit. Replenishing well
	// before the window is exhausted keeps the client from ever blocking.
	senderCreditReplenishThreshold = 2500
)

// link is one attached AMQP link on a connection.
type link struct {
	handle  uint32
	channel uint16
	name    string
	address string

	// clientIsSender is true when the client attached as the sender (it sends
	// transfers to us); false when the client is the receiver (we send to it).
	clientIsSender bool
	isCBS          bool

	mu            sync.Mutex
	cond          *sync.Cond
	credit        uint32
	deliveryCount uint32
	closed        bool
	done          chan struct{}

	// received counts transfers accepted from a client-sender on this link,
	// driving credit replenishment.
	received uint32
	// sinceFlow counts transfers consumed since the last replenishing flow.
	sinceFlow uint32
	// partial buffers the body of a multi-frame delivery while more==true.
	partial []byte
}

func newLink() *link {
	l := &link{done: make(chan struct{})}
	l.cond = sync.NewCond(&l.mu)
	return l
}

// addCredit increases the link credit and wakes the delivery loop.
func (l *link) addCredit(delta uint32) {
	l.mu.Lock()
	l.credit += delta
	l.cond.Broadcast()
	l.mu.Unlock()
}

// drainCredit discards all outstanding credit (in response to a drain flow) and
// returns the current delivery count so the echo can report it.
func (l *link) drainCredit() uint32 {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.credit = 0
	return l.deliveryCount
}

// takeCredit blocks until the link has credit, then atomically consumes one
// unit and advances the delivery count. It returns false if the link closed
// before any credit became available. Holding the lock across the wait and the
// decrement ensures a concurrent drain can never be raced into a delivery.
func (l *link) takeCredit() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for l.credit == 0 && !l.closed {
		l.cond.Wait()
	}
	if l.closed {
		return false
	}
	l.credit--
	l.deliveryCount++
	return true
}

// appendPartial buffers a chunk of a multi-frame delivery's body.
func (l *link) appendPartial(chunk []byte) {
	l.mu.Lock()
	l.partial = append(l.partial, chunk...)
	l.mu.Unlock()
}

// takePartial appends the final chunk and returns the complete buffered body,
// resetting the buffer for the next delivery.
func (l *link) takePartial(final []byte) []byte {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.partial) == 0 {
		// Single-frame delivery: avoid an allocation, return the chunk as-is.
		return final
	}
	msg := append(l.partial, final...)
	l.partial = nil
	return msg
}

// recordReceived counts one completed incoming transfer on a client-sender
// link. It returns whether a replenishing flow should be emitted and, if so, the
// total received count to advertise as the flow's delivery-count.
func (l *link) recordReceived() (replenish bool, deliveryCount uint32) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.received++
	l.sinceFlow++
	if l.sinceFlow >= senderCreditReplenishThreshold {
		l.sinceFlow = 0
		return true, l.received
	}
	return false, l.received
}

func (l *link) close() {
	l.mu.Lock()
	if !l.closed {
		l.closed = true
		close(l.done)
	}
	l.cond.Broadcast()
	l.mu.Unlock()
}
