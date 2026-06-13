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
| Entra ID (AAD) | `https://127.0.0.1:10006` | HTTP / REST (HTTPS) |
| Resource Manager (ARM) | `https://127.0.0.1:10007` | HTTP / REST (HTTPS) |

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
credential values are not sensitive in this context.

**What success looks like.** After `az login`, `az account show` reports the
fixed subscription minted by the ARM emulator:

```console
$ az account show --query "{name:name, tenantId:tenantId, type:user.type}"
{
  "name": "localaz",
  "tenantId": "adfs",
  "type": "servicePrincipal"
}
```

From there `az group create/list/delete`, `az servicebus ...` and
`az monitor log-analytics query` all route to localaz. A resource group
created against the emulator comes back with a terminal
`provisioningState=Succeeded`:

```console
$ az group create -n rg1 -l localaz --query properties.provisioningState -o tsv
Succeeded
```

## Example: Go SDK

The official `azidentity` and `armresources` clients work unchanged once you
point them at a `cloud.Configuration` describing localaz. The AAD authority is
set with `ActiveDirectoryAuthorityHost`, and ARM is registered as the
`cloud.ResourceManager` service with both its `Endpoint` and `Audience` set to
the ARM URL (the emulator does not validate the audience, so any consistent
value works).

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

// Describe localaz as a custom cloud: AAD authority + the ARM endpoint.
localCloud := cloud.Configuration{
    ActiveDirectoryAuthorityHost: "https://127.0.0.1:10006/",
    Services: map[cloud.ServiceName]cloud.ServiceConfiguration{
        cloud.ResourceManager: {
            Endpoint: "https://127.0.0.1:10007/",
            Audience: "https://127.0.0.1:10007/",
        },
    },
}
opts := azcore.ClientOptions{Cloud: localCloud}

// Tenant "adfs" lets MSAL skip public instance discovery; the client id and
// secret are accepted but never validated. DisableInstanceDiscovery avoids a
// round trip to the (non-existent) public instance metadata endpoint.
cred, err := azidentity.NewClientSecretCredential("adfs", "<app-id>", "<secret>",
    &azidentity.ClientSecretCredentialOptions{
        ClientOptions:            opts,
        DisableInstanceDiscovery: true,
    })
if err != nil {
    // handle error
}

client, err := armresources.NewResourceGroupsClient("<subscription-id>", cred,
    &arm.ClientOptions{ClientOptions: opts})
if err != nil {
    // handle error
}

resp, err := client.CreateOrUpdate(context.Background(), "rg1",
    armresources.ResourceGroup{Location: to.Ptr("localaz")}, nil)
// resp.Properties.ProvisioningState is "Succeeded"; resp.Name is "rg1".
```

The minted token carries `aud=https://management.azure.com` and `tid=adfs`,
and the resource-group response echoes back `Name="rg1"` with
`ProvisioningState="Succeeded"`. The same `localCloud` value is reused for both
the credential and every service client (the `arm` package above is
`github.com/Azure/azure-sdk-for-go/sdk/azcore/arm`).
