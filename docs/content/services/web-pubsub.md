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

Connect to a hub's REST surface and broadcast to every connected client. The
`AccessKey` is accepted but not verified.

```bash
go get github.com/Azure/azure-sdk-for-go/sdk/messaging/azwebpubsub
```

```go
import (
    "context"
    "strings"

    "github.com/Azure/azure-sdk-for-go/sdk/azcore/streaming"
    "github.com/Azure/azure-sdk-for-go/sdk/messaging/azwebpubsub"
)

ctx := context.Background()

client, _ := azwebpubsub.NewClientFromConnectionString(
    "Endpoint=http://127.0.0.1:10004;AccessKey=test;", "hub1", nil)

client.SendToAll(ctx, azwebpubsub.ContentTypeTextPlain,
    streaming.NopCloser(strings.NewReader("hello everyone")), nil)
```

## Example: Azure CLI

Not applicable — the `az webpubsub` data-plane commands cannot be redirected to
a local endpoint. Use the Go SDK example above.
