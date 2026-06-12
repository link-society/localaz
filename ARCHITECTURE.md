# Architecture

localaz is a single Go process that speaks the native Azure Storage REST
protocols. The Azure CLI and the official SDKs talk to it exactly as they would
talk to Azure — no shims, no client changes.

## Layout

```
cmd/localaz            entrypoint / process wiring
internal/blobserver    Azure Blob REST protocol (routing, XML, headers, errors)
internal/blobstore     storage abstraction (the Store interface)
  └── fsstore          filesystem-backed implementation (in-memory index + disk)
internal/azerr         faithful Azure error responses
test/sdk               integration tests via the Azure Go SDK
test/e2e               end-to-end tests via the Azure CLI
docker/                Dockerfile and dev compose stack
docs/                  configuration, supported APIs, testing
```

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

## Authentication

The emulator accepts the Shared Key `Authorization` header that the SDKs and CLI
send but does not verify the signature. This is standard behaviour for a local
development emulator (Azurite behaves the same way by default) and lets any
client work with the well-known development credentials. Signature verification
is on the roadmap as an opt-in mode.

## Adding a service

A new service (Queue, Table, Service Bus, …) follows the same pattern:

1. Define a storage interface for the service under `internal/<svc>store`.
2. Implement the REST protocol in `internal/<svc>server`, depending only on that
   interface.
3. Mount its handler in `cmd/localaz` on the same process (a different port or
   host prefix, matching Azure's endpoint conventions).
4. Add a Go SDK suite under `test/sdk` and CLI coverage under `test/e2e`.
