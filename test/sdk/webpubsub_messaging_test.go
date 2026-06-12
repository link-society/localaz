package sdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/streaming"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azwebpubsub"
	"github.com/gorilla/websocket"

	"localaz.dev/internal/servers/wpsserver"
)

// TestWebPubSubPublishToGroup exercises the full Web PubSub pub/sub path: a
// WebSocket client joins a group, the official azwebpubsub service client
// publishes to that group over REST, and the client receives the message.
func TestWebPubSubPublishToGroup(t *testing.T) {
	ts := httptest.NewServer(wpsserver.New())
	defer ts.Close()

	const (
		hub   = "chat"
		group = "room1"
	)

	connStr := fmt.Sprintf("Endpoint=%s;AccessKey=localaz-test-key;", ts.URL)
	client, err := azwebpubsub.NewClientFromConnectionString(connStr, nil)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	access, err := client.GenerateClientAccessURL(ctx(t), hub, &azwebpubsub.GenerateClientAccessURLOptions{
		Roles:  []string{"webpubsub.joinLeaveGroup", "webpubsub.sendToGroup"},
		Groups: []string{group},
	})
	if err != nil {
		t.Fatalf("generate access url: %v", err)
	}

	dialer := websocket.Dialer{Subprotocols: []string{wpsSubprotocol}}
	wsClient, _, err := dialer.Dial(access.URL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer wsClient.Close()
	_ = wsClient.SetReadDeadline(time.Now().Add(5 * time.Second))

	// First frame is the system "connected" notification.
	connected := readFrame(t, wsClient)
	if connected["type"] != "system" || connected["event"] != "connected" {
		t.Fatalf("first frame = %v, want system/connected", connected)
	}

	// Explicitly join the group with an ack so publishing is race-free.
	if err := wsClient.WriteJSON(map[string]any{"type": "joinGroup", "group": group, "ackId": 1}); err != nil {
		t.Fatalf("send joinGroup: %v", err)
	}
	if ack := waitForType(t, wsClient, "ack"); ack["success"] != true {
		t.Fatalf("joinGroup ack = %v, want success", ack)
	}

	payload := map[string]string{"text": "hello room"}
	body, _ := json.Marshal(payload)
	if _, err := client.SendToGroup(ctx(t), hub, group, azwebpubsub.ContentTypeApplicationJSON,
		streaming.NopCloser(bytes.NewReader(body)), nil); err != nil {
		t.Fatalf("send to group: %v", err)
	}

	msg := waitForType(t, wsClient, "message")
	if msg["from"] != "group" || msg["group"] != group {
		t.Errorf("message envelope = %v, want from=group group=%s", msg, group)
	}
	data, ok := msg["data"].(map[string]any)
	if !ok {
		t.Fatalf("message data = %T, want object", msg["data"])
	}
	if data["text"] != payload["text"] {
		t.Errorf("message data = %v, want %v", data, payload)
	}
}

// TestWebPubSubBroadcast verifies SendToAll reaches a connected client.
func TestWebPubSubBroadcast(t *testing.T) {
	ts := httptest.NewServer(wpsserver.New())
	defer ts.Close()

	const hub = "chat"

	connStr := fmt.Sprintf("Endpoint=%s;AccessKey=localaz-test-key;", ts.URL)
	client, err := azwebpubsub.NewClientFromConnectionString(connStr, nil)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	access, err := client.GenerateClientAccessURL(ctx(t), hub, nil)
	if err != nil {
		t.Fatalf("generate access url: %v", err)
	}

	dialer := websocket.Dialer{Subprotocols: []string{wpsSubprotocol}}
	wsClient, _, err := dialer.Dial(access.URL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer wsClient.Close()
	_ = wsClient.SetReadDeadline(time.Now().Add(5 * time.Second))

	if connected := readFrame(t, wsClient); connected["event"] != "connected" {
		t.Fatalf("first frame = %v, want connected", connected)
	}

	payload := map[string]string{"text": "broadcast"}
	body, _ := json.Marshal(payload)
	if _, err := client.SendToAll(ctx(t), hub, azwebpubsub.ContentTypeApplicationJSON,
		streaming.NopCloser(bytes.NewReader(body)), nil); err != nil {
		t.Fatalf("send to all: %v", err)
	}

	msg := waitForType(t, wsClient, "message")
	if msg["from"] != "server" {
		t.Errorf("message from = %v, want server", msg["from"])
	}
	data, ok := msg["data"].(map[string]any)
	if !ok {
		t.Fatalf("message data = %T, want object", msg["data"])
	}
	if data["text"] != payload["text"] {
		t.Errorf("message data = %v, want %v", data, payload)
	}
}
