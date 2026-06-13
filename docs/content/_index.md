---
title: "localaz"
description: "A local Azure emulator — one Docker container, native Azure protocols."
---

# localaz

**localaz** is a local **Azure emulator**, in the spirit of LocalStack (AWS) and
localgcp (GCP). You run a single Docker container and point the **Azure CLI** or
any **Azure SDK** at it — no code changes required.

It speaks the native Azure service protocols, so the CLI and the official SDKs
build, sign, and parse requests exactly as they would against Azure. Everything
runs on its own port inside the one process and container.

## Supported services

| Service | Endpoint | Protocol | Persisted | Notes |
| ------- | -------- | -------- | --------- | ----- |
| [Blob Storage](services/blob/) | `http://127.0.0.1:10000/devstoreaccount1` | HTTP / REST (XML) | &#9989; | Block blobs, metadata, ranges, listing |
| [Queue Storage](services/queue/) | `http://127.0.0.1:10001/devstoreaccount1` | HTTP / REST (XML) | &#9989; | Messages, visibility timeouts, pop receipts |
| [Table Storage](services/table/) | `http://127.0.0.1:10002/devstoreaccount1` | HTTP / REST (OData JSON) | &#9989; | Entities, `$filter`, ETag concurrency |
| [Event Grid](services/event-grid/) | `http://127.0.0.1:10003` | HTTP / REST | &#10060; | Namespace topics, pull delivery |
| [Web PubSub](services/web-pubsub/) | `http://127.0.0.1:10004` | HTTP + WebSocket | &#10060; | REST broadcast + `json.webpubsub.azure.v1` |
| [Service Bus](services/service-bus/) | `sb://127.0.0.1:5672` | AMQP 1.0 over TCP | &#10060; | Queues + topic/subscription fan-out |
| [Monitor Logs](services/monitor-logs/) | `https://127.0.0.1:10005` | HTTPS / REST | &#10060; | Logs ingestion + KQL-subset query |
| [Control plane](services/control-plane/) | `https://127.0.0.1:10006`–`10007` | HTTPS / REST | &#10060; | Entra ID (AAD) + Resource Manager (ARM) |

Blob, Queue and Table use the standard Azure development-storage ports
(`10000`/`10001`/`10002`) with the well-known `devstoreaccount1` account, so the
`UseDevelopmentStorage=true` shorthand and existing tooling and connection
strings work unchanged. The pub/sub services follow on `10003`/`10004`/`10005`,
and the Entra ID + Resource Manager control plane on `10006`/`10007`.

## Architecture

localaz is a **single Go process**. Each emulated service listens on its own
port, but they all share the one binary and ship in the one container.

```mermaid
flowchart LR
    clients["Azure CLI / SDKs"]

    subgraph proc["localaz — single process & container"]
      direction TB
      storage["Storage data plane<br/>Blob · Queue · Table"]
      pubsub["Pub/sub &amp; logs<br/>Event Grid · Web PubSub<br/>Service Bus · Monitor"]
      control["Control plane<br/>Entra ID · Resource Manager"]
    end

    clients --> proc
    storage --> disk[("/data volume")]
```

- The **storage** services (Blob, Queue, Table) persist their state under
  `/data`; mount a volume to keep it across restarts.
- The **pub/sub and logs** services (Event Grid, Web PubSub, Service Bus,
  Monitor Logs) keep transient state in memory.
- The **control plane** (Entra ID + Resource Manager) lets the CLI/SDKs register
  localaz as a custom Azure cloud, sign in, and route data-plane commands to it.
  It is in-memory and requires TLS, because MSAL and azure-core refuse to send
  bearer tokens over plain HTTP.

## Next steps

- [Get Started](get-started/) — build and run the Docker image.
- [Services](services/) — per-service URLs, configuration, and SDK/CLI examples.
