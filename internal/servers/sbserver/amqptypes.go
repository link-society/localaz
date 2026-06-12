package sbserver

// This file models the shared AMQP 1.0 type-system surface used by both the
// encoder and decoder: the symbol/described value types and the constructor
// codes for the primitive and compound types localaz needs to speak to the
// official go-amqp client. Message bodies are relayed verbatim, so only the
// performative/CBS surface is modelled.

// symbol is an AMQP symbol (ASCII), distinct from a UTF-8 string so it can be
// re-encoded with the correct type code.
type symbol string

// described is an AMQP described type: a descriptor (usually a small ulong code
// or a symbol) paired with a value.
type described struct {
	descriptor any
	value      any
}

// AMQP type constructor codes used by the encoder/decoder.
const (
	typeNull       = 0x40
	typeBoolTrue   = 0x41
	typeBoolFalse  = 0x42
	typeBool       = 0x56
	typeUbyte      = 0x50
	typeUshort     = 0x60
	typeUint       = 0x70
	typeSmallUint  = 0x52
	typeUint0      = 0x43
	typeUlong      = 0x80
	typeSmallUlong = 0x53
	typeUlong0     = 0x44
	typeByte       = 0x51
	typeShort      = 0x61
	typeInt        = 0x71
	typeSmallInt   = 0x54
	typeLong       = 0x81
	typeSmallLong  = 0x55
	typeFloat      = 0x72
	typeDouble     = 0x82
	typeTimestamp  = 0x83
	typeUUID       = 0x98
	typeVbin8      = 0xa0
	typeVbin32     = 0xb0
	typeStr8       = 0xa1
	typeStr32      = 0xb1
	typeSym8       = 0xa3
	typeSym32      = 0xb3
	typeList0      = 0x45
	typeList8      = 0xc0
	typeList32     = 0xd0
	typeMap8       = 0xc1
	typeMap32      = 0xd1
	typeDescribed  = 0x00
)
