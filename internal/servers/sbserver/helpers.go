package sbserver

import "errors"

// errClose signals that the peer sent a close performative.
var errClose = errors.New("amqp: connection closed")

const cbsAddress = "$cbs"

func asUint32(v any) uint32 {
	switch n := v.(type) {
	case uint32:
		return n
	case uint64:
		return uint32(n)
	case uint16:
		return uint32(n)
	case uint8:
		return uint32(n)
	case int32:
		return uint32(n)
	case int64:
		return uint32(n)
	default:
		return 0
	}
}

func asBool(v any) bool {
	b, _ := v.(bool)
	return b
}

// addressOf extracts the address field from a described source or target.
func addressOf(node any) string {
	desc, ok := node.(described)
	if !ok {
		return ""
	}
	fields, ok := desc.value.([]any)
	if !ok || len(fields) == 0 {
		return ""
	}
	addr, _ := fields[0].(string)
	return addr
}
