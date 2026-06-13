---
title: "Queue Storage"
description: "Messages, visibility timeouts, and pop receipts."
weight: 2
---

Azure Queue Storage over the native REST (XML) protocol, served at
`http://127.0.0.1:10001/devstoreaccount1` and compatible with the `azqueue` SDK
and the `az storage queue` / `az storage message` commands. Queue state is
persisted under `/data`. See [Configuration](/configuration) to change ports,
addresses, and the data directory.

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

Queue metadata, the approximate message count, visibility timeouts and pop
receipts, message TTL (including the infinite `-1` TTL), and dequeue counts are
supported.

**Not implemented:** SAS and Shared Key signature verification.

## Azure CLI

```bash
export AZURE_STORAGE_CONNECTION_STRING="UseDevelopmentStorage=true"

az storage queue create --name work-items
az storage message put --queue-name work-items --content "hello-from-the-cli"
az storage message peek --queue-name work-items
az storage message get --queue-name work-items
az storage message clear --queue-name work-items
az storage queue delete --name work-items
```

## Go SDK

```go
package main

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azqueue"
)

func main() {
	ctx := context.Background()
	svc, err := azqueue.NewServiceClientWithNoCredential("http://127.0.0.1:10001/devstoreaccount1", nil)
	if err != nil {
		panic(err)
	}

	queue := svc.NewQueueClient("work-items")
	queue.Create(ctx, nil)
	queue.EnqueueMessage(ctx, "hello queue", nil)

	resp, err := queue.DequeueMessage(ctx, nil)
	if err != nil {
		panic(err)
	}
	msg := resp.Messages[0]
	fmt.Println(*msg.MessageText)

	queue.DeleteMessage(ctx, *msg.MessageID, *msg.PopReceipt, nil)
}
```
