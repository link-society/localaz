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

## Control Plane Configuration

| Description | Flag | Environment variable | Default |
| ----------- | ---- | -------------------- | ------- |
| Entra ID (AAD) address | `-aad-addr` | `LOCALAZ_AAD_ADDR` | `:10006` |
| Resource Manager (ARM) address | `-arm-addr` | `LOCALAZ_ARM_ADDR` | `:10007` |
| Cloud Name | `-arm-cloud-name` | `LOCALAZ_ARM_CLOUD_NAME` | `localaz` |
| Hosts used to reach the control plane | `-advertise-host` | `LOCALAZ_ADVERTISE_HOST` | `127.0.0.1` |

## TLS Configuration

| Description | Flag | Environment variable | Default |
| ----------- | ---- | -------------------- | ------- |
| Generate a self-signed certificate | `-tls-auto` | `LOCALAZ_TLS_AUTO` | N/A |
| Path to the PEM encoded public key of a certificate | `-tls-cert` | `LOCALAZ_TLS_CERT` | N/A |
| Path to the PEM encoded private key of a certificate | `-tls-key` | `LOCALAZ_TLS_KEY` | N/A |

If `-tls-auto` is specified, then `-tls-cert` and `-tls-key` are ignored.

## Storage Configuration

| Description | Flag | Environment variable | Default |
| ----------- | ---- | -------------------- | ------- |
| Path to on-disk directory where to store persisted data | `-data` | `LOCALAZ_DATA_DIR` | `/data` |
