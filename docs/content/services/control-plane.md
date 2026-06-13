---
title: "Control plane (Entra ID + ARM)"
description: "Register localaz as a custom Azure cloud, sign in, and route data-plane commands."
weight: 8
---

The **Entra ID (AAD)** and **Resource Manager (ARM)** emulators let the Azure
CLI and the Azure SDKs treat localaz as a custom Azure cloud: register it once,
`az login` with a service principal, and data-plane commands (such as
`az monitor log-analytics query` or `az servicebus`) are routed to localaz.

## Endpoints

| Service | URL | Protocol |
| ------- | --- | -------- |
| Entra ID (AAD) | `https://127.0.0.1:10006` | HTTPS / REST |
| Resource Manager (ARM) | `https://127.0.0.1:10007` | HTTPS / REST |

State is in-memory and transient: a single fixed subscription and tenant, plus
resource groups and generic resources created at runtime.

## Configuration

| Flag | Environment variable | Default | Description |
| ---- | -------------------- | ------- | ----------- |
| `-aad-addr` | `LOCALAZ_AAD_ADDR` | `:10006` | Entra ID listen address |
| `-arm-addr` | `LOCALAZ_ARM_ADDR` | `:10007` | Resource Manager listen address |
| `-arm-cloud-name` | `LOCALAZ_ARM_CLOUD_NAME` | `localaz` | Cloud name in the ARM metadata document |
| `-advertise-host` | `LOCALAZ_ADVERTISE_HOST` | `127.0.0.1` | Host clients use to reach the control plane |
| `-tls-cert` | `LOCALAZ_TLS_CERT` | _(unset)_ | PEM certificate for the control plane |
| `-tls-key` | `LOCALAZ_TLS_KEY` | _(unset)_ | PEM private key for the control plane |
| `-tls-auto` | `LOCALAZ_TLS_AUTO` | _(off)_ | Generate a self-signed certificate at startup |

> **TLS is required.** MSAL and azure-core refuse to send bearer tokens over
> plain HTTP. Either supply `-tls-cert`/`-tls-key` or use `-tls-auto`, which
> writes `<data>/tls/localaz.crt` and `localaz.key`. When TLS is enabled, the
> Monitor, AAD and ARM services serve HTTPS.

## Supported operations

### Entra ID (AAD)

| Operation | REST surface |
| --------- | ------------ |
| OpenID configuration | `GET /{tenant}/.well-known/openid-configuration` |
| JWKS (signing keys) | `GET /{tenant}/discovery/keys` |
| Token | `POST /{tenant}/oauth2/token` (and `/v2.0/token`) |

Tokens are minted as hand-rolled **RS256** JWTs and verify against the published
JWKS. Client id, secret and assertions are accepted but not validated.

### Resource Manager (ARM)

| Operation | REST surface |
| --------- | ------------ |
| Cloud metadata | `GET /metadata/endpoints` |
| List tenants / subscriptions | `GET /tenants`, `GET /subscriptions` |
| Resource groups | `GET`/`PUT`/`DELETE /subscriptions/{id}/resourcegroups/{name}` |
| Generic resources | `GET`/`PUT`/`DELETE .../providers/{ns}/{type}/{name}` |
| Name availability | `POST .../providers/{ns}/checkNameAvailability` |
| List keys | `POST .../providers/{ns}/.../authorizationRules/{rule}/listKeys` |

A generic resource-provider surface stores any
`.../providers/{ns}/{type}/{name}` body and echoes it back with a terminal
`provisioningState=Succeeded`. This is exercised end to end by `az servicebus`
(`Microsoft.ServiceBus`).

**Not yet implemented:** token signature/scope validation and RBAC,
device-code/interactive logins, long-running-operation polling, and
subscription/tenant management.

## Register localaz as a cloud

Run localaz with `-tls-auto`, then trust the generated certificate and register
the cloud. The registered cloud name **must equal** `-arm-cloud-name`.

```bash
export REQUESTS_CA_BUNDLE=<data>/tls/localaz.crt
export SSL_CERT_FILE=<data>/tls/localaz.crt
export ARM_CLOUD_METADATA_URL=https://127.0.0.1:10007/metadata/endpoints

az cloud register -n localaz \
  --endpoint-active-directory https://127.0.0.1:10006/ \
  --endpoint-resource-manager https://127.0.0.1:10007/ \
  --endpoint-management https://127.0.0.1:10007/ \
  --endpoint-active-directory-resource-id https://127.0.0.1:10007/ \
  --suffix-storage-endpoint 127.0.0.1 --skip-endpoint-discovery
az cloud set -n localaz

# Sign in with the ADFS authority so MSAL skips public instance discovery.
az login --service-principal -u <app-id> -p <any-secret> --tenant adfs
```

The emulator does not validate the client id, secret or tokens, so the exact
credential values are not sensitive in this context. Once signed in,
`az account show`, `az group create/list/delete`, `az servicebus ...` and
`az monitor log-analytics query` all route to localaz.

## Example: Go SDK

This tutorial authenticates with a service-principal credential and creates a
resource group through the emulated Resource Manager. The key step is the custom
`localCloud`, which points the SDK at the local Entra ID and ARM endpoints.

> **Prerequisites:** localaz must serve HTTPS and the Go client must trust its
> certificate. See [Register localaz as a cloud](#register-localaz-as-a-cloud).
> Credentials are not validated, so the exact values are not sensitive here.

```go
import (
    "context"

    "github.com/Azure/azure-sdk-for-go/sdk/azcore"
    "github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
    "github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
    "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
    "github.com/Azure/azure-sdk-for-go/sdk/azidentity"
    "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

ctx := context.Background()

// localCloud points the SDK at the local Entra ID (sign-in) and ARM endpoints.
localCloud := cloud.Configuration{
    ActiveDirectoryAuthorityHost: "https://127.0.0.1:10006/",
    Services: map[cloud.ServiceName]cloud.ServiceConfiguration{
        cloud.ResourceManager: {
            Endpoint: "https://127.0.0.1:10007/",
            Audience: "https://127.0.0.1:10007/",
        },
    },
}

cred, _ := azidentity.NewClientSecretCredential("adfs", "<app-id>", "<secret>",
    &azidentity.ClientSecretCredentialOptions{
        ClientOptions: azcore.ClientOptions{Cloud: localCloud},
    })

client, _ := armresources.NewResourceGroupsClient("<subscription-id>", cred,
    &arm.ClientOptions{ClientOptions: azcore.ClientOptions{Cloud: localCloud}})
client.CreateOrUpdate(ctx, "rg1", armresources.ResourceGroup{Location: to.Ptr("local")}, nil)
```
