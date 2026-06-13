---
title: "Troubleshooting"
description: "Common issues, fixes, and documented limitations."
weight: 5
---

Common failure modes when running localaz and the Azure CLI / SDKs against it,
plus the limitations that are intentional by design.

## TLS / certificate trust failures

**Symptom.** The Azure CLI or an SDK reports certificate-verification errors, or
bearer-token failures, when talking to the control plane (Entra ID, Resource
Manager) or Monitor.

**Fix.** The control plane and Monitor serve HTTPS, so the client must trust the
certificate. Run localaz with `-tls-auto` (it writes `<data>/tls/localaz.crt`
and `localaz.key`) and set **both** environment variables to the generated cert:

```bash
export REQUESTS_CA_BUNDLE=<data>/tls/localaz.crt
export SSL_CERT_FILE=<data>/tls/localaz.crt
```

Set both: `AZURE_CLI_DISABLE_CONNECTION_VERIFICATION=1` covers MSAL but not the
secondary OIDC/metadata fetches, so the CA-bundle variables are the reliable
path. You may also supply your own `-tls-cert`/`-tls-key` instead of `-tls-auto`.

## "Bearer token over plain HTTP" / tokens refused

**Symptom.** MSAL or azure-core (azcore) refuses to send the bearer token, or
rejects an `http://` authority outright.

**Fix.** The control plane (`https://127.0.0.1:10006` Entra ID,
`https://127.0.0.1:10007` Resource Manager) and Monitor
(`https://127.0.0.1:10005`) must serve HTTPS. Enable TLS with `-tls-auto` (or
`-tls-cert`/`-tls-key`). Tokens are never sent over plain HTTP, so HTTPS is
mandatory for these three services.

## `CloudEndpointNotSetException` on `az` commands

**Symptom.** `az monitor log-analytics query` (or another data-plane command)
fails with `CloudEndpointNotSetException`.

**Fix.** The registered cloud name must exactly match `-arm-cloud-name`
(default `localaz`). The CLI resolves data-plane hosts by matching the active
cloud against the `name` field of the ARM `/metadata/endpoints` document; a
mismatch produces this error. Register the cloud with the same name:

```bash
az cloud register -n localaz ...
az cloud set -n localaz
```

## Ports already in use

**Symptom.** localaz fails to bind a listener at startup because a port is taken.

**Fix.** localaz uses ports `10000`–`10007` for HTTP/control-plane services and
`5672` for Service Bus (AMQP 1.0):

| Service | Port |
| ------- | ---- |
| Blob | `10000` |
| Queue | `10001` |
| Table | `10002` |
| Event Grid | `10003` |
| Web PubSub | `10004` |
| Monitor Logs | `10005` |
| Entra ID (AAD) | `10006` |
| Resource Manager (ARM) | `10007` |
| Service Bus | `5672` |

Free the conflicting port, or remap the service with its `-*-addr` flag or the
matching `LOCALAZ_*_ADDR` environment variable (for example `-queue-addr` /
`LOCALAZ_QUEUE_ADDR`). Note the Blob flag is `-addr` (not `-blob-addr`) to stay
compatible with existing dev-storage tooling.

## Cannot write to `/data`

**Symptom.** Persistence fails inside the container with permission errors when
writing to `/data`.

**Fix.** The runtime image runs as a non-root user (uid `65532`). Mount a volume
that this user can write to; the Dockerfile creates `/data` owned by
`65532:65532` so the default in-image directory is writable. If you bind-mount a
host directory, make sure it is writable by uid `65532`.

## Service Bus client cannot connect

**Symptom.** A Service Bus client times out or fails to connect to
`sb://127.0.0.1:5672`.

**Fix.** Service Bus is raw AMQP 1.0 over plain TCP (no TLS) and authenticates
with SASL ANONYMOUS. The connection string must set
`UseDevelopmentEmulator=true`, which tells the SDK to disable TLS and connect to
the local emulator. Without that flag the client tries TLS/CBS and cannot
connect.

## Documented limitations

These limits are intentional, not bugs. They keep the emulator small and
self-contained while covering the common SDK/CLI paths.

### Monitor: KQL query subset

The Log Analytics query endpoint supports only a subset of KQL:
`where` / `project` / `sort by` / `take` (`limit`) / `count` over
string/number/bool literals, with the comparison operators
`==` `!=` `<` `<=` `>` `>=` and the boolean operators `and` / `or`. There is no
`summarize`, `join`, function calls, parentheses, or timespan filtering. See the
[Monitor Logs reference](../services/monitor-logs/).

### Table: `$filter` subset

The Table `$filter` supports only `eq` / `ne` / `gt` / `ge` / `lt` / `le` over
string/number/bool literals combined with `and` / `or` and parentheses. There
are no OData functions, typed literals, or continuation tokens. A comparison
against an absent property or across incompatible types evaluates to false for
all operators (including `ne`), so a `$filter` selects only entities that have
the property and satisfy the comparison. See the
[Table reference](../services/table/).

### Control plane: not yet implemented

The Entra ID + Resource Manager control plane does not implement token
signature/scope validation and RBAC, device-code/interactive logins,
long-running-operation polling, or subscription/tenant management. Client id,
secret, assertions and tokens are accepted but never validated. See the
[control plane reference](../services/control-plane/).

### Other per-service notes

State for the pub/sub and control-plane services (Event Grid, Web PubSub,
Service Bus, Monitor Logs, AAD/ARM) is in-memory and does not survive a restart;
only Blob, Queue and Table persist under `/data`. Shared Key (storage) and CBS
(Service Bus) credentials are accepted but not validated. For the exact REST
surface and any per-operation caveats, see each
[service reference](../services/).
