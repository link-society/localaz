---
title: "Services"
description: "Per-service URLs, configuration, and SDK/CLI examples."
weight: 3
---

localaz emulates the following Azure services. Each runs on its own port inside
the single process. Pick a service for its endpoint, configuration flags, and
quick examples with the Go SDK and the Azure CLI.

## Configuration overview

Every service is configured through a command-line flag or an environment
variable; flags take precedence. The shared data directory flag is `-data` /
`LOCALAZ_DATA_DIR` (default `/data`).

| Service | Flag | Environment variable | Default |
| ------- | ---- | -------------------- | ------- |
| Blob | `-addr` | `LOCALAZ_BLOB_ADDR` | `:10000` |
| Queue | `-queue-addr` | `LOCALAZ_QUEUE_ADDR` | `:10001` |
| Table | `-table-addr` | `LOCALAZ_TABLE_ADDR` | `:10002` |
| Event Grid | `-eventgrid-addr` | `LOCALAZ_EVENTGRID_ADDR` | `:10003` |
| Web PubSub | `-webpubsub-addr` | `LOCALAZ_WEBPUBSUB_ADDR` | `:10004` |
| Monitor Logs | `-monitor-addr` | `LOCALAZ_MONITOR_ADDR` | `:10005` |
| Entra ID (AAD) | `-aad-addr` | `LOCALAZ_AAD_ADDR` | `:10006` |
| Resource Manager (ARM) | `-arm-addr` | `LOCALAZ_ARM_ADDR` | `:10007` |
| Service Bus | `-servicebus-addr` | `LOCALAZ_SERVICEBUS_ADDR` | `:5672` |

This table covers the listen-address flags. For the **complete** reference —
including the data directory, the control-plane options (`-advertise-host`,
`-arm-cloud-name`), and the TLS flags (`-tls-auto`, `-tls-cert`, `-tls-key`) —
see [Reference](../reference/).
