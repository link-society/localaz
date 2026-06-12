package wpsserver

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

// wsConn adapts a gorilla WebSocket to the conn interface and runs the
// json.webpubsub.azure.v1 protocol loop.
type wsConn struct {
	connID string
	user   string
	socket *websocket.Conn
	hub    *hub
	logger *slog.Logger

	writeMu sync.Mutex
	once    sync.Once
}

func (c *wsConn) id() string     { return c.connID }
func (c *wsConn) userID() string { return c.user }

func (c *wsConn) send(frame []byte) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.socket.WriteMessage(websocket.TextMessage, frame)
}

func (c *wsConn) closeNow() {
	c.once.Do(func() {
		_ = c.socket.Close()
	})
}

// run sends the connected frame, joins any groups carried in the token, then
// reads client messages until the socket closes.
func (c *wsConn) run(initialGroups []string) {
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
