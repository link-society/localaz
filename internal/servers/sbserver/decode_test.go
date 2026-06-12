package sbserver

import (
	"errors"
	"testing"
)

// TestDecodeCollectionSizeBounds verifies that a crafted list/map header whose
// element count vastly exceeds the buffer is rejected with an error rather than
// triggering a multi-GB allocation (unauthenticated remote crash).
func TestDecodeCollectionSizeBounds(t *testing.T) {
	tests := []struct {
		name string
		buf  []byte
	}{
		{
			// list32: type code, 4-byte size, 4-byte count = 0x7FFFFFFF.
			name: "list32 huge count",
			buf:  []byte{typeList32, 0xff, 0xff, 0xff, 0xff, 0x7f, 0xff, 0xff, 0xff},
		},
		{
			// map32: type code, 4-byte size, 4-byte count = 0x7FFFFFFF.
			name: "map32 huge count",
			buf:  []byte{typeMap32, 0xff, 0xff, 0xff, 0xff, 0x7f, 0xff, 0xff, 0xff},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := newDecoder(tt.buf)
			// Must reject with the specific bound error BEFORE allocating; a
			// generic buffer-underrun error would mean the guard never fired.
			if _, err := dec.readValue(); !errors.Is(err, errCollectionTooLarge) {
				t.Fatalf("expected errCollectionTooLarge, got %v", err)
			}
		})
	}
}

// TestDecodeCollectionRoundTrip ensures the bound does not break legitimate
// frames: encode a list and a map, decode them back, and assert equality.
func TestDecodeCollectionRoundTrip(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		items := []any{uint32(1), "hello", true}
		enc := newEncoder()
		enc.writeValue(items)

		dec := newDecoder(enc.bytes())
		got, err := dec.readValue()
		if err != nil {
			t.Fatalf("decode list: %v", err)
		}
		list, ok := got.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", got)
		}
		if len(list) != len(items) {
			t.Fatalf("length = %d, want %d", len(list), len(items))
		}
		for i := range items {
			if list[i] != items[i] {
				t.Errorf("item %d = %v, want %v", i, list[i], items[i])
			}
		}
	})

	t.Run("map", func(t *testing.T) {
		m := map[any]any{"k": uint32(7)}
		enc := newEncoder()
		enc.writeValue(m)

		dec := newDecoder(enc.bytes())
		got, err := dec.readValue()
		if err != nil {
			t.Fatalf("decode map: %v", err)
		}
		decoded, ok := got.(map[any]any)
		if !ok {
			t.Fatalf("expected map[any]any, got %T", got)
		}
		if decoded["k"] != uint32(7) {
			t.Errorf("map[k] = %v, want %v", decoded["k"], uint32(7))
		}
	})
}
