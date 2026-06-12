package sbserver

import "sync"

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

func (l *link) close() {
	l.mu.Lock()
	if !l.closed {
		l.closed = true
		close(l.done)
	}
	l.cond.Broadcast()
	l.mu.Unlock()
}
