# localaz

A local **Azure emulator**, in the spirit of LocalStack (AWS) and localgcp
(GCP). You run a single Docker container and point the **Azure CLI** or any
**Azure SDK** at it — no code changes required.

## Quick start

```bash
task docker:up
# or, without Task:
docker compose -f docker/docker-compose.dev.yml up --build
```

The Blob endpoint is then available at `http://127.0.0.1:10000/devstoreaccount1`.

Generate a connection string for your shell (matches Azurite's well-known
development credentials, so existing tooling works unchanged):

```bash
eval "$(task env:conn-string)"
```

This exports `AZURE_STORAGE_CONNECTION_STRING`, which both the Azure CLI and the
SDKs pick up automatically.

### With the Azure CLI

```bash
az storage container create --name demo
az storage blob upload --container-name demo --name hello.txt --file ./hello.txt
az storage blob list --container-name demo -o table
```

### With the Go SDK

```go
connStr := os.Getenv("AZURE_STORAGE_CONNECTION_STRING")
client, _ := azblob.NewClientFromConnectionString(connStr, nil)
client.UploadBuffer(ctx, "demo", "hello.txt", []byte("hi"), nil)
```

## Common tasks

```bash
task                 # list all tasks
task build           # build the binary
task run             # run locally
task test:unit       # Go SDK integration suite
task test:e2e        # Azure CLI end-to-end suite (requires az)
task lint            # gofmt check + go vet
task docker:build    # build the container image
task docker:up       # run the dev compose stack
task docker:down     # stop the dev compose stack
task env:conn-string # print an export line for the connection string
```

## Documentation

- [Architecture](ARCHITECTURE.md) — process layout and design decisions.
- [Configuration](docs/configuration.md) — flags, env vars, endpoint and credentials.
- [Supported APIs](docs/supported-apis.md) — implemented Blob operations and roadmap.
- [Testing](docs/testing.md) — running the suites and why the E2E suite is in Go.
- [AGENTS.md](AGENTS.md) — guide for AI agents and contributors working on the code.

## Roadmap

- Queue Storage
- Table Storage
- Service Bus (pub/sub)
- Optional Shared Key signature verification
