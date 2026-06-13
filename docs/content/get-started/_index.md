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

Storage, Event Grid, and Web PubSub are served over plain HTTP. Entra ID,
Resource Manager, and Monitor are served over HTTPS (Monitor refuses bearer
tokens over plain HTTP).

## Connect and run

Follow these four steps to go from a cold start to a verified connection.

### 1. Run the container

If you have not already, start the image as shown in
[Run the container](#run-the-container) above. Leave it running in its own
terminal (or detach it with `-d`).

### 2. Export the connection string

localaz reuses the credentials of the well-known local development storage
account and listens on the default development storage ports
(`10000`/`10001`/`10002`), so existing dev-storage tooling works unchanged. That
means the SDKs and CLI can connect with the `UseDevelopmentStorage=true`
shorthand — there is no account name or key to write down or paste anywhere:

```bash
export AZURE_STORAGE_CONNECTION_STRING="UseDevelopmentStorage=true"
```

Both the Azure CLI and the SDKs pick up `AZURE_STORAGE_CONNECTION_STRING`
automatically.

### 3. Smoke test with the Azure CLI

Create a container, upload a file, and list it back:

```bash
echo "hello localaz" > hello.txt

az storage container create --name demo
az storage blob upload --container-name demo --name hello.txt --file ./hello.txt
az storage blob list --container-name demo -o table
```

`container create` reports that the container was created:

```text
{
  "created": true
}
```

`blob upload` echoes the committed content hash:

```text
{
  "etag": "\"0x8D...\"",
  "lastModified": "2026-06-13T12:00:00+00:00"
}
```

`blob list -o table` prints one row per blob, confirming the round trip:

```text
Name        Blob Type    Blob Tier    Length    Content Type    Last Modified              Snapshot
----------  -----------  -----------  --------  --------------  -------------------------  ----------
hello.txt   BlockBlob    Hot          14        text/plain      2026-06-13T12:00:00+00:00
```

If you see `hello.txt` in that table, the CLI is talking to localaz.

> The emulator accepts the Shared Key `Authorization` header (and Service Bus
> CBS tokens) but does **not** verify them.

### 4. Next steps

See [Services](../services/) for per-service configuration flags, environment
variables, and SDK/CLI examples covering the other endpoints above.

