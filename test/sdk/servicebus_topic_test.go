package sdk

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

func TestServiceBusTopicSubscription(t *testing.T) {
	connStr := startServiceBus(t)

	client, err := azservicebus.NewClientFromConnectionString(connStr, nil)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close(context.Background())

	const (
		topic = "mytopic"
		sub   = "mysub"
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Register the subscription by briefly attaching a receiver, then close it.
	// The broker keeps the subscription so a later send fans out to it; real
	// Service Bus likewise requires a subscription to exist before publishing.
	warmReceiver, err := client.NewReceiverForSubscription(topic, sub, nil)
	if err != nil {
		t.Fatalf("new warm receiver: %v", err)
	}
	warmCtx, warmCancel := context.WithTimeout(ctx, 2*time.Second)
	_, _ = warmReceiver.ReceiveMessages(warmCtx, 1, nil)
	warmCancel()
	warmReceiver.Close(ctx)

	sender, err := client.NewSender(topic, nil)
	if err != nil {
		t.Fatalf("new sender: %v", err)
	}
	if err := sender.SendMessage(ctx, &azservicebus.Message{Body: []byte("topic payload")}, nil); err != nil {
		t.Fatalf("send: %v", err)
	}
	sender.Close(ctx)

	receiver, err := client.NewReceiverForSubscription(topic, sub, nil)
	if err != nil {
		t.Fatalf("new receiver: %v", err)
	}
	defer receiver.Close(ctx)

	msgs, err := receiver.ReceiveMessages(ctx, 1, nil)
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if got := string(msgs[0].Body); got != "topic payload" {
		t.Fatalf("body = %q, want %q", got, "topic payload")
	}
	if err := receiver.CompleteMessage(ctx, msgs[0], nil); err != nil {
		t.Fatalf("complete: %v", err)
	}
}
