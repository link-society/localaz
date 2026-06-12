package sbserver

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

// This file implements the subset of the AMQP 1.0 type system needed to talk to
// the official go-amqp client used by azservicebus: primitive types, strings,
// symbols, binary, lists, maps and described types. Message bodies are never
// decoded; they are relayed verbatim, so only the performative/CBS surface is
// modelled here.

// decoder reads AMQP-encoded values from a byte slice.
type decoder struct {
	buf []byte
	pos int
}

func newDecoder(buf []byte) *decoder { return &decoder{buf: buf} }

func (d *decoder) remaining() int { return len(d.buf) - d.pos }

func (d *decoder) readByte() (byte, error) {
	if d.pos >= len(d.buf) {
		return 0, errors.New("amqp: unexpected end of buffer")
	}
	b := d.buf[d.pos]
	d.pos++
	return b, nil
}

func (d *decoder) readN(n int) ([]byte, error) {
	if d.pos+n > len(d.buf) {
		return nil, errors.New("amqp: unexpected end of buffer")
	}
	b := d.buf[d.pos : d.pos+n]
	d.pos += n
	return b, nil
}

// readValue decodes a single AMQP value into a Go representation.
func (d *decoder) readValue() (any, error) {
	code, err := d.readByte()
	if err != nil {
		return nil, err
	}
	return d.readBody(code)
}

func (d *decoder) readBody(code byte) (any, error) {
	switch code {
	case typeNull:
		return nil, nil
	case typeBoolTrue:
		return true, nil
	case typeBoolFalse:
		return false, nil
	case typeBool:
		b, err := d.readByte()
		return b != 0, err
	case typeUint0:
		return uint32(0), nil
	case typeUlong0:
		return uint64(0), nil
	case typeSmallUint:
		b, err := d.readByte()
		return uint32(b), err
	case typeSmallUlong:
		b, err := d.readByte()
		return uint64(b), err
	case typeSmallInt:
		b, err := d.readByte()
		return int32(int8(b)), err
	case typeSmallLong:
		b, err := d.readByte()
		return int64(int8(b)), err
	case typeUbyte:
		b, err := d.readByte()
		return b, err
	case typeByte:
		b, err := d.readByte()
		return int8(b), err
	case typeUshort:
		raw, err := d.readN(2)
		if err != nil {
			return nil, err
		}
		return binary.BigEndian.Uint16(raw), nil
	case typeShort:
		raw, err := d.readN(2)
		if err != nil {
			return nil, err
		}
		return int16(binary.BigEndian.Uint16(raw)), nil
	case typeUint:
		raw, err := d.readN(4)
		if err != nil {
			return nil, err
		}
		return binary.BigEndian.Uint32(raw), nil
	case typeInt:
		raw, err := d.readN(4)
		if err != nil {
			return nil, err
		}
		return int32(binary.BigEndian.Uint32(raw)), nil
	case typeUlong:
		raw, err := d.readN(8)
		if err != nil {
			return nil, err
		}
		return binary.BigEndian.Uint64(raw), nil
	case typeLong:
		raw, err := d.readN(8)
		if err != nil {
			return nil, err
		}
		return int64(binary.BigEndian.Uint64(raw)), nil
	case typeTimestamp:
		raw, err := d.readN(8)
		if err != nil {
			return nil, err
		}
		ms := int64(binary.BigEndian.Uint64(raw))
		return time.UnixMilli(ms).UTC(), nil
	case typeUUID:
		raw, err := d.readN(16)
		if err != nil {
			return nil, err
		}
		var u [16]byte
		copy(u[:], raw)
		return u, nil
	case typeVbin8:
		return d.readBinary(1)
	case typeVbin32:
		return d.readBinary(4)
	case typeStr8:
		return d.readString(1, false)
	case typeStr32:
		return d.readString(4, false)
	case typeSym8:
		return d.readString(1, true)
	case typeSym32:
		return d.readString(4, true)
	case typeList0:
		return []any{}, nil
	case typeList8:
		return d.readList(1)
	case typeList32:
		return d.readList(4)
	case typeMap8:
		return d.readMap(1)
	case typeMap32:
		return d.readMap(4)
	case typeDescribed:
		desc, err := d.readValue()
		if err != nil {
			return nil, err
		}
		val, err := d.readValue()
		if err != nil {
			return nil, err
		}
		return described{descriptor: desc, value: val}, nil
	case typeFloat:
		raw, err := d.readN(4)
		if err != nil {
			return nil, err
		}
		return binary.BigEndian.Uint32(raw), nil
	case typeDouble:
		raw, err := d.readN(8)
		if err != nil {
			return nil, err
		}
		return binary.BigEndian.Uint64(raw), nil
	default:
		return nil, fmt.Errorf("amqp: unsupported type code 0x%02x", code)
	}
}

func (d *decoder) readSize(width int) (int, error) {
	switch width {
	case 1:
		b, err := d.readByte()
		return int(b), err
	default:
		raw, err := d.readN(4)
		if err != nil {
			return 0, err
		}
		return int(binary.BigEndian.Uint32(raw)), nil
	}
}

func (d *decoder) readBinary(width int) ([]byte, error) {
	size, err := d.readSize(width)
	if err != nil {
		return nil, err
	}
	raw, err := d.readN(size)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(raw))
	copy(out, raw)
	return out, nil
}

func (d *decoder) readString(width int, sym bool) (any, error) {
	size, err := d.readSize(width)
	if err != nil {
		return nil, err
	}
	raw, err := d.readN(size)
	if err != nil {
		return nil, err
	}
	if sym {
		return symbol(raw), nil
	}
	return string(raw), nil
}

func (d *decoder) readList(width int) ([]any, error) {
	if _, err := d.readSize(width); err != nil {
		return nil, err
	}
	count, err := d.readSize(width)
	if err != nil {
		return nil, err
	}
	items := make([]any, 0, count)
	for i := 0; i < count; i++ {
		v, err := d.readValue()
		if err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, nil
}

func (d *decoder) readMap(width int) (map[any]any, error) {
	if _, err := d.readSize(width); err != nil {
		return nil, err
	}
	count, err := d.readSize(width)
	if err != nil {
		return nil, err
	}
	m := make(map[any]any, count/2)
	for i := 0; i < count; i += 2 {
		k, err := d.readValue()
		if err != nil {
			return nil, err
		}
		v, err := d.readValue()
		if err != nil {
			return nil, err
		}
		m[normalizeKey(k)] = v
	}
	return m, nil
}

// normalizeKey makes symbol and string keys comparable as plain strings while
// leaving other key types untouched.
func normalizeKey(k any) any {
	switch v := k.(type) {
	case symbol:
		return string(v)
	default:
		return v
	}
}
