---
title: "Queue Storage"
description: "Messages, visibility timeouts, and pop receipts."
weight: 2
---

Azure Queue Storage over the native REST (XML) protocol, served at
`https://127.0.0.1:10001/devstoreaccount1` and compatible with the `azqueue` SDK
and the `az storage queue` / `az storage message` commands. Queue state is
persisted under `/data`. See [Configuration](/configuration) to change ports,
addresses, and the data directory.

localaz serves every HTTP API over TLS. Trust the self-signed certificate it
writes to `<data>/tls/localaz.crt` (`./localaz-data/tls/localaz.crt` with the
Docker volume from [Get Started](/get-started)); the examples below show how.

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

Trust localaz's certificate:

```bash
export SSL_CERT_FILE=./localaz-data/tls/localaz.crt
export REQUESTS_CA_BUNDLE="$SSL_CERT_FILE"
```

Generate a random storage key:

```bash
export AZURE_STORAGE_KEY="$(openssl rand -base64 64 | tr -d '\n')"
```

Configure the connection string and run the CLI:

```bash
export AZURE_STORAGE_CONNECTION_STRING="DefaultEndpointsProtocol=https;AccountName=devstoreaccount1;AccountKey=${AZURE_STORAGE_KEY};QueueEndpoint=https://127.0.0.1:10001/devstoreaccount1;"

az storage queue create --name work-items
az storage message put --queue-name work-items --content "hello-from-the-cli"
az storage message peek --queue-name work-items
az storage message get --queue-name work-items
az storage message clear --queue-name work-items
az storage queue delete --name work-items
```

## Go SDK

Trust localaz's certificate:

```bash
export SSL_CERT_FILE=./localaz-data/tls/localaz.crt
```

Generate a random storage key:

```bash
export AZURE_STORAGE_KEY="$(openssl rand -base64 64 | tr -d '\n')"
```

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azqueue"
)

func main() {
	ctx := context.Background()
	connStr := "DefaultEndpointsProtocol=https;AccountName=devstoreaccount1;" +
		"AccountKey=" + os.Getenv("AZURE_STORAGE_KEY") + ";" +
		"QueueEndpoint=https://127.0.0.1:10001/devstoreaccount1;"
	svc, err := azqueue.NewServiceClientFromConnectionString(connStr, nil)
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
