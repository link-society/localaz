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

```go
const connStr = "Endpoint=sb://127.0.0.1:5672;SharedAccessKeyName=test;SharedAccessKey=test;UseDevelopmentEmulator=true"
client, _ := azservicebus.NewClientFromConnectionString(connStr, nil)

sender, _ := client.NewSender("myqueue", nil)
sender.SendMessage(ctx, &azservicebus.Message{Body: []byte("hello")}, nil)

receiver, _ := client.NewReceiverForQueue("myqueue", nil)
msgs, _ := receiver.ReceiveMessages(ctx, 1, nil)
receiver.CompleteMessage(ctx, msgs[0], nil)
```

## Example: Azure CLI

The data plane (send/receive) has no CLI surface, but the **management plane**
runs through the emulated Resource Manager:

```bash
# After registering localaz as a cloud and signing in (see Control plane):
az servicebus namespace create --name ns1 --resource-group rg1 --location local
az servicebus queue create --namespace-name ns1 --resource-group rg1 --name myqueue
az servicebus topic create --namespace-name ns1 --resource-group rg1 --name events
```
