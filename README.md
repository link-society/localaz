# localaz

[![build](https://github.com/link-society/localaz/actions/workflows/build.yml/badge.svg)](https://github.com/link-society/localaz/actions/workflows/build.yml)
[![release](https://img.shields.io/github/v/release/link-society/localaz)](https://github.com/link-society/localaz/releases)
[![Docker Hub](https://img.shields.io/docker/pulls/linksociety/localaz)](https://hub.docker.com/r/linksociety/localaz)
[![Go version](https://img.shields.io/github/go-mod/go-version/link-society/localaz)](go.mod)
[![license](https://img.shields.io/badge/license-Unlicense-blue.svg)](LICENSE.txt)

A local **Azure emulator**, in the spirit of LocalStack (AWS) and localgcp
(GCP). You run a single Docker container and point the **Azure CLI** or any
**Azure SDK** at it — no code changes required.

It speaks the native Azure service protocols, so the CLI and the official SDKs
build, sign, and parse requests exactly as they would against Azure. Everything
runs on its own port inside the **one process and container**.

Documentation: **[localaz.dev](https://localaz.dev)**

## Contents

- [Supported services](#supported-services)
- [Quick start](#quick-start)
- [Connect a client](#connect-a-client)
- [Usage](#usage)
- [Configuration](#configuration)
- [Common tasks](#common-tasks)
- [Project structure](#project-structure)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [License](#license)

## Supported services

localaz currently emulates:

| Service | Endpoint | Protocol | Persisted |
| ------- | -------- | -------- | --------- |
| Blob Storage | `http://127.0.0.1:10000/devstoreaccount1` | HTTP / REST (XML) | Yes |
| Queue Storage | `http://127.0.0.1:10001/devstoreaccount1` | HTTP / REST (XML) | Yes |
| Table Storage | `http://127.0.0.1:10002/devstoreaccount1` | HTTP / REST (OData JSON) | Yes |
| Event Grid | `http://127.0.0.1:10003` | HTTP / REST | No |
| Web PubSub | `http://127.0.0.1:10004` | HTTP + WebSocket | No |
| Monitor Logs | `https://127.0.0.1:10005` | HTTPS / REST | No |
| Entra ID (AAD) | `https://127.0.0.1:10006` | HTTPS / REST | No |
| Resource Manager (ARM) | `https://127.0.0.1:10007` | HTTPS / REST | No |
| Service Bus | `sb://127.0.0.1:5672` | AMQP 1.0 over TCP | No |

The storage and pub/sub services speak plain HTTP. The **control plane**
(Entra ID + Resource Manager) and Monitor Logs carry bearer tokens, which the
SDKs refuse over plain HTTP — so they serve **HTTPS** once TLS is enabled. The
control plane lets the CLI/SDKs register localaz as a custom Azure cloud, log in,
and route data-plane commands to it.

Blob, Queue and Table use the standard Azure development-storage ports
(`10000`/`10001`/`10002`) with the well-known `devstoreaccount1` account, so the
`UseDevelopmentStorage=true` shorthand and existing tooling work unchanged.

## Quick start

The published image is on Docker Hub at
[`linksociety/localaz`](https://hub.docker.com/r/linksociety/localaz) (built for
`linux/amd64` and `linux/arm64`):

```bash
docker pull linksociety/localaz
```

Or run the development Compose stack, which builds from source and exposes every
service port:

```bash
task docker:up
# or, without Task:
docker compose -f docker/docker-compose.dev.yml up --build
```

## Connect a client

localaz uses the well-known Azure development storage account (`devstoreaccount1`)
on the standard development ports, so the SDKs and the CLI connect with the
`UseDevelopmentStorage=true` shorthand — no account name or key to write down:

```bash
eval "$(task env:conn-string)"
```

This exports `AZURE_STORAGE_CONNECTION_STRING`, which both the Azure CLI and the
SDKs pick up automatically.

> The control-plane services (Entra ID, Resource Manager, Monitor Logs) require
> TLS and a one-time cloud registration — see
> [As a custom Azure cloud](#as-a-custom-azure-cloud-entra-id--resource-manager).

## Usage

### With the Azure CLI

```bash
az storage container create --name demo
az storage blob upload --container-name demo --name hello.txt --file ./hello.txt
az storage blob list --container-name demo -o table

az storage queue create --name work-items
az storage message put --queue-name work-items --content "hello"

az storage table create --name people
az storage entity insert --table-name people --entity PartitionKey=team RowKey=alice Name=Alice
```

### With the Go SDK

```go
connStr := os.Getenv("AZURE_STORAGE_CONNECTION_STRING")
client, _ := azblob.NewClientFromConnectionString(connStr, nil)
client.UploadBuffer(ctx, "demo", "hello.txt", []byte("hi"), nil)
```

### With the Service Bus SDK

Use the development connection string (plain TCP, no TLS):

```go
const connStr = "Endpoint=sb://127.0.0.1:5672;SharedAccessKeyName=test;SharedAccessKey=test;UseDevelopmentEmulator=true"
client, _ := azservicebus.NewClientFromConnectionString(connStr, nil)
sender, _ := client.NewSender("myqueue", nil)
sender.SendMessage(ctx, &azservicebus.Message{Body: []byte("hello")}, nil)
```

### As a custom Azure cloud (Entra ID + Resource Manager)

Register localaz as a cloud, sign in, and drive data-plane commands such as
`az monitor log-analytics query` through it. The control plane needs TLS, so
run localaz with `-tls-auto` and trust the generated cert. See the
[control plane docs](https://localaz.dev/services/control-plane/) for the full
recipe:

```bash
export REQUESTS_CA_BUNDLE=<data>/tls/localaz.crt SSL_CERT_FILE=<data>/tls/localaz.crt
export ARM_CLOUD_METADATA_URL=https://127.0.0.1:10007/metadata/endpoints
az cloud register -n localaz --endpoint-resource-manager https://127.0.0.1:10007/ \
  --endpoint-active-directory https://127.0.0.1:10006/ --skip-endpoint-discovery
az cloud set -n localaz
az login --service-principal -u <app-id> -p <any-secret> --tenant adfs
```

## Configuration

Every setting is a command-line flag with a matching environment variable; flags
take precedence. The most common ones:

| Flag | Environment variable | Default | Description |
| ---- | -------------------- | ------- | ----------- |
| `-addr` | `LOCALAZ_BLOB_ADDR` | `:10000` | Blob listen address |
| `-data` | `LOCALAZ_DATA_DIR` | `/data` | Directory for persisted state |
| `-advertise-host` | `LOCALAZ_ADVERTISE_HOST` | `127.0.0.1` | Host clients use to reach the control plane |
| `-arm-cloud-name` | `LOCALAZ_ARM_CLOUD_NAME` | `localaz` | Cloud name in the ARM metadata document |
| `-tls-auto` | `LOCALAZ_TLS_AUTO` | _(off)_ | Generate a self-signed cert for the control plane |
| `-tls-cert` / `-tls-key` | `LOCALAZ_TLS_CERT` / `LOCALAZ_TLS_KEY` | _(unset)_ | Supply your own control-plane certificate |

See the **[full reference](https://localaz.dev/reference/)** for every flag, the
per-service listen addresses, and the on-disk data layout.

## Common tasks

```bash
task                 # list all tasks
task build           # build the binary
task run             # run locally
task test:unit       # Go SDK integration suite
task test:cli        # Azure CLI end-to-end suite (requires az)
task lint            # gofmt check + go vet
task docker:build    # build the container image
task docker:up       # run the dev compose stack
task docker:down     # stop the dev compose stack
task env:conn-string # print an export line for the connection string
```

## Project structure

```
cmd/localaz                    entrypoint / process wiring (one listener per service)
internal/servers/<svc>server   Azure wire protocol per service (routing, codec, errors)
internal/stores/<svc>store     state + persistence per service (behind a Store interface)
internal/utils/...             shared helpers (httpx, azwire, azerr, atomicfile, devcert)
test/sdk                       integration tests via the Azure Go SDKs
test/cli                       end-to-end tests via the Azure CLI (build tag: cli)
docker/                        localaz.dockerfile, docker-compose.dev.yml
docs/                          Hugo documentation site (published at localaz.dev)
```

See [Architecture](https://localaz.dev/architecture/) for how the pieces fit
together.

## Documentation

- **[localaz.dev](https://localaz.dev)** — full documentation: per-service
  guides, the configuration [reference](https://localaz.dev/reference/), and the
  [architecture](https://localaz.dev/architecture/) overview.
- **[AGENTS.md](AGENTS.md)** — deep contributor/internals reference (per-service
  design notes and gotchas).
- **[CONTRIBUTING.md](CONTRIBUTING.md)** — development setup, conventions, and the
  pull request workflow.
- **[CHANGELOG.md](CHANGELOG.md)** — release history.

## Contributing

Contributions are welcome! Please read **[CONTRIBUTING.md](CONTRIBUTING.md)** for
the development setup, coding conventions, commit style, and PR process. In
short: `task lint` and `task test:unit` must pass, and commits follow
[gitmoji](https://gitmoji.dev).

## License

Released into the public domain under the [Unlicense](LICENSE.txt).
