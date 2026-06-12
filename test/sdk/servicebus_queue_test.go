package sdk

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

func TestServiceBusQueueSendReceive(t *testing.T) {
	connStr := startServiceBus(t)

	client, err := azservicebus.NewClientFromConnectionString(connStr, nil)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close(context.Background())

	const queue = "myqueue"

	sender, err := client.NewSender(queue, nil)
	if err != nil {
		t.Fatalf("new sender: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := sender.SendMessage(ctx, &azservicebus.Message{Body: []byte("hello service bus")}, nil); err != nil {
		t.Fatalf("send: %v", err)
	}
	sender.Close(ctx)

	receiver, err := client.NewReceiverForQueue(queue, nil)
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
	if got := string(msgs[0].Body); got != "hello service bus" {
		t.Fatalf("body = %q, want %q", got, "hello service bus")
	}
	if err := receiver.CompleteMessage(ctx, msgs[0], nil); err != nil {
		t.Fatalf("complete: %v", err)
	}
}
