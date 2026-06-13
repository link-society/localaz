---
title: "Web PubSub"
description: "REST broadcast plus the json.webpubsub.azure.v1 WebSocket subprotocol."
weight: 5
---

Azure Web PubSub emulation: a REST management surface plus WebSocket client
connections using the `json.webpubsub.azure.v1` subprotocol. Compatible with the
`azwebpubsub` SDK.

## Endpoint

| | |
| --- | --- |
| URL | `http://127.0.0.1:10004` |
| Protocol | HTTP + WebSocket |
| Persisted | No — hub state is in-memory |

## Configuration

| Flag | Environment variable | Default |
| ---- | -------------------- | ------- |
| `-webpubsub-addr` | `LOCALAZ_WEBPUBSUB_ADDR` | `:10004` |

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

> Web PubSub is **SDK-only** for the CLI: `az webpubsub service` resolves its
> endpoint from ARM via `-n`/`-g`, with no `--endpoint`/`--connection-string`
> override to redirect it to `127.0.0.1`.

## Example: Go SDK

Prerequisites: localaz running with Web PubSub on `:10004`, and the
`github.com/Azure/azure-sdk-for-go/sdk/messaging/azwebpubsub` SDK installed.

```go
import (
    "bytes"
    "context"
    "encoding/json"

    "github.com/Azure/azure-sdk-for-go/sdk/azcore/streaming"
    "github.com/Azure/azure-sdk-for-go/sdk/messaging/azwebpubsub"
)

ctx := context.Background()

// The endpoint is HTTP (Web PubSub serves no TLS); the AccessKey is signed
// locally, so any non-empty value works.
client, _ := azwebpubsub.NewClientFromConnectionString(
    "Endpoint=http://127.0.0.1:10004;AccessKey=localaz-test-key;", nil)

// Broadcast to every client connected to the hub. The hub name is the first
// argument; SendToAll has no separate hub on the client.
body, _ := json.Marshal(map[string]string{"text": "hello everyone"})
client.SendToAll(ctx, "chat", azwebpubsub.ContentTypeApplicationJSON,
    streaming.NopCloser(bytes.NewReader(body)), nil)
```

Expected result: every WebSocket client connected to the `chat` hub receives
the broadcast as a `message` frame (`from: "server"`) carrying the JSON payload.

## Example: Azure CLI

Not applicable — the `az webpubsub` data-plane commands cannot be redirected to
a local endpoint. Use the Go SDK example above.
