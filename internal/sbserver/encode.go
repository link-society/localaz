package sbserver

import (
	"encoding/binary"
	"fmt"
	"time"
)

// encoder builds AMQP-encoded values.
type encoder struct {
	buf []byte
}

func newEncoder() *encoder { return &encoder{} }

func (e *encoder) bytes() []byte { return e.buf }

func (e *encoder) writeByte(b byte) { e.buf = append(e.buf, b) }

func (e *encoder) write(b []byte) { e.buf = append(e.buf, b...) }

// writeValue encodes a single Go value as AMQP.
func (e *encoder) writeValue(v any) {
	switch val := v.(type) {
	case nil:
		e.writeByte(typeNull)
	case bool:
		if val {
			e.writeByte(typeBoolTrue)
		} else {
			e.writeByte(typeBoolFalse)
		}
	case uint8:
		e.writeByte(typeUbyte)
		e.writeByte(val)
	case uint16:
		e.writeByte(typeUshort)
		e.buf = binary.BigEndian.AppendUint16(e.buf, val)
	case uint32:
		e.writeByte(typeUint)
		e.buf = binary.BigEndian.AppendUint32(e.buf, val)
	case uint64:
		e.writeByte(typeUlong)
		e.buf = binary.BigEndian.AppendUint64(e.buf, val)
	case int32:
		e.writeByte(typeInt)
		e.buf = binary.BigEndian.AppendUint32(e.buf, uint32(val))
	case int64:
		e.writeByte(typeLong)
		e.buf = binary.BigEndian.AppendUint64(e.buf, uint64(val))
	case string:
		e.writeString(val)
	case symbol:
		e.writeSymbol(val)
	case []byte:
		e.writeBinary(val)
	case time.Time:
		e.writeByte(typeTimestamp)
		e.buf = binary.BigEndian.AppendUint64(e.buf, uint64(val.UnixMilli()))
	case [16]byte:
		e.writeByte(typeUUID)
		e.write(val[:])
	case described:
		e.writeByte(typeDescribed)
		e.writeValue(val.descriptor)
		e.writeValue(val.value)
	case []any:
		e.writeList(val)
	case map[any]any:
		e.writeMap(val)
	default:
		panic(fmt.Sprintf("amqp: cannot encode %T", v))
	}
}

func (e *encoder) writeString(s string) {
	if len(s) <= 255 {
		e.writeByte(typeStr8)
		e.writeByte(byte(len(s)))
	} else {
		e.writeByte(typeStr32)
		e.buf = binary.BigEndian.AppendUint32(e.buf, uint32(len(s)))
	}
	e.write([]byte(s))
}

func (e *encoder) writeSymbol(s symbol) {
	if len(s) <= 255 {
		e.writeByte(typeSym8)
		e.writeByte(byte(len(s)))
	} else {
		e.writeByte(typeSym32)
		e.buf = binary.BigEndian.AppendUint32(e.buf, uint32(len(s)))
	}
	e.write([]byte(s))
}

func (e *encoder) writeBinary(b []byte) {
	if len(b) <= 255 {
		e.writeByte(typeVbin8)
		e.writeByte(byte(len(b)))
	} else {
		e.writeByte(typeVbin32)
		e.buf = binary.BigEndian.AppendUint32(e.buf, uint32(len(b)))
	}
	e.write(b)
}

func (e *encoder) writeList(items []any) {
	if len(items) == 0 {
		e.writeByte(typeList0)
		return
	}
	// Encode items into a temporary buffer to compute the size prefix.
	inner := newEncoder()
	for _, item := range items {
		inner.writeValue(item)
	}
	body := inner.bytes()

	if len(items) <= 255 && len(body)+1 <= 255 {
		e.writeByte(typeList8)
		e.writeByte(byte(len(body) + 1))
		e.writeByte(byte(len(items)))
	} else {
		e.writeByte(typeList32)
		e.buf = binary.BigEndian.AppendUint32(e.buf, uint32(len(body)+4))
		e.buf = binary.BigEndian.AppendUint32(e.buf, uint32(len(items)))
	}
	e.write(body)
}

func (e *encoder) writeMap(m map[any]any) {
	inner := newEncoder()
	count := 0
	for k, v := range m {
		inner.writeValue(k)
		inner.writeValue(v)
		count += 2
	}
	body := inner.bytes()

	if count <= 255 && len(body)+1 <= 255 {
		e.writeByte(typeMap8)
		e.writeByte(byte(len(body) + 1))
		e.writeByte(byte(count))
	} else {
		e.writeByte(typeMap32)
		e.buf = binary.BigEndian.AppendUint32(e.buf, uint32(len(body)+4))
		e.buf = binary.BigEndian.AppendUint32(e.buf, uint32(count))
	}
	e.write(body)
}

// encodeDescribedList encodes a performative or message section: a described
// type whose descriptor is a small ulong code and whose body is a list of
// fields. Trailing nil fields are trimmed, matching AMQP encoding conventions.
func encodeDescribedList(code uint64, fields []any) []byte {
	trimmed := fields
	for len(trimmed) > 0 && trimmed[len(trimmed)-1] == nil {
		trimmed = trimmed[:len(trimmed)-1]
	}

	e := newEncoder()
	e.writeByte(typeDescribed)
	e.writeByte(typeSmallUlong)
	e.writeByte(byte(code))
	e.writeList(trimmed)
	return e.bytes()
}
