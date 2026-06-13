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

The Blob flag is `-addr` (not `-blob-addr`) for back-compat with existing
dev-storage tooling. Port `10000` matches the `UseDevelopmentStorage=true`
shorthand expected by the SDKs and CLI.

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

**Prerequisites:** export `AZURE_STORAGE_CONNECTION_STRING` so the SDK points at
localaz — see the [Get Started guide](../../get-started/).

Required imports: `context`, `fmt`, `io`, `os`, and
`github.com/Azure/azure-sdk-for-go/sdk/storage/azblob`. Every call takes a
`context.Context`.

```go
connStr := os.Getenv("AZURE_STORAGE_CONNECTION_STRING")
client, err := azblob.NewClientFromConnectionString(connStr, nil)
if err != nil {
	panic(err)
}

ctx := context.Background()

// Creates the "demo" container. Returns an error if it already exists.
client.CreateContainer(ctx, "demo", nil)

// Uploads a block blob. The SDK auto-stages blocks for large buffers.
client.UploadBuffer(ctx, "demo", "hello.txt", []byte("hi"), nil)

// Downloads the blob; data holds the bytes written above.
resp, _ := client.DownloadStream(ctx, "demo", "hello.txt", nil)
data, _ := io.ReadAll(resp.Body)
resp.Body.Close()
fmt.Println(string(data)) // prints: hi
```

Expected result: the container and blob are created, the download returns the
uploaded bytes, and the program prints `hi`.

## Example: Azure CLI

```bash
# Export AZURE_STORAGE_CONNECTION_STRING first — see the Get Started guide.

az storage container create --name demo
az storage blob upload --container-name demo --name hello.txt --file ./hello.txt
az storage blob list --container-name demo -o table
az storage blob download --container-name demo --name hello.txt --file ./out.txt
```

Expected result: the container is created, `hello.txt` is uploaded and appears in
the listing, and `download` writes the same bytes back to `./out.txt`.
