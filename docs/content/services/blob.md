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

The Blob flag is `-addr` (not `-blob-addr`) for back-compat with Azurite. The
port matches Azurite's `UseDevelopmentStorage=true` shorthand.

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

```go
connStr := os.Getenv("AZURE_STORAGE_CONNECTION_STRING")
client, _ := azblob.NewClientFromConnectionString(connStr, nil)

ctx := context.Background()
client.CreateContainer(ctx, "demo", nil)
client.UploadBuffer(ctx, "demo", "hello.txt", []byte("hi"), nil)

resp, _ := client.DownloadStream(ctx, "demo", "hello.txt", nil)
data, _ := io.ReadAll(resp.Body)
fmt.Println(string(data))
```

## Example: Azure CLI

```bash
# Export AZURE_STORAGE_CONNECTION_STRING first — see the Get Started guide.

az storage container create --name demo
az storage blob upload --container-name demo --name hello.txt --file ./hello.txt
az storage blob list --container-name demo -o table
az storage blob download --container-name demo --name hello.txt --file ./out.txt
```
