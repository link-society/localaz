# Configuration

localaz is configured through command-line flags or environment variables.
Flags take precedence; environment variables are convenient inside the
container.

| Flag    | Environment variable | Default  | Description                       |
| ------- | -------------------- | -------- | --------------------------------- |
| `-addr` | `LOCALAZ_BLOB_ADDR`  | `:10000` | Blob service listen address       |
| `-data` | `LOCALAZ_DATA_DIR`   | `/data`  | Directory for persisted state     |

## Examples

Run locally on a custom port with state in `./data`:

```bash
go run ./cmd/localaz -addr :11000 -data ./data
```

Run via Docker with a named volume for persistence:

```bash
docker run -p 10000:10000 -v localaz-data:/data localaz:dev
```

## Endpoint and credentials

| Setting          | Value                                            |
| ---------------- | ------------------------------------------------ |
| Blob endpoint    | `http://127.0.0.1:10000/devstoreaccount1`        |
| Account name     | `devstoreaccount1`                               |
| Account key      | Azurite's well-known development key             |

These match Azurite's defaults, so existing tooling and connection strings work
unchanged. Generate a ready-to-use connection string with:

```bash
eval "$(task env:conn-string)"
```

The emulator accepts the Shared Key `Authorization` header but does not verify
the signature, so the exact key value is not sensitive in this context.
