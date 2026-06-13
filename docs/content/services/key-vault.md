---
title: "Key Vault"
description: "Secrets data plane with challenge-based authentication."
weight: 8
---

Azure Key Vault **secrets** data plane, served at `https://127.0.0.1:10008` and
compatible with the `azsecrets` SDK. Secrets persist to
`<data>/keyvault/secrets.json`, namespaced by vault host. See
[Configuration](/configuration) to change the listen address.

Key Vault uses **challenge-based authentication**: the first request is answered
with `401` and a `WWW-Authenticate` header, and the SDK then attaches a bearer
token and retries. localaz accepts the token but never validates it, like every
other localaz credential. The SDK refuses to send that token over plain HTTP, so
localaz serves Key Vault over TLS. Trust the self-signed certificate it writes to
`<data>/tls/localaz.crt`:

```bash
export SSL_CERT_FILE=./localaz-data/tls/localaz.crt
```

## Supported operations

| Operation | REST surface |
| --------- | ------------ |
| Set Secret | `PUT /secrets/{name}` |
| Get Secret | `GET /secrets/{name}[/{version}]` |
| Update Secret | `PATCH /secrets/{name}[/{version}]` |
| Delete Secret | `DELETE /secrets/{name}` |
| List Secrets | `GET /secrets` |
| List Secret Versions | `GET /secrets/{name}/versions` |

A secret value is immutable; `PUT` always creates a new version and makes it
current. `PATCH` updates attributes, content type and tags. Delete is a hard
delete (`recoveryLevel` is `Purgeable`).

**Not implemented:** keys, certificates, storage accounts, soft-delete /
recovery / purge, and backup / restore.

## Azure CLI

`az keyvault secret` resolves the vault host from `--vault-name` plus the active
cloud's Key Vault DNS suffix and verifies the authentication challenge resource,
neither of which fits an `IP:port` emulator endpoint. Secrets are therefore
**SDK-only** — use the Go example below.

## Go SDK

The challenge resource is the emulator host (`IP:port`), which cannot satisfy the
SDK's vault-domain check, so set `DisableChallengeResourceVerification`. Trust
localaz's certificate:

```bash
export SSL_CERT_FILE=./localaz-data/tls/localaz.crt
```

```go
package main

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
)

func main() {
	const endpoint = "https://127.0.0.1:10008"
	ctx := context.Background()

	opts := azcore.ClientOptions{}
	cred, err := azidentity.NewClientSecretCredential("adfs", "<app-id>", "<secret>",
		&azidentity.ClientSecretCredentialOptions{ClientOptions: opts, DisableInstanceDiscovery: true})
	if err != nil {
		panic(err)
	}

	client, err := azsecrets.NewClient(endpoint, cred, &azsecrets.ClientOptions{
		ClientOptions:                        opts,
		DisableChallengeResourceVerification: true,
	})
	if err != nil {
		panic(err)
	}

	client.SetSecret(ctx, "db-password", azsecrets.SetSecretParameters{
		Value: to.Ptr("s3cr3t"),
		Tags:  map[string]*string{"env": to.Ptr("dev")},
	}, nil)

	got, err := client.GetSecret(ctx, "db-password", "", nil)
	if err != nil {
		panic(err)
	}
	fmt.Println(*got.Value)

	pager := client.NewListSecretPropertiesPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			panic(err)
		}
		for _, item := range page.Value {
			fmt.Println(item.ID.Name())
		}
	}
}
```
