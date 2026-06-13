# localaz

A local **Azure emulator**, in the spirit of LocalStack (AWS) and localgcp
(GCP). You run a single Docker container and point the **Azure CLI** or any
**Azure SDK** at it — no code changes required.

It currently emulates parts of the following services:

 - Blob Storage
 - Queue Storage
 - Table Storage
 - Key Vault
 - Event Grid
 - Web PubSub
 - Service Bus
 - Monitor Logs
 - Entra ID (AAD) + Resource Manager (ARM) control plane

## Quick start

Pull the Docker image:

```bash
docker pull linksociety/localaz:latest
```

The services are then available at:

| Service          | Endpoint                                   |
| ---------------- | ------------------------------------------ |
| Blob Storage     | `https://127.0.0.1:10000/devstoreaccount1` |
| Queue Storage    | `https://127.0.0.1:10001/devstoreaccount1` |
| Table Storage    | `https://127.0.0.1:10002/devstoreaccount1` |
| Key Vault        | `https://127.0.0.1:10008`                  |
| Event Grid       | `https://127.0.0.1:10003`                  |
| Web PubSub       | `https://127.0.0.1:10004`                  |
| Service Bus      | `sb://127.0.0.1:5672` (AMQP)               |
| Monitor Logs     | `https://127.0.0.1:10005`                  |
| Entra ID         | `https://127.0.0.1:10006`                  |
| Resource Manager | `https://127.0.0.1:10007`                  |

### Usage with the Azure CLI

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

### Usage with the Go SDK

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

## Documentation

For more information about how to use each service, you can read the
documentation at: **[localaz.dev](https://localaz.dev)**.

## License

This project is released under the terms of the [Unlicense](./LICENSE.txt)
license.
