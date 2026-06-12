package wpsserver

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// outboundBuffer bounds the per-connection send queue. A client that
	// cannot keep up fills this buffer and is then dropped, so one slow
	// consumer never stalls a broadcast or holds the hub lock.
	outboundBuffer = 64
	// writeWait is the deadline applied to each socket write.
	writeWait = 10 * time.Second
	// readLimit caps the size of an inbound frame to bound memory use.
	readLimit = 1 << 20
)

// wsSocket is the subset of *websocket.Conn that wsConn depends on. It exists
// so the socket can be stubbed in tests without a live network connection.
type wsSocket interface {
	WriteMessage(messageType int, data []byte) error
	ReadMessage() (messageType int, p []byte, err error)
	Close() error
	SetReadLimit(limit int64)
	SetWriteDeadline(t time.Time) error
}

// wsConn adapts a gorilla WebSocket to the conn interface and runs the
// json.webpubsub.azure.v1 protocol loop. Outbound frames go through a buffered
// channel drained by a dedicated writer goroutine, so send is non-blocking and
// a stalled client is dropped rather than stalling the hub.
type wsConn struct {
	connID string
	user   string
	socket wsSocket
	hub    *hub

	outbound chan []byte
	quit     chan struct{}
	once     sync.Once
}

// newWSConn builds a wsConn with a buffered outbound queue of the given size.
func newWSConn(connID, user string, socket wsSocket, h *hub, bufSize int) *wsConn {
	return &wsConn{
		connID:   connID,
		user:     user,
		socket:   socket,
		hub:      h,
		outbound: make(chan []byte, bufSize),
		quit:     make(chan struct{}),
	}
}

func (c *wsConn) id() string     { return c.connID }
func (c *wsConn) userID() string { return c.user }

// send enqueues a frame for the writer goroutine without ever blocking the
// caller. If the outbound buffer is full the consumer is too slow, so the
// connection is dropped instead of stalling the hub (and the hub lock).
func (c *wsConn) send(frame []byte) {
	select {
	case <-c.quit:
		// Already closing; drop silently.
	case c.outbound <- frame:
	default:
		// Buffer full: slow consumer. Drop the connection.
		c.closeNow()
	}
}

// startWriter launches the goroutine that drains the outbound queue and writes
// to the socket, applying a per-message write deadline.
func (c *wsConn) startWriter() {
	go func() {
		for {
			select {
			case <-c.quit:
				return
			case frame := <-c.outbound:
				_ = c.socket.SetWriteDeadline(time.Now().Add(writeWait))
				if err := c.socket.WriteMessage(websocket.TextMessage, frame); err != nil {
					c.closeNow()
					return
				}
			}
		}
	}()
}

func (c *wsConn) closeNow() {
	c.once.Do(func() {
		close(c.quit)
		_ = c.socket.Close()
	})
}

// run sends the connected frame, joins any groups carried in the token, then
// reads client messages until the socket closes.
func (c *wsConn) run(initialGroups []string) {
	c.socket.SetReadLimit(readLimit)
	c.startWriter()

	c.hub.add(c)
	defer c.hub.remove(c.connID)
	defer c.closeNow()

	c.send(mustJSON(systemMessage{
		Type:         "system",
		Event:        "connected",
		ConnectionID: c.connID,
		UserID:       c.user,
	}))

	for _, g := range initialGroups {
		if g != "" {
			c.hub.join(g, c.connID)
		}
	}

	for {
		_, raw, err := c.socket.ReadMessage()
		if err != nil {
			return
		}
		c.handle(raw)
	}
}

func (c *wsConn) handle(raw []byte) {
	var msg clientMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return
	}

	switch msg.Type {
	case "joinGroup":
		c.hub.join(msg.Group, c.connID)
		c.ack(msg.AckID, true, nil)
	case "leaveGroup":
		c.hub.leave(msg.Group, c.connID)
		c.ack(msg.AckID, true, nil)
	case "sendToGroup":
		dataType := msg.DataType
		if dataType == "" {
			dataType = "json"
		}
		c.hub.sendGroup(msg.Group, mustJSON(dataMessage{
			Type:     "message",
			From:     "group",
			Group:    msg.Group,
			DataType: dataType,
			Data:     msg.Data,
		}))
		c.ack(msg.AckID, true, nil)
	case "event":
		// Custom events are accepted and acknowledged but not routed
		// anywhere in the emulator.
		c.ack(msg.AckID, true, nil)
	default:
		c.ack(msg.AckID, false, &ackErr{Name: "InvalidMessage", Message: "unsupported message type"})
	}
}

func (c *wsConn) ack(ackID *uint64, success bool, e *ackErr) {
	if ackID == nil {
		return
	}
	c.send(mustJSON(ackMessage{
		Type:    "ack",
		AckID:   *ackID,
		Success: success,
		Error:   e,
	}))
}

func newConnID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return base64.RawURLEncoding.EncodeToString(b[:])
}

// tokenClaims is the subset of JWT claims the emulator reads from a client
// access token to seed the connection's identity and group membership.
type tokenClaims struct {
	Subject string   `json:"sub"`
	Groups  []string `json:"webpubsub.group"`
}

// parseToken extracts claims from a JWT without verifying its signature. The
// emulator does not validate credentials, so only the payload is decoded.
func parseToken(token string) tokenClaims {
	var claims tokenClaims
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return claims
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims
	}
	_ = json.Unmarshal(payload, &claims)
	return claims
}
