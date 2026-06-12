package sdk

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/messaging"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/eventgrid/aznamespaces"

	"localaz.dev/internal/egserver"
	"localaz.dev/internal/egstore"
)

// newEventGrid spins up an in-process Event Grid emulator and returns sender and
// receiver clients pointed at the given topic / subscription.
func newEventGrid(t *testing.T, topic, subscription string) (*aznamespaces.SenderClient, *aznamespaces.ReceiverClient) {
	t.Helper()
	// The Event Grid SDK refuses to send shared-key credentials over plain HTTP,
	// so the emulator is exposed over TLS and the SDK is pointed at the test
	// server's trusting client.
	ts := httptest.NewTLSServer(egserver.New(egstore.New()))
	t.Cleanup(ts.Close)

	cred := azcore.NewKeyCredential("localaz-dev-key")

	sendOpts := &aznamespaces.SenderClientOptions{}
	sendOpts.Transport = ts.Client()
	sender, err := aznamespaces.NewSenderClientWithSharedKeyCredential(ts.URL, topic, cred, sendOpts)
	if err != nil {
		t.Fatalf("create sender: %v", err)
	}

	recvOpts := &aznamespaces.ReceiverClientOptions{}
	recvOpts.Transport = ts.Client()
	receiver, err := aznamespaces.NewReceiverClientWithSharedKeyCredential(ts.URL, topic, subscription, cred, recvOpts)
	if err != nil {
		t.Fatalf("create receiver: %v", err)
	}
	return sender, receiver
}

func TestEventGridPublishReceiveAcknowledge(t *testing.T) {
	sender, receiver := newEventGrid(t, "orders", "fulfillment")
	c := ctx(t)

	payload := map[string]string{"orderId": "A-1001", "status": "placed"}
	event, err := messaging.NewCloudEvent("/orders/api", "Order.Placed", payload, nil)
	if err != nil {
		t.Fatalf("build cloud event: %v", err)
	}
	if _, err := sender.SendEvent(c, &event, nil); err != nil {
		t.Fatalf("send event: %v", err)
	}

	resp, err := receiver.ReceiveEvents(c, &aznamespaces.ReceiveEventsOptions{MaxEvents: to.Ptr[int32](5)})
	if err != nil {
		t.Fatalf("receive events: %v", err)
	}
	if len(resp.Details) != 1 {
		t.Fatalf("expected 1 event, got %d", len(resp.Details))
	}

	got := resp.Details[0]
	if got.Event.Type != "Order.Placed" {
		t.Errorf("event type = %q, want %q", got.Event.Type, "Order.Placed")
	}
	if got.Event.Source != "/orders/api" {
		t.Errorf("event source = %q, want %q", got.Event.Source, "/orders/api")
	}
	gotData, ok := got.Event.Data.([]byte)
	if !ok {
		t.Fatalf("event data type = %T, want []byte", got.Event.Data)
	}
	var gotPayload map[string]string
	if err := json.Unmarshal(gotData, &gotPayload); err != nil {
		t.Fatalf("unmarshal event data: %v", err)
	}
	if gotPayload["orderId"] != payload["orderId"] || gotPayload["status"] != payload["status"] {
		t.Errorf("event data = %v, want %v", gotPayload, payload)
	}
	if got.BrokerProperties.LockToken == nil {
		t.Fatal("expected a lock token")
	}

	ack, err := receiver.AcknowledgeEvents(c, []string{*got.BrokerProperties.LockToken}, nil)
	if err != nil {
		t.Fatalf("acknowledge: %v", err)
	}
	if len(ack.SucceededLockTokens) != 1 {
		t.Fatalf("expected 1 acknowledged token, got %d", len(ack.SucceededLockTokens))
	}

	// After acknowledgement the event must not be redelivered.
	empty, err := receiver.ReceiveEvents(c, &aznamespaces.ReceiveEventsOptions{MaxEvents: to.Ptr[int32](5)})
	if err != nil {
		t.Fatalf("second receive: %v", err)
	}
	if len(empty.Details) != 0 {
		t.Fatalf("expected no events after ack, got %d", len(empty.Details))
	}
}

func TestEventGridReleaseRedelivers(t *testing.T) {
	sender, receiver := newEventGrid(t, "telemetry", "audit")
	c := ctx(t)

	event, err := messaging.NewCloudEvent("/devices/sensor-7", "Telemetry.Reading", map[string]int{"temp": 21}, nil)
	if err != nil {
		t.Fatalf("build cloud event: %v", err)
	}
	if _, err := sender.SendEvent(c, &event, nil); err != nil {
		t.Fatalf("send event: %v", err)
	}

	first, err := receiver.ReceiveEvents(c, &aznamespaces.ReceiveEventsOptions{MaxEvents: to.Ptr[int32](1)})
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	if len(first.Details) != 1 {
		t.Fatalf("expected 1 event, got %d", len(first.Details))
	}
	if dc := first.Details[0].BrokerProperties.DeliveryCount; dc == nil || *dc != 1 {
		t.Fatalf("delivery count = %v, want 1", dc)
	}

	if _, err := receiver.ReleaseEvents(c, []string{*first.Details[0].BrokerProperties.LockToken}, nil); err != nil {
		t.Fatalf("release: %v", err)
	}

	second, err := receiver.ReceiveEvents(c, &aznamespaces.ReceiveEventsOptions{MaxEvents: to.Ptr[int32](1)})
	if err != nil {
		t.Fatalf("re-receive: %v", err)
	}
	if len(second.Details) != 1 {
		t.Fatalf("expected redelivery, got %d events", len(second.Details))
	}
	if dc := second.Details[0].BrokerProperties.DeliveryCount; dc == nil || *dc != 2 {
		t.Fatalf("redelivered delivery count = %v, want 2", dc)
	}
}
