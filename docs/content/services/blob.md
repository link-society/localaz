---
title: "Blob Storage"
description: "Block blobs, metadata, ranges, and container/blob listing."
weight: 1
---

Azure Blob Storage over the native REST (XML) protocol, served at
`https://127.0.0.1:10000/devstoreaccount1` and compatible with the `azblob` SDK
and the `az storage blob` / `az storage container` commands. Blob state is
persisted under `/data`. See [Configuration](/configuration) to change ports,
addresses, and the data directory.

localaz serves every HTTP API over TLS. Trust the self-signed certificate it
writes to `<data>/tls/localaz.crt` (`./localaz-data/tls/localaz.crt` with the
Docker volume from [Get Started](/get-started)); the examples below show how.

## Supported operations

| Operation | REST surface |
| --------- | ------------ |
| List Containers | `GET /{account}?comp=list` |
| Create / Delete Container | `PUT` / `DELETE /{account}/{container}?restype=container` |
| Get Container Properties | `GET`/`HEAD /{account}/{container}?restype=container` |
| List Blobs — flat & hierarchical, paginated | `GET /{account}/{container}?restype=container&comp=list` |
| Put Blob (block blob) | `PUT /{account}/{container}/{blob}` |
| Put Block / Put Block List | `PUT .../{blob}?comp=block` / `comp=blocklist` |
| Get Blob (+ range requests) | `GET /{account}/{container}/{blob}` |
| Get / Head Blob Properties | `HEAD /{account}/{container}/{blob}` |
| Delete Blob | `DELETE /{account}/{container}/{blob}` |

Metadata (`x-ms-meta-*`), content settings, Content-MD5 round-tripping,
single-shot and staged-block uploads, and virtual-directory listing via a
delimiter are supported.

**Not implemented:** page/append blobs, leases, snapshots, versioning, soft
delete, tags, SAS, and Shared Key signature verification.

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
export AZURE_STORAGE_CONNECTION_STRING="DefaultEndpointsProtocol=https;AccountName=devstoreaccount1;AccountKey=${AZURE_STORAGE_KEY};BlobEndpoint=https://127.0.0.1:10000/devstoreaccount1;"

echo "hello, localaz!" > hello.txt

az storage container create --name data
az storage blob upload --container-name data --name hello.txt --file ./hello.txt --content-type text/plain
az storage blob exists --container-name data --name hello.txt
az storage blob show --container-name data --name hello.txt
az storage blob list --container-name data -o table
az storage blob download --container-name data --name hello.txt --file ./out.txt
az storage blob delete --container-name data --name hello.txt
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
	"io"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
)

func main() {
	ctx := context.Background()
	connStr := "DefaultEndpointsProtocol=https;AccountName=devstoreaccount1;" +
		"AccountKey=" + os.Getenv("AZURE_STORAGE_KEY") + ";" +
		"BlobEndpoint=https://127.0.0.1:10000/devstoreaccount1;"
	client, err := azblob.NewClientFromConnectionString(connStr, nil)
	if err != nil {
		panic(err)
	}

	client.CreateContainer(ctx, "data", nil)
	client.UploadBuffer(ctx, "data", "hello.txt", []byte("hello, localaz!"),
		&azblob.UploadBufferOptions{
			HTTPHeaders: &blob.HTTPHeaders{BlobContentType: to.Ptr("text/plain")},
		})

	resp, err := client.DownloadStream(ctx, "data", "hello.txt", nil)
	if err != nil {
		panic(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Println(string(body))

	pager := client.NewListBlobsFlatPager("data", nil)
	for pager.More() {
		page, _ := pager.NextPage(ctx)
		for _, b := range page.Segment.BlobItems {
			fmt.Println(*b.Name)
		}
	}

	client.DeleteBlob(ctx, "data", "hello.txt", nil)
}
```
