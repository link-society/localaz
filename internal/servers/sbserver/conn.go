package sbserver

import (
	"bufio"
	"bytes"
	"net"
	"strings"
	"sync"

	"localaz.dev/internal/stores/sbstore"
)

// conn drives the AMQP 1.0 protocol for a single TCP connection: SASL
// negotiation, the open/begin/attach handshake, then transfer/flow handling.
type conn struct {
	netConn net.Conn
	reader  *bufio.Reader
	broker  *sbstore.Broker

	writeMu sync.Mutex

	linksMu        sync.Mutex
	links          map[uint64]*link
	nextDeliveryID uint32
}

func newConn(netConn net.Conn, broker *sbstore.Broker) *conn {
	return &conn{
		netConn: netConn,
		reader:  bufio.NewReader(netConn),
		broker:  broker,
		links:   make(map[uint64]*link),
	}
}

// linkKey combines a session channel and link handle into a single map key.
// AMQP handles are only unique within a session, so both are needed.
func linkKey(channel uint16, handle uint32) uint64 {
	return uint64(channel)<<32 | uint64(handle)
}

func (c *conn) serve() {
	defer c.netConn.Close()
	defer c.closeAllLinks()

	if err := c.negotiate(); err != nil {
		return
	}

	for {
		f, err := readFrame(c.reader)
		if err != nil {
			return
		}
		if len(f.fields) == 0 && f.code == 0 {
			continue // heartbeat / empty frame
		}
		if err := c.dispatch(f); err != nil {
			return
		}
	}
}

// negotiate performs the protocol header exchange and SASL ANONYMOUS handshake
// (which azservicebus always uses), then the AMQP header exchange.
func (c *conn) negotiate() error {
	hdr, err := readProtocolHeader(c.reader)
	if err != nil {
		return err
	}

	if hdr[4] == 0x03 { // SASL layer
		if err := c.rawWrite(saslHeader); err != nil {
			return err
		}
		if err := c.writeRaw(frameTypeSASL, 0, descSASLMechanisms, []any{symbol("ANONYMOUS")}, nil); err != nil {
			return err
		}
		// Expect sasl-init.
		if _, err := readFrame(c.reader); err != nil {
			return err
		}
		// sasl-outcome: code 0 (ok).
		if err := c.writeRaw(frameTypeSASL, 0, descSASLOutcome, []any{uint8(0)}, nil); err != nil {
			return err
		}
		// Now the real AMQP header.
		if _, err := readProtocolHeader(c.reader); err != nil {
			return err
		}
	}

	return c.rawWrite(amqpHeader)
}

func (c *conn) dispatch(f *frame) error {
	switch f.code {
	case descOpen:
		return c.onOpen(f)
	case descBegin:
		return c.onBegin(f)
	case descAttach:
		return c.onAttach(f)
	case descFlow:
		return c.onFlow(f)
	case descTransfer:
		return c.onTransfer(f)
	case descDisposition:
		return c.onDisposition(f)
	case descDetach:
		return c.onDetach(f)
	case descEnd:
		return c.writeAMQP(f.channel, descEnd, []any{}, nil)
	case descClose:
		_ = c.writeAMQP(0, descClose, []any{}, nil)
		return errClose
	default:
		return nil
	}
}

func (c *conn) onOpen(f *frame) error {
	return c.writeAMQP(0, descOpen, []any{
		"localaz",         // container-id
		nil,               // hostname
		uint32(1024 * 64), // max-frame-size
		uint16(65535),     // channel-max
	}, nil)
}

func (c *conn) onBegin(f *frame) error {
	return c.writeAMQP(f.channel, descBegin, []any{
		f.channel,     // remote-channel
		uint32(0),     // next-outgoing-id
		uint32(65535), // incoming-window
		uint32(65535), // outgoing-window
		uint32(65535), // handle-max
	}, nil)
}

func (c *conn) onAttach(f *frame) error {
	name, _ := f.field(0).(string)
	handle := asUint32(f.field(1))
	clientRole := asBool(f.field(2)) // false=client sender, true=client receiver
	sndSettle := f.field(3)
	rcvSettle := f.field(4)
	source := f.field(5)
	target := f.field(6)

	clientIsSender := !clientRole

	address := addressOf(target)
	if !clientIsSender {
		address = addressOf(source)
	}

	l := newLink()
	l.handle = handle
	l.channel = f.channel
	l.name = name
	l.address = address
	l.clientIsSender = clientIsSender
	l.isCBS = address == cbsAddress

	c.linksMu.Lock()
	c.links[linkKey(f.channel, handle)] = l
	c.linksMu.Unlock()

	// Register real subscription receivers (not CBS or $management links) so
	// topic sends fan out to them.
	if !clientIsSender && !l.isCBS && !strings.Contains(address, "$management") {
		c.broker.RegisterSubscription(address)
	}

	// Echo the attach. Our role is the opposite of the client's.
	ourRole := !clientRole
	fields := []any{
		name,
		handle,
		ourRole,
		sndSettle,
		rcvSettle,
		source,
		target,
		nil,       // unsettled
		false,     // incomplete-unsettled
		uint32(0), // initial-delivery-count
	}
	if err := c.writeAMQP(f.channel, descAttach, fields, nil); err != nil {
		return err
	}

	if clientIsSender {
		// We are the receiver: grant credit so the client can send.
		return c.grantCredit(f.channel, handle, 5000)
	}

	// CBS responses are delivered directly (see handleCBS); other receiver
	// links pull from the broker.
	if !l.isCBS {
		go c.deliverLoop(l)
	}
	return nil
}

// grantCredit sends a flow frame extending link-credit to the client.
func (c *conn) grantCredit(channel uint16, handle, credit uint32) error {
	return c.writeAMQP(channel, descFlow, []any{
		uint32(0),     // next-incoming-id
		uint32(65535), // incoming-window
		uint32(0),     // next-outgoing-id
		uint32(65535), // outgoing-window
		handle,        // handle
		uint32(0),     // delivery-count
		credit,        // link-credit
	}, nil)
}

func (c *conn) onFlow(f *frame) error {
	if f.field(4) == nil {
		return nil // connection/session level flow, no link handle
	}
	handle := asUint32(f.field(4))
	credit := asUint32(f.field(6))
	drain := asBool(f.field(8))

	c.linksMu.Lock()
	l := c.links[linkKey(f.channel, handle)]
	c.linksMu.Unlock()
	if l == nil || l.clientIsSender {
		return nil
	}

	if drain {
		// The client wants to drain: drop outstanding credit and echo a flow
		// reporting zero remaining credit so its drain completes.
		count := l.drainCredit()
		return c.writeAMQP(f.channel, descFlow, []any{
			uint32(0),     // next-incoming-id
			uint32(65535), // incoming-window
			uint32(0),     // next-outgoing-id
			uint32(65535), // outgoing-window
			handle,        // handle
			count,         // delivery-count
			uint32(0),     // link-credit
			uint32(0),     // available
			true,          // drain
		}, nil)
	}

	l.addCredit(credit)
	return nil
}

func (c *conn) onTransfer(f *frame) error {
	handle := asUint32(f.field(0))
	deliveryID := asUint32(f.field(1))

	c.linksMu.Lock()
	l := c.links[linkKey(f.channel, handle)]
	c.linksMu.Unlock()
	if l == nil {
		return nil
	}

	if l.isCBS {
		if err := c.handleCBS(l, f.payload); err != nil {
			return err
		}
	} else {
		c.broker.Send(l.address, f.payload)
	}

	// Settle the incoming delivery as accepted.
	return c.writeAMQP(f.channel, descDisposition, []any{
		true,       // role: receiver
		deliveryID, // first
		deliveryID, // last
		true,       // settled
		described{descriptor: uint64(descAccepted), value: []any{}},
	}, nil)
}

func (c *conn) onDetach(f *frame) error {
	handle := asUint32(f.field(0))
	key := linkKey(f.channel, handle)
	c.linksMu.Lock()
	if l := c.links[key]; l != nil {
		l.close()
		delete(c.links, key)
	}
	c.linksMu.Unlock()
	return c.writeAMQP(f.channel, descDetach, []any{handle, true}, nil)
}

// onDisposition handles the client settling one of our deliveries (e.g. via
// CompleteMessage). We confirm by sending our own settled disposition so a
// receiver operating in second settlement mode unblocks.
func (c *conn) onDisposition(f *frame) error {
	role := asBool(f.field(0)) // true = receiver
	if !role {
		return nil // sender-role disposition, nothing to confirm
	}
	first := asUint32(f.field(1))
	last := first
	if f.field(2) != nil {
		last = asUint32(f.field(2))
	}
	settled := asBool(f.field(3))
	if settled {
		return nil // client already settled; no confirmation needed
	}
	return c.writeAMQP(f.channel, descDisposition, []any{
		false, // role: sender
		first,
		last,
		true, // settled
		described{descriptor: uint64(descAccepted), value: []any{}},
	}, nil)
}

// deliverLoop pushes queued messages to a client receiver link as credit allows.
func (c *conn) deliverLoop(l *link) {
	var pending []byte
	for {
		if pending == nil {
			pending = c.broker.Dequeue(l.address)
			if pending == nil {
				select {
				case <-c.broker.WaitChan(l.address):
				case <-l.done:
					return
				}
				continue
			}
		}
		// Block until the client grants credit, then consume it atomically.
		// A pending message is held locally so a drain between dequeue and
		// delivery never loses or wrongly delivers it.
		if !l.takeCredit() {
			// Link closed with a message in hand: return it so a later
			// receiver on the same address still gets it.
			c.broker.Requeue(l.address, pending)
			return
		}
		if err := c.deliver(l, pending); err != nil {
			return
		}
		pending = nil
	}
}

func (c *conn) deliver(l *link, msg []byte) error {
	c.linksMu.Lock()
	deliveryID := c.nextDeliveryID
	c.nextDeliveryID++
	c.linksMu.Unlock()

	tag := []byte{byte(deliveryID), byte(deliveryID >> 8), byte(deliveryID >> 16), byte(deliveryID >> 24)}
	return c.writeAMQP(l.channel, descTransfer, []any{
		l.handle,   // handle
		deliveryID, // delivery-id
		tag,        // delivery-tag
		uint32(0),  // message-format
		false,      // settled (peek-lock: client settles)
		false,      // more
	}, msg)
}

func (c *conn) closeAllLinks() {
	c.linksMu.Lock()
	for _, l := range c.links {
		l.close()
	}
	c.linksMu.Unlock()
}

// rawWrite writes bytes directly (protocol headers) under the write lock.
func (c *conn) rawWrite(b []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err := c.netConn.Write(b)
	return err
}

func (c *conn) writeAMQP(channel uint16, code uint64, fields []any, payload []byte) error {
	return c.writeRaw(frameTypeAMQP, channel, code, fields, payload)
}

func (c *conn) writeRaw(typ byte, channel uint16, code uint64, fields []any, payload []byte) error {
	var buf bytes.Buffer
	if err := writeFrame(&buf, typ, channel, code, fields, payload); err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err := c.netConn.Write(buf.Bytes())
	return err
}
