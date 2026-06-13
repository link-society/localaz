---
title: "Blob Storage"
description: "Block blobs, metadata, ranges, and container/blob listing."
weight: 1
---

Azure Blob Storage emulation: containers and block blobs over the native REST
(XML) protocol. Compatible with the `azblob` SDK and the `az storage` CLI.

## Endpoint

| | |
| --- | --- |
| URL | `http://127.0.0.1:10000/devstoreaccount1` |
| Protocol | HTTP / REST (XML) |
| Persisted | Yes — state lives under `/data` |

## Configuration

| Flag | Environment variable | Default |
| ---- | -------------------- | ------- |
| `-addr` | `LOCALAZ_BLOB_ADDR` | `:10000` |
| `-data` | `LOCALAZ_DATA_DIR` | `/data` |

The Blob flag is `-addr` (not `-blob-addr`) to match the well-known Azure
development-storage convention: port `10000` with the `devstoreaccount1`
account, which the SDKs and the CLI reach through the `UseDevelopmentStorage=true`
shorthand.

## Supported operations

| Operation | REST surface |
| --------- | ------------ |
| List Containers | `GET /{account}?comp=list` |
| Create Container | `PUT /{account}/{container}?restype=container` |
| Get Container Properties | `GET/HEAD /{account}/{container}?restype=container` |
| Delete Container | `DELETE /{account}/{container}?restype=container` |
| List Blobs (flat + hierarchical) | `GET /{account}/{container}?restype=container&comp=list` |
| Put Blob (block blob) | `PUT /{account}/{container}/{blob}` |
| Put Block / Put Block List | `PUT /{account}/{container}/{blob}?comp=block` / `comp=blocklist` |
| Get Blob (+ range requests) | `GET /{account}/{container}/{blob}` |
| Get/Head Blob Properties | `HEAD /{account}/{container}/{blob}` |
| Delete Blob | `DELETE /{account}/{container}/{blob}` |

Supported semantics include container/blob metadata (`x-ms-meta-*`), content
settings, Content-MD5 round-tripping, virtual-directory listing via a delimiter,
single-shot and staged-block uploads, and single-range `GET` requests.

**Not yet implemented:** page/append blobs, leases, snapshots, versioning, soft
delete, tags, SAS, and Shared Key signature verification.

## Example: Go SDK

This tutorial creates a container, uploads a blob, and reads it back.

> **Prerequisites:** localaz is running and `AZURE_STORAGE_CONNECTION_STRING` is
> exported — see [Get Started](../../get-started/). Install the SDK with
> `go get github.com/Azure/azure-sdk-for-go/sdk/storage/azblob`.

```go
import (
    "context"
    "fmt"
    "io"
    "os"

    "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

// The connection string carries the endpoint and credentials.
connStr := os.Getenv("AZURE_STORAGE_CONNECTION_STRING")
client, _ := azblob.NewClientFromConnectionString(connStr, nil)

ctx := context.Background()

// 1. Create a container named "demo".
client.CreateContainer(ctx, "demo", nil)

// 2. Upload a small block blob into it.
client.UploadBuffer(ctx, "demo", "hello.txt", []byte("hi"), nil)

// 3. Download it again and print the body — prints: hi
resp, _ := client.DownloadStream(ctx, "demo", "hello.txt", nil)
data, _ := io.ReadAll(resp.Body)
fmt.Println(string(data))
```

## Example: Azure CLI

The same flow with the `az storage` commands. Export
`AZURE_STORAGE_CONNECTION_STRING` first — see [Get Started](../../get-started/).

```bash
# 1. Create a container.
az storage container create --name demo

# 2. Upload a local file as a blob.
az storage blob upload --container-name demo --name hello.txt --file ./hello.txt

# 3. List the blobs in the container as a table.
az storage blob list --container-name demo -o table

# 4. Download the blob back to a local file.
az storage blob download --container-name demo --name hello.txt --file ./out.txt
```
