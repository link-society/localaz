---
title: "Event Grid"
description: "Namespace topics with pull delivery."
weight: 4
---

Azure Event Grid emulation — namespace topics with **pull delivery**
(api-version `2024-06-01`). Compatible with the `aznamespaces` SDK.

## Endpoint

| | |
| --- | --- |
| URL | `http://127.0.0.1:10003` |
| Protocol | HTTP / REST |
| Persisted | No — state is in-memory |

## Configuration

| Flag | Environment variable | Default |
| ---- | -------------------- | ------- |
| `-eventgrid-addr` | `LOCALAZ_EVENTGRID_ADDR` | `:10003` |

## Supported operations

| Operation | REST surface |
| --------- | ------------ |
| Publish Cloud Event(s) | `POST /topics/{topic}:publish` |
| Receive | `POST /topics/{topic}/eventsubscriptions/{sub}:receive` |
| Acknowledge | `POST /topics/{topic}/eventsubscriptions/{sub}:acknowledge` |
| Release | `POST /topics/{topic}/eventsubscriptions/{sub}:release` |
| Reject | `POST /topics/{topic}/eventsubscriptions/{sub}:reject` |
| Renew Locks | `POST /topics/{topic}/eventsubscriptions/{sub}:renewLock` |

CloudEvents are stored and echoed back verbatim (single and batch). Topics and
subscriptions are created on first use. Pull delivery uses lock tokens and
delivery counts, with acknowledge/release/reject settlement.

> Event Grid is **SDK-only**: `az eventgrid` is an ARM management-plane command,
> so there is no CLI data-plane publish/receive surface to redirect to localaz.

## Example: Go SDK

This tutorial publishes a CloudEvent to a namespace topic, then pulls and
acknowledges it from a subscription.

> **Prerequisites:** localaz is running. Install the SDK with
> `go get github.com/Azure/azure-sdk-for-go/sdk/messaging/eventgrid/aznamespaces`.
> localaz does **not** verify credentials, so any `azcore.TokenCredential`
> works for `cred`.

```go
import (
    "context"

    "github.com/Azure/azure-sdk-for-go/sdk/messaging"
    "github.com/Azure/azure-sdk-for-go/sdk/messaging/eventgrid/aznamespaces"
)

ctx := context.Background()
const endpoint = "http://127.0.0.1:10003" // the local Event Grid endpoint

// 1. Publish a CloudEvent to the topic (created on first use).
sender, _ := aznamespaces.NewSenderClient(endpoint, "my-topic", cred, nil)
event, _ := messaging.NewCloudEvent("source", "type", map[string]string{"k": "v"}, nil)
sender.SendEvent(ctx, &event, nil)

// 2. Pull events from a subscription (pull delivery).
receiver, _ := aznamespaces.NewReceiverClient(endpoint, "my-topic", "sub1", cred, nil)
resp, _ := receiver.ReceiveEvents(ctx, nil)

// 3. Acknowledge each event by its broker lock token so it is settled.
for _, d := range resp.Details {
    receiver.AcknowledgeEvents(ctx, []string{*d.BrokerProperties.LockToken}, nil)
}
```

## Example: Azure CLI

Not applicable — Event Grid has no CLI data-plane commands that can be pointed
at a local endpoint. Use the Go SDK example above.
