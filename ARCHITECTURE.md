# Architecture

localaz is a single Go process that speaks the native Azure service protocols.
The Azure CLI and the official SDKs talk to it exactly as they would talk to
Azure — no shims, no client changes. Each emulated service listens on its own
port (mirroring Azurite), but they all run inside the one process and ship in
the one container.

## Layout

```
cmd/localaz            entrypoint / process wiring (one listener per service)
internal/blobserver    Azure Blob REST protocol (routing, XML, headers, errors)
internal/blobstore     storage abstraction (the Store interface)
  └── fsstore          filesystem-backed implementation (in-memory index + disk)
internal/queueserver   Azure Queue REST protocol (XML messages, pop receipts)
internal/queuestore    queue + message store (in-memory index, JSON persistence)
internal/tableserver   Azure Table REST protocol (OData JSON, $filter, ETags)
internal/tablestore    entity store (in-memory index, JSON persistence)
internal/egserver      Event Grid REST protocol (namespace topics, pull delivery)
internal/egstore       in-memory Event Grid pub/sub state
internal/wpsserver     Web PubSub (REST + WebSocket json.webpubsub.azure.v1)
internal/sbserver      Service Bus AMQP 1.0 protocol (hand-rolled framing/codec)
internal/sbstore       in-memory Service Bus broker (queues + topic fan-out)
internal/monitorserver Monitor Logs REST protocol (ingestion + KQL-subset query)
internal/monitorstore  in-memory Monitor Logs tables
internal/aadserver     Entra ID (AAD): OIDC discovery, JWKS, RS256 JWT tokens
internal/armserver     Resource Manager: cloud metadata, subscriptions, groups
internal/armstore      in-memory ARM state (one subscription/tenant + groups)
internal/devcert       self-signed TLS material for the control plane
internal/azerr         faithful Azure error responses
test/sdk               integration tests via the Azure Go SDKs
test/e2e               end-to-end tests via the Azure CLI
docker/                Dockerfile and dev compose stack
docs/                  configuration, supported APIs, testing
```

## Services and ports

| Service     | Port    | Protocol          | Packages                       |
| ----------- | ------- | ----------------- | ------------------------------ |
| Blob        | `10000` | HTTP/REST         | `blobserver` + `blobstore`     |
| Queue       | `10001` | HTTP/REST         | `queueserver` + `queuestore`   |
| Table       | `10002` | HTTP/REST (OData) | `tableserver` + `tablestore`   |
| Event Grid  | `10003` | HTTP/REST         | `egserver` + `egstore`         |
| Web PubSub  | `10004` | HTTP + WebSocket  | `wpsserver`                    |
| Monitor Logs| `10005` | HTTP/REST         | `monitorserver` + `monitorstore` |
| Entra ID    | `10006` | HTTP/REST (HTTPS) | `aadserver` + `devcert`        |
| Resource Mgr| `10007` | HTTP/REST (HTTPS) | `armserver` + `armstore`       |
| Service Bus | `5672`  | AMQP 1.0 over TCP | `sbserver` + `sbstore`         |

Blob, Queue and Table share Azurite's `UseDevelopmentStorage=true` ports
(`10000`/`10001`/`10002`); the pub/sub services, which have no Azurite
convention, follow on `10003`/`10004`/`10005`, and the Entra ID + Resource
Manager control plane on `10006`/`10007`. The control-plane ports serve HTTPS
when TLS is enabled (`-tls-auto`, or `-tls-cert`/`-tls-key`), because MSAL and
azure-core refuse to send bearer tokens over plain HTTP.

`cmd/localaz` starts each HTTP service on its own `http.Server` goroutine and
starts Service Bus on a raw `net.Listen` accept loop (AMQP is not HTTP), then
shuts them all down together on signal.

## Layering

```mermaid
flowchart LR
    cli[Azure CLI] -->|REST| server
    sdk[Azure SDKs] -->|REST| server
    subgraph localaz process
      server[blobserver<br/>REST protocol] --> store[(blobstore.Store)]
      store --> fsstore[fsstore<br/>filesystem backend]
    end
    fsstore --> disk[(/data)]
```

The HTTP layer (`internal/blobserver`) depends **only** on the
`blobstore.Store` interface. It knows nothing about how bytes are stored. This
boundary is the key design decision:

- The protocol code can be tested and reasoned about independently.
- A different backend (PostgreSQL for metadata, object storage or Redis for
  payloads) can be dropped in by implementing `Store`, without touching a single
  line of protocol code.
- Whatever a backend needs would be **embedded inside the same container**, so
  the user-facing promise — run one container — never changes.

## The storage backend

`fsstore` keeps an in-memory index of accounts → containers → blobs for fast
listing and lookups, and writes every mutation through to disk. On startup it
rebuilds the index from disk, so a mounted `/data` volume preserves state across
restarts. Blob payloads live on disk and are read on demand.

On-disk layout:

```
<root>/<account>/<container>/_container.json    container metadata
<root>/<account>/<container>/data/<key>         blob payload
<root>/<account>/<container>/meta/<key>.json    blob metadata
<root>/<account>/<container>/blocks/<key>/<id>  staged, uncommitted block
```

where `<key>` is the URL-safe base64 encoding of the blob name (blob names may
contain `/`, which is not filesystem-safe).

Queue and Table use the same in-memory-index-plus-write-through approach, but
their state is small and structured, so each persists a single JSON document
(`<root>/queue/queues.json`, `<root>/table/tables.json`) that is rewritten
atomically on every mutation and reloaded on startup.

## Authentication

The emulator accepts the Shared Key `Authorization` header that the SDKs and CLI
send but does not verify the signature. This is standard behaviour for a local
development emulator (Azurite behaves the same way by default) and lets any
client work with the well-known development credentials. Signature verification
is on the roadmap as an opt-in mode.

## Adding a service

A new service follows the same pattern Queue and Table did:

1. Define a storage interface for the service under `internal/<svc>store`.
2. Implement the protocol in `internal/<svc>server`, depending only on that
   interface.
3. Mount it in `cmd/localaz` on the same process (its own port, matching Azure's
   endpoint conventions): HTTP services join the `services` slice; a raw-TCP
   service (like Service Bus AMQP) gets its own accept loop.
4. Add a Go SDK suite under `test/sdk` and CLI coverage under `test/e2e`.

## The pub/sub services

Event Grid, Web PubSub, Service Bus, and Monitor Logs keep their state in
memory — pub/sub and log-ingestion traffic is transient, so there is no disk
format to preserve. Service Bus is the
one service that does not speak HTTP: `internal/sbserver` implements just enough
of AMQP 1.0 (protocol headers, SASL ANONYMOUS, the open/begin/attach handshake,
flow-controlled transfers, dispositions, and CBS auth) for the official
`azservicebus` SDK to work unchanged when it connects with
`UseDevelopmentEmulator=true`. Message bodies are relayed verbatim, so only the
performative and CBS surface is decoded.

## The control plane (Entra ID + Resource Manager)

`aadserver` and `armserver` let the Azure CLI and SDKs treat localaz as a
custom Azure cloud. `aadserver` publishes an OIDC discovery document and JWKS,
and mints hand-rolled RS256 JWTs (`crypto/rsa`, no third-party dependency) that
verify against the published keys. `armserver` serves the cloud metadata
document (which advertises the login, resource-manager and Log Analytics
endpoints), a single fixed subscription and tenant, and runtime resource
groups via `armstore`. Both are in-memory and transient, like the pub/sub
services.

The flow a client follows: register localaz as a cloud whose `name` equals
`-arm-cloud-name`, `az login` against the emulated AAD (using the ADFS
authority so MSAL skips public instance discovery), then run data-plane
commands — the CLI resolves their host from the ARM cloud metadata and reaches
localaz. `devcert` generates the self-signed TLS material this requires.
