---
title: "Event Grid"
description: "Namespace topics with pull delivery."
weight: 4
---

Azure Event Grid namespace topics with **pull delivery** (api-version
`2024-06-01`), served at `https://127.0.0.1:10003` and compatible with the
`aznamespaces` SDK. State is in-memory; topics and subscriptions are created on
first use. See [Configuration](/configuration) to change the listen address.

The `aznamespaces` SDK refuses to send its shared-key credential over plain
HTTP, so localaz serves Event Grid over TLS like every other HTTP API. Trust the
self-signed certificate it writes to `<data>/tls/localaz.crt`:

```bash
export SSL_CERT_FILE=./localaz-data/tls/localaz.crt
```

## Supported operations

| Operation | REST surface |
| --------- | ------------ |
| Publish Cloud Event(s) | `POST /topics/{topic}:publish` |
| Receive | `POST /topics/{topic}/eventsubscriptions/{sub}:receive` |
| Acknowledge | `POST /topics/{topic}/eventsubscriptions/{sub}:acknowledge` |
| Release | `POST /topics/{topic}/eventsubscriptions/{sub}:release` |
| Reject | `POST /topics/{topic}/eventsubscriptions/{sub}:reject` |
| Renew Locks | `POST /topics/{topic}/eventsubscriptions/{sub}:renewLock` |

CloudEvents are stored and echoed back verbatim (single and batch). Pull
delivery uses lock tokens and delivery counts, with acknowledge / release /
reject settlement.

**Not implemented:** push delivery, event domains, filtering, and dead-lettering.

## Azure CLI

Event Grid has no data-plane CLI surface (`az eventgrid` is management-plane
only), so publish and receive are **SDK-only** — use the Go example below.

## Go SDK

Trust localaz's certificate:

```bash
export SSL_CERT_FILE=./localaz-data/tls/localaz.crt
```

```go
package main

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/messaging"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/eventgrid/aznamespaces"
)

func main() {
	const endpoint = "https://127.0.0.1:10003"
	ctx := context.Background()
	cred := azcore.NewKeyCredential("localaz-dev-key")

	sender, err := aznamespaces.NewSenderClientWithSharedKeyCredential(endpoint, "orders", cred, nil)
	if err != nil {
		panic(err)
	}
	event, _ := messaging.NewCloudEvent("/orders/api", "Order.Placed",
		map[string]string{"orderId": "A-1001", "status": "placed"}, nil)
	sender.SendEvent(ctx, &event, nil)

	receiver, err := aznamespaces.NewReceiverClientWithSharedKeyCredential(endpoint, "orders", "fulfillment", cred, nil)
	if err != nil {
		panic(err)
	}
	resp, err := receiver.ReceiveEvents(ctx, &aznamespaces.ReceiveEventsOptions{MaxEvents: to.Ptr[int32](5)})
	if err != nil {
		panic(err)
	}
	for _, d := range resp.Details {
		fmt.Println(d.Event.Type)
		receiver.AcknowledgeEvents(ctx, []string{*d.BrokerProperties.LockToken}, nil)
	}
}
```
