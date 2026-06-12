---
title: "Get Started"
description: "Build and run the localaz Docker image."
weight: 2
---

This page walks through building the localaz container image and running it so
the Azure CLI and SDKs can talk to it.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) (with Compose).
- Optionally [Task](https://taskfile.dev) for the convenience commands below.
- The [Azure CLI](https://learn.microsoft.com/cli/azure/install-azure-cli)
  (`az`) and/or an Azure SDK, if you want to drive the emulator.

## Build the image

With Task:

```bash
task docker:build
```

Or directly with Docker:

```bash
docker build -f docker/localaz.dockerfile -t localaz:dev .
```

The image is built in two stages: a `golang:1.26-alpine` build stage compiles a
static binary, and the runtime stage is a distroless `nonroot` image. State
lives in `/data`, owned by the non-root user (uid 65532).

## Run the container

The simplest path is the development Compose stack, which exposes every service
port and mounts a named volume for persistence:

```bash
task docker:up
# or, without Task:
docker compose -f docker/docker-compose.dev.yml up --build
```

To run the image by hand, publish each service port and mount a volume:

```bash
docker run \
  -p 10000:10000 -p 10001:10001 -p 10002:10002 \
  -p 10003:10003 -p 10004:10004 -p 10005:10005 \
  -p 10006:10006 -p 10007:10007 -p 5672:5672 \
  -v localaz-data:/data localaz:dev
```

Stop the Compose stack with:

```bash
task docker:down
# or
docker compose -f docker/docker-compose.dev.yml down
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
tooling works unchanged:

| Setting | Value |
| ------- | ----- |
| Account name | `devstoreaccount1` |
| Account key | Azurite's well-known development key |

Generate a ready-to-use connection string for your shell:

```bash
eval "$(task env:conn-string)"
```

This exports `AZURE_STORAGE_CONNECTION_STRING`, which both the Azure CLI and the
SDKs pick up automatically. A quick smoke test with the CLI:

```bash
az storage container create --name demo
az storage blob upload --container-name demo --name hello.txt --file ./hello.txt
az storage blob list --container-name demo -o table
```

> The emulator accepts the Shared Key `Authorization` header (and Service Bus
> CBS tokens) but does **not** verify them, so the exact key value is not
> sensitive in this context.

## Running locally without Docker

For development you can run the binary directly:

```bash
task build && task run
# or
go run ./cmd/localaz -addr :10000 -data ./data
```

See [Services](../services/) for per-service configuration flags, environment
variables, and SDK/CLI examples.
