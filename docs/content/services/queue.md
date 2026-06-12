---
title: "Queue Storage"
description: "Messages, visibility timeouts, and pop receipts."
weight: 2
---

Azure Queue Storage emulation over the native REST (XML) protocol. Compatible
with the `azqueue` SDK and the `az storage queue` / `az storage message` CLI
commands.

## Endpoint

| | |
| --- | --- |
| URL | `http://127.0.0.1:10001/devstoreaccount1` |
| Protocol | HTTP / REST (XML) |
| Persisted | Yes — state lives under `/data` |

## Configuration

| Flag | Environment variable | Default |
| ---- | -------------------- | ------- |
| `-queue-addr` | `LOCALAZ_QUEUE_ADDR` | `:10001` |
| `-data` | `LOCALAZ_DATA_DIR` | `/data` |

## Supported operations

| Operation | REST surface |
| --------- | ------------ |
| List Queues | `GET /{account}?comp=list` |
| Create / Delete Queue | `PUT` / `DELETE /{account}/{queue}` |
| Get / Set Queue Metadata | `GET` / `PUT /{account}/{queue}?comp=metadata` |
| Put Message | `POST /{account}/{queue}/messages` |
| Get / Peek Messages | `GET /{account}/{queue}/messages[?peekonly=true]` |
| Update Message | `PUT /{account}/{queue}/messages/{messageid}` |
| Delete Message | `DELETE /{account}/{queue}/messages/{messageid}` |
| Clear Messages | `DELETE /{account}/{queue}/messages` |

Supported semantics: queue metadata and the approximate message count, dequeue
visibility timeouts and pop receipts, message TTL (including the infinite `-1`
TTL with eviction on access), and dequeue counts.

**Not yet implemented:** SAS, and Shared Key signature verification.

## Example: Go SDK

```go
connStr := os.Getenv("AZURE_STORAGE_CONNECTION_STRING")
client, _ := azqueue.NewServiceClientFromConnectionString(connStr, nil)
queue := client.NewQueueClient("work-items")

ctx := context.Background()
queue.Create(ctx, nil)
queue.EnqueueMessage(ctx, "hello", nil)

resp, _ := queue.DequeueMessage(ctx, nil)
msg := resp.Messages[0]
queue.DeleteMessage(ctx, *msg.MessageID, *msg.PopReceipt, nil)
```

## Example: Azure CLI

```bash
# Export AZURE_STORAGE_CONNECTION_STRING first — see the Get Started guide.

az storage queue create --name work-items
az storage message put --queue-name work-items --content "hello"
az storage message get --queue-name work-items
az storage message peek --queue-name work-items
```
