---
title: "Service Bus"
description: "Queues and topic/subscription fan-out over AMQP 1.0."
weight: 6
---

Azure Service Bus over **AMQP 1.0** on plain TCP at `sb://127.0.0.1:5672`,
compatible with the `azservicebus` SDK when the connection string sets
`UseDevelopmentEmulator=true` (plain TCP, SASL ANONYMOUS, CBS tokens accepted
without verification). Broker state is in-memory; entities are auto-created on
first use. See [Configuration](/configuration) to change the listen address.

## Supported operations

| Capability | API |
| ---------- | --- |
| Queue send | `client.NewSender(queue)` → `SendMessage` |
| Queue receive + settle | `client.NewReceiverForQueue(queue)` → `ReceiveMessages` / `CompleteMessage` |
| Topic send | `client.NewSender(topic)` → `SendMessage` |
| Subscription receive | `client.NewReceiverForSubscription(topic, sub)` |

Message bodies are relayed verbatim with peek-lock delivery and
disposition-based settlement; topic sends fan out to registered subscriptions.

**Not implemented:** dead-letter queues, scheduled / deferred messages,
sessions, and lock renewal.

## Azure CLI

The data plane (send / receive) has no CLI surface, but the **management plane**
runs through the emulated Resource Manager. After registering localaz as a cloud
and signing in (see [Control plane](/services/control-plane)):

```bash
az servicebus namespace create -g rg1 -n ns1 -l localaz --sku Standard
az servicebus namespace show -g rg1 -n ns1
az servicebus queue create -g rg1 --namespace-name ns1 -n q1
az servicebus topic create -g rg1 --namespace-name ns1 -n t1
az servicebus topic subscription create -g rg1 --namespace-name ns1 --topic-name t1 -n s1
az servicebus queue delete -g rg1 --namespace-name ns1 -n q1
```

## Go SDK

```go
package main

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

func main() {
	ctx := context.Background()
	const connStr = "Endpoint=sb://127.0.0.1:5672;SharedAccessKeyName=test;SharedAccessKey=test;UseDevelopmentEmulator=true"

	client, err := azservicebus.NewClientFromConnectionString(connStr, nil)
	if err != nil {
		panic(err)
	}
	defer client.Close(ctx)

	sender, _ := client.NewSender("myqueue", nil)
	sender.SendMessage(ctx, &azservicebus.Message{Body: []byte("hello service bus")}, nil)
	sender.Close(ctx)

	receiver, _ := client.NewReceiverForQueue("myqueue", nil)
	defer receiver.Close(ctx)

	msgs, err := receiver.ReceiveMessages(ctx, 1, nil)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(msgs[0].Body))
	receiver.CompleteMessage(ctx, msgs[0], nil)
}
```

> `SharedAccessKey=test` is the non-secret placeholder required by
> `UseDevelopmentEmulator=true`; localaz ignores it.
