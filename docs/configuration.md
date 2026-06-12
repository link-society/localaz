# Configuration

localaz is configured through command-line flags or environment variables.
Flags take precedence; environment variables are convenient inside the
container.

Each emulated service listens on its own port (mirroring Azurite's multi-port
layout), but they all run inside the single localaz process.

| Flag              | Environment variable      | Default  | Description                       |
| ----------------- | ------------------------- | -------- | --------------------------------- |
| `-addr`           | `LOCALAZ_BLOB_ADDR`       | `:10000` | Blob service listen address       |
| `-queue-addr`     | `LOCALAZ_QUEUE_ADDR`      | `:10001` | Queue service listen address      |
| `-table-addr`     | `LOCALAZ_TABLE_ADDR`      | `:10002` | Table service listen address      |
| `-eventgrid-addr` | `LOCALAZ_EVENTGRID_ADDR`  | `:10003` | Event Grid service listen address |
| `-webpubsub-addr` | `LOCALAZ_WEBPUBSUB_ADDR`  | `:10004` | Web PubSub service listen address |
| `-monitor-addr`   | `LOCALAZ_MONITOR_ADDR`    | `:10005` | Monitor Logs service listen address |
| `-aad-addr`       | `LOCALAZ_AAD_ADDR`        | `:10006` | Entra ID (AAD) service listen address |
| `-arm-addr`       | `LOCALAZ_ARM_ADDR`        | `:10007` | Resource Manager (ARM) service listen address |
| `-servicebus-addr`| `LOCALAZ_SERVICEBUS_ADDR` | `:5672`  | Service Bus AMQP listen address   |
| `-data`           | `LOCALAZ_DATA_DIR`        | `/data`  | Directory for persisted state     |

The blob flag is `-addr` (not `-blob-addr`) for compatibility with existing
Azurite tooling. Blob, Queue and Table use the same ports as Azurite's
`UseDevelopmentStorage=true` shorthand (`10000`/`10001`/`10002`); the pub/sub
services follow on `10003`/`10004`/`10005`, and the control plane (Entra ID and
Resource Manager) on `10006`/`10007`.

## Control plane (Entra ID + Resource Manager)

The AAD and ARM emulators let the Azure CLI and the Azure SDKs treat localaz as
a custom Azure cloud: register it once, `az login` with a service principal,
and data-plane commands (such as `az monitor log-analytics query`) are routed
to localaz. The control plane is in-memory and transient — a single fixed
subscription and tenant plus any resource groups created at runtime.

| Flag               | Environment variable      | Default     | Description                                       |
| ------------------ | ------------------------- | ----------- | ------------------------------------------------- |
| `-arm-cloud-name`  | `LOCALAZ_ARM_CLOUD_NAME`  | `localaz`   | Cloud name advertised by the ARM metadata document |
| `-advertise-host`  | `LOCALAZ_ADVERTISE_HOST`  | `127.0.0.1` | Host clients use to reach the control-plane services |
| `-tls-cert`        | `LOCALAZ_TLS_CERT`        | _(unset)_   | PEM certificate for the bearer/control-plane services |
| `-tls-key`         | `LOCALAZ_TLS_KEY`         | _(unset)_   | PEM private key for the bearer/control-plane services |
| `-tls-auto`        | `LOCALAZ_TLS_AUTO`        | _(off)_     | Generate a self-signed certificate at startup     |

TLS is required for the control plane: MSAL and azure-core refuse to send
bearer tokens over plain HTTP. Either supply a certificate (`-tls-cert` /
`-tls-key`) or let localaz generate a self-signed one with `-tls-auto`, which
writes `<data>/tls/localaz.crt` and `localaz.key`. When TLS is enabled the
Monitor, AAD and ARM services serve HTTPS.

The registered cloud's name **must equal** `-arm-cloud-name`: the CLI matches
the active cloud against the `name` field in the ARM metadata document when it
resolves data-plane hosts. Sign in with the ADFS authority (`--tenant adfs`),
which tells MSAL to skip public instance discovery and talk only to localaz:

```bash
export REQUESTS_CA_BUNDLE=<data>/tls/localaz.crt
export SSL_CERT_FILE=<data>/tls/localaz.crt
export ARM_CLOUD_METADATA_URL=https://127.0.0.1:10007/metadata/endpoints

az cloud register -n localaz \
  --endpoint-active-directory https://127.0.0.1:10006/ \
  --endpoint-resource-manager https://127.0.0.1:10007/ \
  --endpoint-management https://127.0.0.1:10007/ \
  --endpoint-active-directory-resource-id https://127.0.0.1:10007/ \
  --suffix-storage-endpoint 127.0.0.1 --skip-endpoint-discovery
az cloud set -n localaz
az login --service-principal -u <app-id> -p <any-secret> --tenant adfs
```

The emulator does not validate the client id, secret or tokens, so the exact
credential values are not sensitive in this context.

## Examples

Run locally on a custom blob port with state in `./data`:

```bash
go run ./cmd/localaz -addr :11000 -data ./data
```

Run via Docker, exposing every service and a named volume for persistence:

```bash
docker run \
  -p 10000:10000 -p 10001:10001 -p 10002:10002 \
  -p 10003:10003 -p 10004:10004 -p 10005:10005 \
  -p 10006:10006 -p 10007:10007 -p 5672:5672 \
  -v localaz-data:/data localaz:dev
```

## Endpoint and credentials

| Setting          | Value                                            |
| ---------------- | ------------------------------------------------ |
| Blob endpoint    | `http://127.0.0.1:10000/devstoreaccount1`        |
| Queue endpoint   | `http://127.0.0.1:10001/devstoreaccount1`        |
| Table endpoint   | `http://127.0.0.1:10002/devstoreaccount1`        |
| Event Grid       | `http://127.0.0.1:10003`                         |
| Web PubSub       | `http://127.0.0.1:10004`                         |
| Monitor Logs     | `http://127.0.0.1:10005`                         |
| Entra ID (AAD)   | `http://127.0.0.1:10006` (HTTPS with TLS enabled) |
| Resource Manager | `http://127.0.0.1:10007` (HTTPS with TLS enabled) |
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
