---
title: "Service Bus"
description: "Queues and topic/subscription fan-out over AMQP 1.0."
weight: 6
---

Azure Service Bus emulation over **AMQP 1.0** on plain TCP. Compatible with the
`azservicebus` SDK when the connection string sets `UseDevelopmentEmulator=true`
(plain TCP, SASL ANONYMOUS, CBS tokens accepted without verification).

## Endpoint

| | |
| --- | --- |
| URL | `sb://127.0.0.1:5672` |
| Protocol | AMQP 1.0 over TCP |
| Persisted | No — broker state is in-memory |

Connection string for the development emulator:

```text
Endpoint=sb://127.0.0.1:5672;SharedAccessKeyName=test;SharedAccessKey=test;UseDevelopmentEmulator=true
```

## Configuration

| Flag | Environment variable | Default |
| ---- | -------------------- | ------- |
| `-servicebus-addr` | `LOCALAZ_SERVICEBUS_ADDR` | `:5672` |

## Supported capabilities

| Capability | Notes |
| ---------- | ----- |
| Queue send | `client.NewSender(queue)` → `SendMessage` |
| Queue receive + settle | `client.NewReceiverForQueue(queue)` → `ReceiveMessages` / `CompleteMessage` |
| Topic send | `client.NewSender(topic)` → `SendMessage` |
| Subscription receive | `client.NewReceiverForSubscription(topic, sub)` |

Message bodies are relayed verbatim. Peek-lock delivery with disposition-based
settlement; topic sends fan out to registered subscriptions. Entities are
auto-created on first data-plane use.

The **management plane** is served by the control-plane ARM resource provider
(`Microsoft.ServiceBus`), so `az servicebus` namespace/queue/topic/subscription
commands work too — see [Control plane](../control-plane/).

**Not yet implemented:** dead-letter queues, scheduled/deferred messages,
sessions, and lock renewal.

## Example: Go SDK

This tutorial sends a message to a queue, then receives and completes it. The
queue is auto-created on first use.

> **Prerequisites:** localaz is running. Install the SDK with
> `go get github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus`. The
> `UseDevelopmentEmulator=true` flag tells the SDK to use plain TCP and SASL
> ANONYMOUS — no TLS — which is how localaz speaks AMQP.

```go
import (
    "context"

    "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

ctx := context.Background()

const connStr = "Endpoint=sb://127.0.0.1:5672;SharedAccessKeyName=test;SharedAccessKey=test;UseDevelopmentEmulator=true"
client, _ := azservicebus.NewClientFromConnectionString(connStr, nil)

// 1. Send a message to the queue.
sender, _ := client.NewSender("myqueue", nil)
sender.SendMessage(ctx, &azservicebus.Message{Body: []byte("hello")}, nil)

// 2. Receive it under a peek-lock.
receiver, _ := client.NewReceiverForQueue("myqueue", nil)
msgs, _ := receiver.ReceiveMessages(ctx, 1, nil)

// 3. Complete it so it is removed from the queue.
receiver.CompleteMessage(ctx, msgs[0], nil)
```

## Example: Azure CLI

The data plane (send/receive) has no CLI surface, but the **management plane**
runs through the emulated Resource Manager. First register localaz as a cloud and
sign in — see [Control plane](../control-plane/).

```bash
# Create a namespace, a queue, and a topic via ARM.
az servicebus namespace create --name ns1 --resource-group rg1 --location local
az servicebus queue create --namespace-name ns1 --resource-group rg1 --name myqueue
az servicebus topic create --namespace-name ns1 --resource-group rg1 --name events
```
