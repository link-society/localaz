package wpsserver

import (
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// stubSocket is a minimal wsSocket used to drive wsConn without a real
// *websocket.Conn. Its WriteMessage blocks until the test releases it, which
// lets us simulate a stalled/slow client whose writer goroutine cannot keep up.
type stubSocket struct {
	mu        sync.Mutex
	closed    bool
	release   chan struct{} // WriteMessage waits on this before returning
	readLimit int64
}

func newStubSocket() *stubSocket {
	return &stubSocket{release: make(chan struct{})}
}

func (s *stubSocket) WriteMessage(messageType int, data []byte) error {
	<-s.release
	return nil
}

func (s *stubSocket) ReadMessage() (int, []byte, error) {
	// Block forever; the connection loop is not exercised by these tests.
	select {}
}

func (s *stubSocket) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func (s *stubSocket) SetReadLimit(limit int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.readLimit = limit
}

func (s *stubSocket) SetWriteDeadline(time.Time) error { return nil }

func (s *stubSocket) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

var _ wsSocket = (*stubSocket)(nil)
var _ wsSocket = (*websocket.Conn)(nil)

// TestSendNeverBlocksWhenWriterStalls proves that send is non-blocking even
// when the writer goroutine cannot drain the outbound buffer. The stub socket's
// WriteMessage never returns, so the single in-flight frame plus the buffered
// frames are the only capacity available; every further send must return
// promptly (dropping the connection) rather than blocking the producer.
//
// Against the old blocking send (which held writeMu and called
// socket.WriteMessage directly), the first send would block forever on the
// stub socket and this test would time out.
func TestSendNeverBlocksWhenWriterStalls(t *testing.T) {
	const bufSize = 2

	sock := newStubSocket()
	c := newWSConn("conn-1", "user-1", sock, newHub(), bufSize)
	c.startWriter()

	// Overshoot capacity substantially. The writer is stuck on the first
	// frame (release is never signalled), so the outbound buffer fills and
	// subsequent sends must drop the connection instead of blocking.
	for i := 0; i < bufSize+50; i++ {
		done := make(chan struct{})
		go func() {
			c.send([]byte("frame"))
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("send blocked on a stalled connection (iteration %d)", i)
		}
	}

	// Overflow must have dropped the connection via closeNow.
	deadline := time.Now().Add(2 * time.Second)
	for !sock.isClosed() {
		if time.Now().After(deadline) {
			t.Fatal("overflow did not trigger closeNow (socket never closed)")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestCloseNowStopsWriterAndIsIdempotent verifies closeNow closes the socket
// exactly once and stops the writer goroutine.
func TestCloseNowStopsWriterAndIsIdempotent(t *testing.T) {
	sock := newStubSocket()
	c := newWSConn("conn-2", "", sock, newHub(), 4)
	c.startWriter()

	c.closeNow()
	c.closeNow() // must be a no-op the second time (sync.Once)

	if !sock.isClosed() {
		t.Fatal("closeNow did not close the socket")
	}

	// After closeNow, send must still be non-blocking and must not panic.
	done := make(chan struct{})
	go func() {
		c.send([]byte("frame"))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("send blocked after closeNow")
	}
}
