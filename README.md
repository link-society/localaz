# localaz

A local **Azure emulator**, in the spirit of LocalStack (AWS) and localgcp
(GCP). You run a single Docker container and point the **Azure CLI** or any
**Azure SDK** at it — no code changes required.

Documentation: **[localaz.dev](https://localaz.dev)**

It currently emulates **Blob Storage**, **Queue Storage**, **Table Storage**,
**Event Grid** (namespace topics, pull delivery), **Web PubSub**, **Service
Bus** (queues and topics over AMQP), and **Monitor Logs** (ingestion and
KQL-subset queries), plus an **Entra ID (AAD) + Resource Manager (ARM) control
plane** that lets the CLI/SDKs register localaz as a custom Azure cloud, log in,
and route data-plane commands to it. Everything runs on its own port inside the
one process and container.

## Quick start

```bash
task docker:up
# or, without Task:
docker compose -f docker/docker-compose.dev.yml up --build
```

The services are then available at:

| Service     | Endpoint                                   |
| ----------- | ------------------------------------------ |
| Blob        | `http://127.0.0.1:10000/devstoreaccount1`  |
| Queue       | `http://127.0.0.1:10001/devstoreaccount1`  |
| Table       | `http://127.0.0.1:10002/devstoreaccount1`  |
| Event Grid  | `http://127.0.0.1:10003`                   |
| Web PubSub  | `http://127.0.0.1:10004`                   |
| Monitor Logs| `http://127.0.0.1:10005`                   |
| Entra ID    | `http://127.0.0.1:10006` (HTTPS with TLS)  |
| Resource Mgr| `http://127.0.0.1:10007` (HTTPS with TLS)  |
| Service Bus | `sb://127.0.0.1:5672` (AMQP)               |

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

## Documentation

Full documentation lives at **[localaz.dev](https://localaz.dev)**.
