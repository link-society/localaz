---
title: "Get Started"
description: "Run the localaz Docker image and connect a client."
weight: 2
---

This page walks through running the localaz container and pointing the Azure CLI
and SDKs at it.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/).
- The [Azure CLI](https://learn.microsoft.com/cli/azure/install-azure-cli)
  (`az`) and/or an Azure SDK, if you want to drive the emulator.

## Run the container

Pull and run the published image from Docker Hub
([`linksociety/localaz`](https://hub.docker.com/r/linksociety/localaz)),
publishing each service port and mounting a volume for persistence:

```bash
docker run --name localaz \
  -p 10000:10000 -p 10001:10001 -p 10002:10002 \
  -p 10003:10003 -p 10004:10004 -p 10005:10005 \
  -p 10006:10006 -p 10007:10007 -p 5672:5672 \
  -v localaz-data:/data \
  linksociety/localaz:latest
```

The image is published for both `linux/amd64` and `linux/arm64` (the latter
covers Apple Silicon / macOS arm64 hosts). State lives in `/data`, owned by the
non-root user (uid 65532); the named `localaz-data` volume keeps it across
restarts.

Stop and remove the container with:

```bash
docker rm -f localaz
```

## Service endpoints

Once running, the services are available at:

| Service | Endpoint |
| ------- | -------- |
| Blob | `http://127.0.0.1:10000/devstoreaccount1` |
| Queue | `http://127.0.0.1:10001/devstoreaccount1` |
| Table | `http://127.0.0.1:10002/devstoreaccount1` |
| Event Grid | `http://127.0.0.1:10003` |
| Web PubSub | `http://127.0.0.1:10004` |
| Monitor Logs | `http://127.0.0.1:10005` |
| Entra ID (AAD) | `http://127.0.0.1:10006` (HTTPS with TLS) |
| Resource Manager (ARM) | `http://127.0.0.1:10007` (HTTPS with TLS) |
| Service Bus | `sb://127.0.0.1:5672` (AMQP) |

## Connect a client

The credentials match Azurite's well-known development account, so existing
tooling works unchanged. Because localaz uses Azurite's default storage ports,
the SDKs and CLI can connect with the `UseDevelopmentStorage=true` shorthand —
no account name or key to write down or paste anywhere:

```bash
export AZURE_STORAGE_CONNECTION_STRING="UseDevelopmentStorage=true"
```

Both the Azure CLI and the SDKs pick up `AZURE_STORAGE_CONNECTION_STRING`
automatically. A quick smoke test with the CLI:

```bash
az storage container create --name demo
az storage blob upload --container-name demo --name hello.txt --file ./hello.txt
az storage blob list --container-name demo -o table
```

> The emulator accepts the Shared Key `Authorization` header (and Service Bus
> CBS tokens) but does **not** verify them.

See [Services](../services/) for per-service configuration flags, environment
variables, and SDK/CLI examples.

