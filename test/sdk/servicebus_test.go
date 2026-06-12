package sdk

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"

	"localaz.dev/internal/sbserver"
	"localaz.dev/internal/sbstore"
)

// startServiceBus boots the emulator's AMQP listener on a random local port and
// returns a connection string that uses the development-emulator flag (plain
// TCP + anonymous SASL).
func startServiceBus(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	server := sbserver.New(sbstore.New())
	go server.Serve(listener)
	t.Cleanup(func() { listener.Close() })

	port := listener.Addr().(*net.TCPAddr).Port
	return fmt.Sprintf(
		"Endpoint=sb://127.0.0.1:%d;SharedAccessKeyName=test;SharedAccessKey=test;UseDevelopmentEmulator=true",
		port,
	)
}

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
