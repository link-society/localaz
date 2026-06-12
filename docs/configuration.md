# Configuration

localaz is configured through command-line flags or environment variables.
Flags take precedence; environment variables are convenient inside the
container.

Each emulated service listens on its own port (mirroring Azurite's multi-port
layout), but they all run inside the single localaz process.

| Flag              | Environment variable      | Default  | Description                       |
| ----------------- | ------------------------- | -------- | --------------------------------- |
| `-addr`           | `LOCALAZ_BLOB_ADDR`       | `:10000` | Blob service listen address       |
| `-eventgrid-addr` | `LOCALAZ_EVENTGRID_ADDR`  | `:10001` | Event Grid service listen address |
| `-webpubsub-addr` | `LOCALAZ_WEBPUBSUB_ADDR`  | `:10002` | Web PubSub service listen address |
| `-servicebus-addr`| `LOCALAZ_SERVICEBUS_ADDR` | `:5672`  | Service Bus AMQP listen address   |
| `-data`           | `LOCALAZ_DATA_DIR`        | `/data`  | Directory for persisted state     |

## Examples

Run locally on a custom blob port with state in `./data`:

```bash
go run ./cmd/localaz -addr :11000 -data ./data
```

Run via Docker, exposing every service and a named volume for persistence:

```bash
docker run \
  -p 10000:10000 -p 10001:10001 -p 10002:10002 -p 5672:5672 \
  -v localaz-data:/data localaz:dev
```

## Endpoint and credentials

| Setting          | Value                                            |
| ---------------- | ------------------------------------------------ |
| Blob endpoint    | `http://127.0.0.1:10000/devstoreaccount1`        |
| Event Grid       | `http://127.0.0.1:10001`                         |
| Web PubSub       | `http://127.0.0.1:10002`                         |
| Service Bus      | `sb://127.0.0.1:5672` (AMQP)                      |
| Account name     | `devstoreaccount1`                               |
| Account key      | Azurite's well-known development key             |

These match Azurite's defaults, so existing tooling and connection strings work
unchanged. Generate a ready-to-use connection string with:

```bash
eval "$(task env:conn-string)"
```

For Service Bus, point the official SDK at the emulator with the development
connection string (plain TCP, no TLS):

```text
Endpoint=sb://127.0.0.1:5672;SharedAccessKeyName=test;SharedAccessKey=test;UseDevelopmentEmulator=true
```

The emulator accepts the Shared Key `Authorization` header (and Service Bus CBS
tokens) but does not verify them, so the exact key value is not sensitive in
this context.
