---
title: "Get Started"
weight: 2
---

## Run localaz

You'll need to port-map all the service ports from the Docker container:

```bash
docker run --name localaz \
  -p 10000:10000 -p 10001:10001 -p 10002:10002 \
  -p 10003:10003 -p 10004:10004 -p 10005:10005 \
  -p 10006:10006 -p 10007:10007 -p 5672:5672 \
  -v ./localaz-data:/data \
  linksociety/localaz:latest
```

All persisted data — including the auto-generated TLS certificate at
`./localaz-data/tls/localaz.crt` — will live in the `./localaz-data` folder.

## Service endpoints

Once running, the services are available at:

| Service | Endpoint |
| --- | --- |
| [Blob Storage](/services/blob) | `https://127.0.0.1:10000/devstoreaccount1` |
| [Queue Storage](/services/queue) | `https://127.0.0.1:10001/devstoreaccount1` |
| [Table Storage](/services/table) | `https://127.0.0.1:10002/devstoreaccount1` |
| [Event Grid](/services/event-grid) | `https://127.0.0.1:10003` |
| [Web PubSub](/services/web-pubsub) | `https://127.0.0.1:10004` |
| [Service Bus](/services/service-bus) | `sb://127.0.0.1:5672` |
| [Monitor Logs](/services/monitor-logs) | `https://127.0.0.1:10005` |
| [Entra ID](/services/control-plane) | `https://127.0.0.1:10006` |
| [Resource Manager](/services/control-plane) | `https://127.0.0.1:10007` |

## Usage

### Azure CLI

Trust localaz's certificate:

```bash
export SSL_CERT_FILE=./localaz-data/tls/localaz.crt
export REQUESTS_CA_BUNDLE="$SSL_CERT_FILE"
```

Generate a random storage key:

```bash
export AZURE_STORAGE_KEY="$(openssl rand -base64 64 | tr -d '\n')"
```

Configure the connection string and run the CLI:

```bash
export AZURE_STORAGE_CONNECTION_STRING="DefaultEndpointsProtocol=https;AccountName=devstoreaccount1;AccountKey=${AZURE_STORAGE_KEY};BlobEndpoint=https://127.0.0.1:10000/devstoreaccount1;QueueEndpoint=https://127.0.0.1:10001/devstoreaccount1;TableEndpoint=https://127.0.0.1:10002/devstoreaccount1;"

az storage container create --name demo
az storage blob upload --container-name demo --name hello.txt --file ./hello.txt
az storage blob list --container-name demo -o table

az storage queue create --name work-items
az storage message put --queue-name work-items --content "hello"

az storage table create --name people
az storage entity insert --table-name people --entity PartitionKey=team RowKey=alice Name=Alice
```

### Go SDK

Trust localaz's certificate and supply a Shared Key before running:

```bash
export SSL_CERT_FILE=./localaz-data/tls/localaz.crt
```

Generate a random storage key:

```bash
export AZURE_STORAGE_KEY="$(openssl rand -base64 64 | tr -d '\n')"
```

```go
connStr := "DefaultEndpointsProtocol=https;AccountName=devstoreaccount1;" +
	"AccountKey=" + os.Getenv("AZURE_STORAGE_KEY") + ";" +
	"BlobEndpoint=https://127.0.0.1:10000/devstoreaccount1;"
client, _ := azblob.NewClientFromConnectionString(connStr, nil)
client.CreateContainer(ctx, "demo", nil)
client.UploadBuffer(ctx, "demo", "hello.txt", []byte("hi"), nil)
```

## Next steps

 - [Per-Service Documentation](/services)
 - [Configuration Reference](/configuration)
