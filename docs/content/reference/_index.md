---
title: "Reference"
description: "The complete command-line flag, environment variable, and data-layout reference."
weight: 5
---

Every localaz setting is a command-line flag with a matching environment
variable. **Flags take precedence over environment variables**, which take
precedence over the built-in default.

## Command-line flags

```bash
localaz [flags]
```

| Flag | Environment variable | Default | Description |
| ---- | -------------------- | ------- | ----------- |
| `-addr` | `LOCALAZ_BLOB_ADDR` | `:10000` | Blob service listen address |
| `-queue-addr` | `LOCALAZ_QUEUE_ADDR` | `:10001` | Queue service listen address |
| `-table-addr` | `LOCALAZ_TABLE_ADDR` | `:10002` | Table service listen address |
| `-eventgrid-addr` | `LOCALAZ_EVENTGRID_ADDR` | `:10003` | Event Grid service listen address |
| `-webpubsub-addr` | `LOCALAZ_WEBPUBSUB_ADDR` | `:10004` | Web PubSub service listen address |
| `-monitor-addr` | `LOCALAZ_MONITOR_ADDR` | `:10005` | Monitor Logs service listen address |
| `-aad-addr` | `LOCALAZ_AAD_ADDR` | `:10006` | Entra ID (AAD) service listen address |
| `-arm-addr` | `LOCALAZ_ARM_ADDR` | `:10007` | Resource Manager (ARM) service listen address |
| `-servicebus-addr` | `LOCALAZ_SERVICEBUS_ADDR` | `:5672` | Service Bus AMQP listen address |
| `-data` | `LOCALAZ_DATA_DIR` | `/data` | Directory for persisted state |
| `-arm-cloud-name` | `LOCALAZ_ARM_CLOUD_NAME` | `localaz` | Cloud name advertised in the ARM metadata document |
| `-advertise-host` | `LOCALAZ_ADVERTISE_HOST` | `127.0.0.1` | Host clients use to reach the control-plane services |
| `-tls-cert` | `LOCALAZ_TLS_CERT` | _(unset)_ | PEM certificate for the bearer/control-plane services |
| `-tls-key` | `LOCALAZ_TLS_KEY` | _(unset)_ | PEM private key for the bearer/control-plane services |
| `-tls-auto` | `LOCALAZ_TLS_AUTO` | _(off)_ | Generate a self-signed certificate at startup |

> The Blob flag is `-addr` (not `-blob-addr`) to match the well-known Azure
> development-storage convention; the account is `devstoreaccount1`.

## Endpoints and protocols

| Service | Endpoint | Protocol | TLS |
| ------- | -------- | -------- | --- |
| Blob | `http://127.0.0.1:10000/devstoreaccount1` | HTTP / REST (XML) | No |
| Queue | `http://127.0.0.1:10001/devstoreaccount1` | HTTP / REST (XML) | No |
| Table | `http://127.0.0.1:10002/devstoreaccount1` | HTTP / REST (OData JSON) | No |
| Event Grid | `http://127.0.0.1:10003` | HTTP / REST | No |
| Web PubSub | `http://127.0.0.1:10004` | HTTP + WebSocket | No |
| Monitor Logs | `https://127.0.0.1:10005` | HTTPS / REST | Yes |
| Entra ID (AAD) | `https://127.0.0.1:10006` | HTTPS / REST | Yes |
| Resource Manager (ARM) | `https://127.0.0.1:10007` | HTTPS / REST | Yes |
| Service Bus | `sb://127.0.0.1:5672` | AMQP 1.0 over TCP | No |

The storage and pub/sub services speak plain HTTP. The control-plane services
(Monitor Logs, Entra ID, Resource Manager) carry bearer tokens, which the SDKs
refuse to send over plain HTTP — so they serve **HTTPS** once a certificate is
configured (`-tls-auto`, or `-tls-cert`/`-tls-key`).

## TLS

The control plane is unusable over plain HTTP, so enable TLS whenever you use
Entra ID, Resource Manager, or Monitor Logs:

- `-tls-auto` generates a self-signed certificate at startup and writes it to
  `<data>/tls/localaz.crt` and `<data>/tls/localaz.key` (both `0600`). The
  certificate's SANs include the loopback defaults plus `-advertise-host`.
- `-tls-cert` / `-tls-key` supply your own PEM key pair instead.

Clients must trust the certificate. For the Azure CLI, set both:

```bash
export REQUESTS_CA_BUNDLE=<data>/tls/localaz.crt
export SSL_CERT_FILE=<data>/tls/localaz.crt
export ARM_CLOUD_METADATA_URL=https://127.0.0.1:10007/metadata/endpoints
```

See [Control plane](../services/control-plane/) for the full cloud-registration
and sign-in recipe.

## Persistence and the data directory

The storage services persist under `-data` (default `/data`); the pub/sub
services and the control plane keep transient state in memory. Mount a volume at
the data directory to keep storage state across restarts.

| Path | Written by | Contents |
| ---- | ---------- | -------- |
| `<data>/<container>/…` | Blob | Blob data files and metadata (block blobs) |
| `<data>/queue/queues.json` | Queue | Queues and messages (single JSON document) |
| `<data>/table/tables.json` | Table | Tables and entities (single JSON document) |
| `<data>/aad/signing-key.pem` | Entra ID | RS256 token-signing key (PKCS#8, `0600`) — stable across restarts |
| `<data>/tls/localaz.{crt,key}` | `-tls-auto` | Generated self-signed certificate and key (`0600`) |

Every on-disk write goes through a crash-safe path (temp file → fsync → rename →
parent-dir fsync), so a crash leaves either the old file or the fully written new
one — never a truncated file.

In the Docker image the data directory is owned by the non-root user (uid
`65532`); a named volume mounted there inherits that ownership.
