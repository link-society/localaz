---
title: "Control plane (Entra ID + ARM)"
description: "Register localaz as a custom Azure cloud, sign in, and route data-plane commands."
weight: 9
---

The **Entra ID (AAD)** and **Resource Manager (ARM)** emulators let the Azure
CLI and SDKs treat localaz as a custom Azure cloud: register it once, sign in
with a service principal, and data-plane commands (`az monitor log-analytics
query`, `az servicebus`, ...) are routed to localaz. AAD is served at
`https://127.0.0.1:10006` and ARM at `https://127.0.0.1:10007`.

State is in-memory: one fixed subscription and tenant, plus resource groups and
generic resources created at runtime.

**TLS is required** — MSAL and azure-core refuse bearer tokens over plain HTTP.
localaz serves TLS by default; trust the self-signed certificate it writes to
`<data>/tls/localaz.crt`. See [Configuration](/configuration) for the TLS flags.

## Supported operations

### Entra ID (AAD)

| Operation | REST surface |
| --------- | ------------ |
| OpenID configuration | `GET /{tenant}/.well-known/openid-configuration` |
| JWKS (signing keys) | `GET /{tenant}/discovery/keys` |
| Token | `POST /{tenant}/oauth2/token` (and `/v2.0/token`) |

Tokens are hand-rolled **RS256** JWTs that verify against the published JWKS.
Client id, secret and assertions are accepted but not validated.

### Resource Manager (ARM)

| Operation | REST surface |
| --------- | ------------ |
| Cloud metadata | `GET /metadata/endpoints` |
| Tenants / subscriptions | `GET /tenants`, `GET /subscriptions` |
| Resource groups | `GET`/`PUT`/`DELETE /subscriptions/{id}/resourcegroups/{name}` |
| Generic resources | `GET`/`PUT`/`DELETE .../providers/{ns}/{type}/{name}` |

A generic resource-provider surface stores any provider body and echoes it back
with `provisioningState=Succeeded`, exercised end to end by `az servicebus`.

**Not implemented:** token signature / scope validation and RBAC,
device-code / interactive logins, long-running-operation polling, and
subscription / tenant management.

## Azure CLI

Trust the self-signed certificate localaz writes to `<data>/tls/localaz.crt`,
then register the cloud. The registered name must equal `-arm-cloud-name`
(default `localaz`):

```bash
export REQUESTS_CA_BUNDLE=./localaz-data/tls/localaz.crt
export SSL_CERT_FILE=./localaz-data/tls/localaz.crt
export ARM_CLOUD_METADATA_URL=https://127.0.0.1:10007/metadata/endpoints

az cloud register -n localaz \
  --endpoint-active-directory https://127.0.0.1:10006/ \
  --endpoint-resource-manager https://127.0.0.1:10007/ \
  --endpoint-management https://127.0.0.1:10007/ \
  --endpoint-active-directory-resource-id https://127.0.0.1:10007/ \
  --suffix-storage-endpoint 127.0.0.1 --skip-endpoint-discovery
az cloud set -n localaz

az login --service-principal -u <app-id> -p <any-secret> --tenant adfs

az account show
az group create -n rg1 -l localaz
az group list
az group delete -n rg1 -y
```

The emulator validates neither the client id, secret nor tokens, so the exact
credential values are not sensitive here.

## Go SDK

```go
package main

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

func main() {
	ctx := context.Background()
	const subscriptionID = "00000000-0000-0000-0000-000000000000"

	localCloud := cloud.Configuration{
		ActiveDirectoryAuthorityHost: "https://127.0.0.1:10006/",
		Services: map[cloud.ServiceName]cloud.ServiceConfiguration{
			cloud.ResourceManager: {
				Endpoint: "https://127.0.0.1:10007",
				Audience: "https://127.0.0.1:10007",
			},
		},
	}
	opts := azcore.ClientOptions{Cloud: localCloud}

	cred, err := azidentity.NewClientSecretCredential("adfs", "<app-id>", "<secret>",
		&azidentity.ClientSecretCredentialOptions{ClientOptions: opts, DisableInstanceDiscovery: true})
	if err != nil {
		panic(err)
	}

	client, err := armresources.NewResourceGroupsClient(subscriptionID, cred,
		&arm.ClientOptions{ClientOptions: opts})
	if err != nil {
		panic(err)
	}

	rg, err := client.CreateOrUpdate(ctx, "rg1",
		armresources.ResourceGroup{Location: to.Ptr("localaz")}, nil)
	if err != nil {
		panic(err)
	}
	fmt.Println(*rg.Name, *rg.Properties.ProvisioningState)
}
```
