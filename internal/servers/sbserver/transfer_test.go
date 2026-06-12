package sbserver

import (
	"bytes"
	"net"
	"testing"

	"localaz.dev/internal/stores/sbstore"
)

// captureConn is a net.Conn whose writes are recorded into a buffer so tests can
// inspect the frames the server emitted. Reads are unused.
type captureConn struct {
	net.Conn
	written bytes.Buffer
}

func (c *captureConn) Write(b []byte) (int, error) { return c.written.Write(b) }
func (c *captureConn) Close() error                { return nil }

// newTestConn builds a conn wired to a real broker with a capturing writer.
func newTestConn(broker *sbstore.Broker) (*conn, *captureConn) {
	cc := &captureConn{}
	c := newConn(cc, broker)
	return c, cc
}

// transferFrame builds a transfer performative frame (described list) followed
// by the message payload, matching what readFrame produces, so it can be fed to
// onTransfer. more controls performative field index 5.
func transferFrame(channel uint16, handle, deliveryID uint32, more bool, payload []byte) *frame {
	return &frame{
		typ:     frameTypeAMQP,
		channel: channel,
		code:    descTransfer,
		fields: []any{
			handle,     // 0 handle
			deliveryID, // 1 delivery-id
			[]byte{1},  // 2 delivery-tag
			uint32(0),  // 3 message-format
			false,      // 4 settled
			more,       // 5 more
		},
		payload: payload,
	}
}

// drainFrames decodes every AMQP frame the conn has written so far.
func drainFrames(t *testing.T, cc *captureConn) []*frame {
	t.Helper()
	var out []*frame
	r := bytes.NewReader(cc.written.Bytes())
	for r.Len() > 0 {
		f, err := readFrame(r)
		if err != nil {
			t.Fatalf("readFrame while draining: %v", err)
		}
		out = append(out, f)
	}
	return out
}

// TestOnTransferReassemblesMultiFrame feeds two transfer frames for one delivery
// (first more=true, second more=false) and asserts the broker receives ONE
// message equal to the concatenation of both payloads, not two fragments.
//
// Against the pre-fix code (which ignores the more field) onTransfer would Send
// each frame separately, so the broker would hold two fragmented messages and
// this test fails.
func TestOnTransferReassemblesMultiFrame(t *testing.T) {
	broker := sbstore.New()
	c, _ := newTestConn(broker)

	const channel = uint16(0)
	const handle = uint32(0)
	const addr = "myqueue"

	l := newLink()
	l.handle = handle
	l.channel = channel
	l.address = addr
	l.clientIsSender = true
	c.links[linkKey(channel, handle)] = l

	first := []byte("hello ")
	second := []byte("world")

	if err := c.onTransfer(transferFrame(channel, handle, 1, true, first)); err != nil {
		t.Fatalf("onTransfer (more=true): %v", err)
	}
	if err := c.onTransfer(transferFrame(channel, handle, 1, false, second)); err != nil {
		t.Fatalf("onTransfer (more=false): %v", err)
	}

	got := broker.Dequeue(addr)
	if got == nil {
		t.Fatalf("broker received no message")
	}
	want := append(append([]byte{}, first...), second...)
	if !bytes.Equal(got, want) {
		t.Fatalf("reassembled message = %q, want %q", got, want)
	}
	// There must be exactly one message: a second Dequeue must be empty,
	// proving fragments were not delivered separately.
	if extra := broker.Dequeue(addr); extra != nil {
		t.Fatalf("expected exactly one message, got an extra fragment %q", extra)
	}
}

// TestOnTransferSettlesOnceWhenComplete asserts that for a multi-frame delivery
// the server settles (sends a disposition) exactly once, on completion — not on
// each intermediate frame.
func TestOnTransferSettlesOnceWhenComplete(t *testing.T) {
	broker := sbstore.New()
	c, cc := newTestConn(broker)

	const channel = uint16(0)
	const handle = uint32(0)

	l := newLink()
	l.handle = handle
	l.channel = channel
	l.address = "q"
	l.clientIsSender = true
	c.links[linkKey(channel, handle)] = l

	// First (more=true) frame must NOT settle.
	if err := c.onTransfer(transferFrame(channel, handle, 7, true, []byte("ab"))); err != nil {
		t.Fatalf("onTransfer (more=true): %v", err)
	}
	for _, f := range drainFrames(t, cc) {
		if f.code == descDisposition {
			t.Fatalf("disposition emitted on intermediate (more=true) frame")
		}
	}

	// Completing frame must settle exactly once.
	cc.written.Reset()
	if err := c.onTransfer(transferFrame(channel, handle, 7, false, []byte("cd"))); err != nil {
		t.Fatalf("onTransfer (more=false): %v", err)
	}
	dispositions := 0
	for _, f := range drainFrames(t, cc) {
		if f.code == descDisposition {
			dispositions++
		}
	}
	if dispositions != 1 {
		t.Fatalf("disposition count on completion = %d, want 1", dispositions)
	}
}

// TestOnTransferReplenishesCredit asserts that after enough transfers on a
// client-sender link a replenishing flow frame is emitted so the client's
// link-credit window is restored (otherwise go-amqp stalls after the initial
// window is consumed).
//
// Against the pre-fix code onTransfer never emits a flow, so no flow frame is
// found and this test fails.
func TestOnTransferReplenishesCredit(t *testing.T) {
	broker := sbstore.New()
	c, cc := newTestConn(broker)

	const channel = uint16(0)
	const handle = uint32(0)

	l := newLink()
	l.handle = handle
	l.channel = channel
	l.address = "q"
	l.clientIsSender = true
	c.links[linkKey(channel, handle)] = l

	// Send exactly enough single-frame transfers to cross the replenish
	// threshold once.
	for i := 0; i < senderCreditReplenishThreshold; i++ {
		f := transferFrame(channel, handle, uint32(i+1), false, []byte("x"))
		if err := c.onTransfer(f); err != nil {
			t.Fatalf("onTransfer %d: %v", i, err)
		}
	}

	var flow *frame
	for _, f := range drainFrames(t, cc) {
		if f.code == descFlow {
			flow = f
		}
	}
	if flow == nil {
		t.Fatalf("no replenishing flow emitted after %d transfers", senderCreditReplenishThreshold)
	}

	// The flow must target this link's handle.
	if h := asUint32(flow.field(4)); h != handle {
		t.Fatalf("flow handle = %d, want %d", h, handle)
	}
	// delivery-count must equal the number of transfers received so the
	// client's recomputed window (delivery-count + link-credit - sent) is
	// consistent and does not stall.
	if dc := asUint32(flow.field(5)); dc != uint32(senderCreditReplenishThreshold) {
		t.Fatalf("flow delivery-count = %d, want %d", dc, senderCreditReplenishThreshold)
	}
	// link-credit must restore the window back to the initial size.
	if lc := asUint32(flow.field(6)); lc != senderInitialCredit {
		t.Fatalf("flow link-credit = %d, want %d", lc, senderInitialCredit)
	}
}

// TestOnTransferNoEarlyReplenish guards the threshold: a single transfer must
// NOT trigger a flow (which would otherwise flood the wire with flows).
func TestOnTransferNoEarlyReplenish(t *testing.T) {
	broker := sbstore.New()
	c, cc := newTestConn(broker)

	const channel = uint16(0)
	const handle = uint32(0)

	l := newLink()
	l.handle = handle
	l.channel = channel
	l.address = "q"
	l.clientIsSender = true
	c.links[linkKey(channel, handle)] = l

	if err := c.onTransfer(transferFrame(channel, handle, 1, false, []byte("x"))); err != nil {
		t.Fatalf("onTransfer: %v", err)
	}
	for _, f := range drainFrames(t, cc) {
		if f.code == descFlow {
			t.Fatalf("flow emitted after a single transfer")
		}
	}
}
