package sbserver

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// AMQP performative descriptor codes.
const (
	descOpen        = 0x10
	descBegin       = 0x11
	descAttach      = 0x12
	descFlow        = 0x13
	descTransfer    = 0x14
	descDisposition = 0x15
	descDetach      = 0x16
	descEnd         = 0x17
	descClose       = 0x18

	descSASLMechanisms = 0x40
	descSASLInit       = 0x41
	descSASLOutcome    = 0x44

	descError    = 0x1d
	descSource   = 0x28
	descTarget   = 0x29
	descAccepted = 0x24

	descHeader     = 0x70
	descProperties = 0x73
	descAppProps   = 0x74
	descData       = 0x75
	descAMQPValue  = 0x77
)

// Frame types.
const (
	frameTypeAMQP = 0x00
	frameTypeSASL = 0x01
)

// maxFrameSize is the largest frame the server accepts. It is advertised to the
// client as max-frame-size in the open performative (see conn.onOpen) and
// enforced in readFrame so a crafted size field cannot trigger a huge body
// allocation.
const maxFrameSize = 1024 * 64

// errFrameTooLarge is returned when a frame header advertises a size beyond
// maxFrameSize, before any body allocation.
var errFrameTooLarge = errors.New("amqp: frame size exceeds max-frame-size")

// Protocol header preambles exchanged before framing begins.
var (
	amqpHeader = []byte{'A', 'M', 'Q', 'P', 0x00, 0x01, 0x00, 0x00}
	saslHeader = []byte{'A', 'M', 'Q', 'P', 0x03, 0x01, 0x00, 0x00}
)

// frame is a decoded AMQP frame: a performative plus any trailing payload
// (used by transfer frames to carry the bare message).
type frame struct {
	typ     byte
	channel uint16
	code    uint64
	fields  []any
	payload []byte
}

// readProtocolHeader reads the 8-byte protocol preamble.
func readProtocolHeader(r io.Reader) ([]byte, error) {
	hdr := make([]byte, 8)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return nil, err
	}
	if string(hdr[:4]) != "AMQP" {
		return nil, fmt.Errorf("amqp: bad protocol header %x", hdr)
	}
	return hdr, nil
}

// readFrame reads one framed performative from r.
func readFrame(r io.Reader) (*frame, error) {
	header := make([]byte, 8)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint32(header[0:4])
	if size < 8 {
		return nil, fmt.Errorf("amqp: invalid frame size %d", size)
	}
	if size > maxFrameSize {
		return nil, fmt.Errorf("%w: %d > %d", errFrameTooLarge, size, maxFrameSize)
	}
	doff := int(header[4]) * 4
	if doff < 8 {
		return nil, fmt.Errorf("amqp: invalid data offset %d", doff)
	}
	typ := header[5]
	channel := binary.BigEndian.Uint16(header[6:8])

	body := make([]byte, int(size)-8)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	// Skip any extended header.
	if doff > 8 {
		if doff-8 > len(body) {
			return nil, errors.New("amqp: data offset past frame body")
		}
		body = body[doff-8:]
	}

	f := &frame{typ: typ, channel: channel}
	if len(body) == 0 {
		// Empty frame (heartbeat).
		return f, nil
	}

	dec := newDecoder(body)
	v, err := dec.readValue()
	if err != nil {
		return nil, err
	}
	desc, ok := v.(described)
	if !ok {
		return nil, fmt.Errorf("amqp: frame body is not a described type: %T", v)
	}
	f.code = descriptorCode(desc.descriptor)
	if list, isList := desc.value.([]any); isList {
		f.fields = list
	}
	f.payload = body[dec.pos:]
	return f, nil
}

// descriptorCode extracts the small-ulong code from a performative descriptor.
func descriptorCode(d any) uint64 {
	switch v := d.(type) {
	case uint64:
		return v
	case uint32:
		return uint64(v)
	case uint8:
		return uint64(v)
	default:
		return 0
	}
}

// writeFrame writes a performative (described list) plus optional payload as a
// single framed message.
func writeFrame(w io.Writer, typ byte, channel uint16, code uint64, fields []any, payload []byte) error {
	body := encodeDescribedList(code, fields)
	size := 8 + len(body) + len(payload)

	header := make([]byte, 8)
	binary.BigEndian.PutUint32(header[0:4], uint32(size))
	header[4] = 2 // DOFF: 8-byte header
	header[5] = typ
	binary.BigEndian.PutUint16(header[6:8], channel)

	if _, err := w.Write(header); err != nil {
		return err
	}
	if _, err := w.Write(body); err != nil {
		return err
	}
	if len(payload) > 0 {
		if _, err := w.Write(payload); err != nil {
			return err
		}
	}
	return nil
}

// field returns the i-th performative field or nil when absent.
func (f *frame) field(i int) any {
	if i < 0 || i >= len(f.fields) {
		return nil
	}
	return f.fields[i]
}
