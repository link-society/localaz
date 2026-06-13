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

**Prerequisites:** localaz running with Event Grid on `http://127.0.0.1:10003`.

The emulator does not validate credentials, so any non-empty shared key works.
Build a key credential with `azcore.NewKeyCredential` and pass it to the
`WithSharedKeyCredential` client constructors. Topics and subscriptions are
created on first use, so no setup call is needed.

> **Transport note.** The `aznamespaces` SDK refuses to send a shared key over
> plain HTTP, and localaz serves Event Grid over HTTP (port `10003`). To drive
> it with this SDK, reach the endpoint over HTTPS — front it with a
> TLS-terminating proxy and point the client there with a transport that trusts
> the certificate. localaz's own suite exercises the protocol in-process this
> way; see `test/sdk/eventgrid_delivery_test.go` for the pattern.

```go
import (
    "context"

    "github.com/Azure/azure-sdk-for-go/sdk/azcore"
    "github.com/Azure/azure-sdk-for-go/sdk/azcore/messaging"
    "github.com/Azure/azure-sdk-for-go/sdk/messaging/eventgrid/aznamespaces"
)

const endpoint = "http://127.0.0.1:10003"

ctx := context.Background()
cred := azcore.NewKeyCredential("localaz-dev-key") // any non-empty key works

sender, _ := aznamespaces.NewSenderClientWithSharedKeyCredential(endpoint, "my-topic", cred, nil)

event, _ := messaging.NewCloudEvent("/orders/api", "Order.Placed", map[string]string{"orderId": "A-1001"}, nil)
sender.SendEvent(ctx, &event, nil)

receiver, _ := aznamespaces.NewReceiverClientWithSharedKeyCredential(endpoint, "my-topic", "sub1", cred, nil)
resp, _ := receiver.ReceiveEvents(ctx, nil)
for _, d := range resp.Details {
    receiver.AcknowledgeEvents(ctx, []string{*d.BrokerProperties.LockToken}, nil)
}
```

**Behavior:** published events are stored on `my-topic` and returned by the next
`ReceiveEvents` on `sub1`; acknowledging an event with its lock token settles it
so it is not redelivered. (Reaching the SDK over HTTPS requires the transport
note above.)

## Example: Azure CLI

Not applicable — Event Grid has no CLI data-plane commands that can be pointed
at a local endpoint. Use the Go SDK example above.
