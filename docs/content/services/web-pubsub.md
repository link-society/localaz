---
title: "Web PubSub"
description: "REST broadcast plus the json.webpubsub.azure.v1 WebSocket subprotocol."
weight: 5
---

Azure Web PubSub: a REST management surface plus WebSocket client connections
using the `json.webpubsub.azure.v1` subprotocol, served at
`http://127.0.0.1:10004` and compatible with the `azwebpubsub` SDK. Hub state is
in-memory. See [Configuration](/configuration) to change the listen address.

## Supported operations

| Operation | Surface |
| --------- | ------- |
| Client connect | `GET /client/hubs/{hub}` (WebSocket upgrade) |
| Send to all | `POST /api/hubs/{hub}/:send` |
| Send to group | `POST /api/hubs/{hub}/groups/{group}/:send` |
| Send to user | `POST /api/hubs/{hub}/users/{user}/:send` |
| Send to connection | `POST /api/hubs/{hub}/connections/{conn}/:send` |
| Add / remove from group | `PUT` / `DELETE /api/hubs/{hub}/groups/{group}/connections/{conn}` |
| Close connection | `DELETE /api/hubs/{hub}/connections/{conn}` |

Client access URLs are signed locally; group join/leave and acks over the
WebSocket are supported.

**Not implemented:** the MQTT subprotocol, event handlers / webhooks, and
connection authentication.

## Azure CLI

`az webpubsub` resolves its endpoint from ARM with no local override, so the
data plane is **SDK-only** — use the Go example below.

## Go SDK

```go
package main

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/streaming"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azwebpubsub"
)

func main() {
	ctx := context.Background()
	client, err := azwebpubsub.NewClientFromConnectionString(
		"Endpoint=http://127.0.0.1:10004;AccessKey=localaz-test-key;", nil)
	if err != nil {
		panic(err)
	}

	body, _ := json.Marshal(map[string]string{"text": "hello room"})
	client.SendToGroup(ctx, "chat", "room1", azwebpubsub.ContentTypeApplicationJSON,
		streaming.NopCloser(bytes.NewReader(body)), nil)

	client.SendToAll(ctx, "chat", azwebpubsub.ContentTypeApplicationJSON,
		streaming.NopCloser(bytes.NewReader(body)), nil)
}
```
