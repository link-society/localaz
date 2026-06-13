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
| Monitor Logs | `https://127.0.0.1:10005` |
| Entra ID (AAD) | `https://127.0.0.1:10006` |
| Resource Manager (ARM) | `https://127.0.0.1:10007` |
| Service Bus | `sb://127.0.0.1:5672` (AMQP) |

The storage and pub/sub services speak plain HTTP. The control-plane services
(Monitor Logs, Entra ID, Resource Manager) carry bearer tokens, which the SDKs
refuse over plain HTTP — so they serve **HTTPS** once TLS is enabled. See
[Control plane](../services/control-plane/) for the TLS and sign-in recipe.

## Connect a client (storage)

localaz uses the **well-known Azure development storage account**
(`devstoreaccount1`) on the standard development ports (`10000`/`10001`/`10002`),
so the SDKs and the CLI can connect with the `UseDevelopmentStorage=true`
shorthand — there is no account name or key to write down or paste anywhere.
Export it once:

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
> CBS tokens) but does **not** verify them — any well-formed credential is
> accepted, which is what makes the shared-account shorthand work.

## Next steps

- **Storage & pub/sub** (Blob, Queue, Table, Event Grid, Web PubSub, Service Bus)
  — see [Services](../services/) for the per-service endpoint, configuration
  flags, and a Go SDK + Azure CLI tutorial for each.
- **Control plane** (Entra ID + Resource Manager) — to drive `az login`,
  `az group`, `az servicebus`, or `az monitor log-analytics query` against
  localaz, follow the TLS and cloud-registration recipe in
  [Control plane](../services/control-plane/).

