package sbserver

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

// TestReadFrameRejectsOversizedSize verifies that a frame header advertising a
// size larger than the negotiated max-frame-size is rejected before the body is
// allocated (guards against an unauthenticated remote OOM crash).
func TestReadFrameRejectsOversizedSize(t *testing.T) {
	header := make([]byte, 8)
	binary.BigEndian.PutUint32(header[0:4], maxFrameSize+1)
	header[4] = 2 // DOFF
	header[5] = frameTypeAMQP

	// Provide only the 8-byte header; the body must never be read because the
	// size check must reject the frame first.
	r := bytes.NewReader(header)
	// Must reject with the specific bound error before reading/allocating the
	// body; a buffer-underrun error would mean the guard never fired.
	if _, err := readFrame(r); !errors.Is(err, errFrameTooLarge) {
		t.Fatalf("expected errFrameTooLarge, got %v", err)
	}
}

// TestReadFrameRoundTrip ensures a legitimately sized frame still decodes after
// the bound was added.
func TestReadFrameRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	fields := []any{"localaz", nil, uint32(1024 * 64)}
	if err := writeFrame(&buf, frameTypeAMQP, 0, descOpen, fields, nil); err != nil {
		t.Fatalf("writeFrame: %v", err)
	}

	f, err := readFrame(&buf)
	if err != nil {
		t.Fatalf("readFrame: %v", err)
	}
	if f.code != descOpen {
		t.Errorf("code = %#x, want %#x", f.code, descOpen)
	}
	if got := f.field(0); got != "localaz" {
		t.Errorf("field(0) = %v, want localaz", got)
	}
}
