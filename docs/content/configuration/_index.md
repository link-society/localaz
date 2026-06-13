---
title: "Configuration"
weight: 4
---

## Service Configuration

| Service | Flag | Environment variable | Default |
| ------- | ---- | -------------------- | ------- |
| Blob Storage | `-blob-addr` | `LOCALAZ_BLOB_ADDR` | `:10000` |
| Queue Storage | `-queue-addr` | `LOCALAZ_QUEUE_ADDR` | `:10001` |
| Table Storage | `-table-addr` | `LOCALAZ_TABLE_ADDR` | `:10002` |
| Event Grid | `-eventgrid-addr` | `LOCALAZ_EVENTGRID_ADDR` | `:10003` |
| Web PubSub | `-webpubsub-addr` | `LOCALAZ_WEBPUBSUB_ADDR` | `:10004` |
| Service Bus | `-servicebus-addr` | `LOCALAZ_SERVICEBUS_ADDR` | `:5672` |
| Monitor Logs | `-monitor-addr` | `LOCALAZ_MONITOR_ADDR` | `:10005` |
| Key Vault | `-keyvault-addr` | `LOCALAZ_KEYVAULT_ADDR` | `:10008` |

## Control Plane Configuration

| Description | Flag | Environment variable | Default |
| ----------- | ---- | -------------------- | ------- |
| Entra ID (AAD) address | `-aad-addr` | `LOCALAZ_AAD_ADDR` | `:10006` |
| Resource Manager (ARM) address | `-arm-addr` | `LOCALAZ_ARM_ADDR` | `:10007` |
| Cloud Name | `-arm-cloud-name` | `LOCALAZ_ARM_CLOUD_NAME` | `localaz` |
| Hosts used to reach the control plane | `-advertise-host` | `LOCALAZ_ADVERTISE_HOST` | `127.0.0.1` |

## TLS Configuration

Every HTTP service is served over TLS (Service Bus, being AMQP, is the only
plaintext listener). When no certificate is supplied, localaz generates a
self-signed one and writes it to `<data>/tls/localaz.{crt,key}` on startup.
Trust that certificate in your clients:

```bash
export SSL_CERT_FILE=<data>/tls/localaz.crt
```

| Description | Flag | Environment variable | Default |
| ----------- | ---- | -------------------- | ------- |
| Path to the PEM encoded public key of a certificate | `-tls-cert` | `LOCALAZ_TLS_CERT` | auto-generated |
| Path to the PEM encoded private key of a certificate | `-tls-key` | `LOCALAZ_TLS_KEY` | auto-generated |

Supply both `-tls-cert` and `-tls-key` to serve your own certificate instead of
the auto-generated one.

## Storage Configuration

| Description | Flag | Environment variable | Default |
| ----------- | ---- | -------------------- | ------- |
| Path to on-disk directory where to store persisted data | `-data` | `LOCALAZ_DATA_DIR` | `/data` |
