# AGENTS.md

Guidance for AI agents and contributors working on localaz.

## Project overview

localaz is a local **Azure emulator**: a single Go process (shipped as one
Docker container) that speaks the native Azure service protocols so the Azure
CLI and the official SDKs work against it unchanged. It currently emulates Blob
Storage, Queue Storage, Table Storage, Event Grid, Web PubSub, Service Bus, and
Monitor Logs, plus an Entra ID (AAD) + Resource Manager (ARM) **control plane**
that lets the CLI/SDKs treat localaz as a custom Azure cloud (register, log in,
and route data-plane commands to localaz). Everything runs in the one process,
each on its own port. The full design and per-service reference live in the
`docs/` Hugo site.

- Module path: `localaz.dev`
- Go version: 1.26
- The **storage** paths (Blob, Queue, Table) are kept dependency-light; the
  `azblob`, `azqueue` and `aztables` SDKs are used only in the test suite. The
  **pub/sub** services (Event Grid, Web PubSub, Service Bus) are allowed
  third-party dependencies — notably `azservicebus` and `go-amqp` for the
  Service Bus AMQP transport.

## Repository layout

```
cmd/localaz            entrypoint / process wiring (one listener per service)
internal/servers/blobserver    Azure Blob REST protocol (routing, XML, headers, errors)
internal/servers/queueserver   Azure Queue REST protocol (XML messages, pop receipts)
internal/servers/tableserver   Azure Table REST protocol (OData JSON, $filter, ETags)
internal/servers/egserver      Event Grid REST protocol (namespace topics, pull delivery)
internal/servers/wpsserver     Web PubSub (REST + WebSocket)
internal/servers/sbserver      Service Bus AMQP 1.0 (hand-rolled framing/codec)
internal/servers/monitorserver Monitor Logs REST (ingestion + KQL-subset query)
internal/servers/aadserver     Entra ID (AAD): OIDC discovery, JWKS, RS256 JWT tokens
internal/servers/armserver     Resource Manager: cloud metadata, subscriptions, groups
internal/stores/blobstore      storage abstraction (the Store interface) + types
  └── fsstore                  filesystem-backed implementation
internal/stores/queuestore     in-memory queue/message state + JSON persistence
internal/stores/tablestore     in-memory entity state + JSON persistence
internal/stores/egstore        in-memory Event Grid pub/sub state
internal/stores/sbstore        in-memory Service Bus broker
internal/stores/monitorstore   in-memory Monitor Logs tables
internal/stores/armstore       in-memory ARM state (one subscription/tenant + groups + generic resources)
internal/utils/httpx           shared HTTP helpers
internal/utils/azwire          shared Azure wire-format helpers
internal/utils/azerr           faithful Azure error responses
internal/utils/devcert               self-signed TLS material for the control plane
test/sdk                       integration tests via the Azure Go SDKs (+ README.md)
test/cli                       end-to-end tests via the Azure CLI, build tag: cli (+ README.md)
test/README.md                 test-suite architecture overview
docker/                        localaz.dockerfile, docker-compose.dev.yml
docs/                          Hugo documentation site (content/, layouts/, static/)
```

## Services and ports

| Service     | Port    | Protocol          | Flag / env                                       |
| ----------- | ------- | ----------------- | ------------------------------------------------ |
| Blob        | `10000` | HTTP/REST         | `-addr` / `LOCALAZ_BLOB_ADDR`                    |
| Queue       | `10001` | HTTP/REST         | `-queue-addr` / `LOCALAZ_QUEUE_ADDR`             |
| Table       | `10002` | HTTP/REST (OData) | `-table-addr` / `LOCALAZ_TABLE_ADDR`             |
| Event Grid  | `10003` | HTTP/REST         | `-eventgrid-addr` / `LOCALAZ_EVENTGRID_ADDR`     |
| Web PubSub  | `10004` | HTTP + WebSocket  | `-webpubsub-addr` / `LOCALAZ_WEBPUBSUB_ADDR`     |
| Monitor Logs| `10005` | HTTP/REST         | `-monitor-addr` / `LOCALAZ_MONITOR_ADDR`         |
| Entra ID    | `10006` | HTTP/REST (HTTPS) | `-aad-addr` / `LOCALAZ_AAD_ADDR`                 |
| Resource Mgr| `10007` | HTTP/REST (HTTPS) | `-arm-addr` / `LOCALAZ_ARM_ADDR`                 |
| Service Bus | `5672`  | AMQP 1.0 over TCP | `-servicebus-addr` / `LOCALAZ_SERVICEBUS_ADDR`   |

The blob flag is still `-addr` (not `-blob-addr`) for back-compat with Azurite
tooling; do not rename it. Blob, Queue and Table deliberately occupy Azurite's
`UseDevelopmentStorage=true` ports (`10000`/`10001`/`10002`), which is why the
pub/sub services moved to `10003`/`10004`/`10005` and the control plane (AAD,
ARM) to `10006`/`10007`. The control-plane ports serve HTTPS when TLS is
enabled (`-tls-auto`, or `-tls-cert`/`-tls-key`).

## Conventions

- **One structure and its methods per file.** Keep files focused and short.
  Put shared helpers in a `helpers.go` (or similarly named) file.
  `stores/blobstore/fsstore` is the reference for this: `store.go`, `types.go`,
  `paths.go`, `helpers.go`, `persistence.go`, `containers.go`, `blobs.go`,
  `blocks.go`.
- **The protocol layer depends only on the store interface.** Never let a
  `*server` package reach into a concrete backend; depend on the
  `<svc>store.Store` type only.
- **Stay faithful to the Azure wire format.** XML element names, header casing,
  and date formats must match what the SDKs expect (e.g. `<Etag>` not `<ETag>`,
  `Last-Modified` as RFC1123 GMT). Blob and Queue speak XML; Table speaks OData
  **JSON** (responses and errors), with `Timestamp` as ISO 8601 with seven
  fractional digits and a weak `odata.etag`. When in doubt, check what the Go
  SDK or CLI actually sends/parses rather than guessing.
- Run `task fmt` and keep `task lint` (gofmt + `go vet`) clean.

## Build, run, and test

Use Task (see [Taskfile.yml](Taskfile.yml)):

```bash
task build        # build the binary
task run          # run locally
task test:unit    # go test ./...  (Go SDK suite, self-contained)
task test:cli     # go test -tags cli ./test/cli/...  (requires az)
task lint         # gofmt check + go vet
task docker:build # build the container image
task docker:up    # docker compose -f docker/docker-compose.dev.yml up --build
```

The CLI suite shells out to the real `az` CLI and is guarded by the `cli` build
tag so it never runs under `task test:unit`. `test/cli/doc.go` is an untagged
package doc file so `go test ./...` does not fail with "build constraints
exclude all Go files".

The documentation lives in `docs/` as a [Hugo](https://gohugo.io) site
(`content/` Markdown, `layouts/` templates, `static/` assets). Preview it with
`hugo server` from `docs/`; build with `hugo`. Architecture diagrams use
fenced ```mermaid``` code blocks rendered by a code-block render hook. The
`docs/public/` and `docs/resources/` build artifacts are git-ignored.

## Adding a new service

1. Define `internal/stores/<svc>store` with a `Store` interface and shared
   types.
2. Implement the protocol in `internal/servers/<svc>server`, depending only on
   that interface.
3. Mount it in `cmd/localaz` on the same process, matching Azure's endpoint
   conventions. HTTP services join the `services` slice; a non-HTTP service
   (like Service Bus AMQP) gets its own `net.Listen` accept loop and is wired
   into the same graceful-shutdown path.
4. Add a Go SDK suite in `test/sdk` and CLI coverage in `test/cli`.

Keep the single-container promise: anything a backend needs is embedded in the
same image.

## Gotchas

- **Auth is not verified.** The Shared Key header (and Service Bus CBS tokens)
  are accepted but not validated (standard for local emulators). Do not add
  verification without making it opt-in.
- **Service Bus is raw AMQP 1.0 over plain TCP.** `azservicebus` connects with
  `UseDevelopmentEmulator=true`, which disables TLS and uses SASL ANONYMOUS.
  `internal/sbserver` hand-rolls the framing/type codec — no Go AMQP *server*
  library exists. AMQP **handles are session-scoped**, so links must be keyed by
  `(channel, handle)`, not handle alone (the `$cbs`/`$management` links collide
  otherwise). Message bodies are relayed verbatim; only performatives and CBS
  are decoded.
- **Pub/sub state is in-memory.** Event Grid, Web PubSub, Service Bus, and
  Monitor Logs do not persist — their traffic is transient, so there is no
  `/data` format for them. The AAD/ARM control plane is in-memory too (one
  fixed subscription/tenant plus runtime resource groups).
- **Monitor Logs is two data planes on one port.** Ingestion
  (`POST /dataCollectionRules/{rule}/streams/{stream}`, returns `204`) and the
  Log Analytics query (`POST /v1/workspaces/{id}/query`, returns `200`) share
  port `10005`, routed by path prefix. The stream name is the destination table
  (a leading `Custom-` is stripped). Both SDKs send bearer tokens, which azcore
  refuses over plain HTTP, so the SDK tests use a TLS `httptest` server + a fake
  `TokenCredential`.
- **Monitor query is a documented KQL subset.** Only
  `where`/`project`/`sort by`/`take`(`limit`)/`count` over string/number/bool
  literals with `==`/`!=`/`<`/`<=`/`>`/`>=` and `and`/`or` are supported — no
  `summarize`, `join`, functions, parentheses, or timespan filtering.
- **Table merge arrives as HTTP `PATCH`.** The `aztables` SDK issues `PATCH`
  (not the `MERGE` verb) for merge updates; `internal/tableserver` accepts both.
  Upsert is a `PUT`/`PATCH` with no `If-Match`; `If-Match: *` requires the
  entity to exist, and a weak ETag enforces optimistic concurrency.
- **Table `$filter` is a documented subset.** Only `eq`/`ne`/`gt`/`ge`/`lt`/`le`
  over string/number/bool literals combined with `and`/`or` and parentheses are
  supported — no OData functions, typed literals, or continuation tokens.
- **Docker `/data` permissions.** The runtime image is distroless `nonroot`
  (uid 65532). The Dockerfile creates `/data` in the build stage and copies it
  with `--chown=65532:65532` so the non-root user can write to it.
- **Persistence (Blob/Queue/Table).** State lives under `/data`; mount a volume
  to keep it across restarts. `fsstore` rebuilds its in-memory blob index from
  disk on startup; `queuestore` and `tablestore` each load and atomically
  rewrite a single JSON document (`<root>/queue/queues.json`,
  `<root>/table/tables.json`).
- **Blob name encoding.** Blob names may contain `/`. On disk they are stored
  under a URL-safe base64 key; do not assume a 1:1 path mapping.

## Control plane (AAD + ARM) gotchas

- **TLS is mandatory for the control plane.** MSAL and azure-core refuse to
  send bearer tokens over plain HTTP, so AAD/ARM/Monitor must serve HTTPS. Use
  `-tls-auto` (writes `<data>/tls/localaz.{crt,key}`) or supply
  `-tls-cert`/`-tls-key`. `http://` authorities are rejected outright by MSAL.
- **Sign in with `--tenant adfs`.** The ADFS authority mode makes MSAL skip
  public `login.microsoftonline.com` instance discovery and talk only to the
  configured authority — essential for offline use.
- **CLI cert trust:** set BOTH `REQUESTS_CA_BUNDLE` and `SSL_CERT_FILE` to the
  cert. `AZURE_CLI_DISABLE_CONNECTION_VERIFICATION=1` covers MSAL but NOT the
  secondary OIDC/metadata fetches, so the CA-bundle env vars are the reliable
  path.
- **Registered cloud name must equal `-arm-cloud-name`.** The CLI matches the
  active cloud against the `name` field of the ARM `/metadata/endpoints`
  document when resolving data-plane hosts; a mismatch yields
  `CloudEndpointNotSetException` on `az monitor log-analytics query`.
- **Log Analytics host discovery quirk.** The `log-analytics` extension reads
  the query host from the metadata index `logAnalyticslogAnalyticsResourceId`
  (doubled prefix, verbatim) only via `ARM_CLOUD_METADATA_URL`; the named
  `log_analytics_resource_id` endpoint does NOT match. `armserver` emits that
  exact key.
- **JWTs are hand-rolled RS256** (crypto/rsa, no third-party dep) and verify
  against the published JWKS; tokens, client id and secret are accepted but
  never validated (opt-in only, like the storage Shared Key).
- **ARM resource providers are generic.** `armserver/provider.go` stores any
  `.../providers/{ns}/{type}/{name}` body in `armstore` and echoes it back with
  a terminal `provisioningState=Succeeded`, which is enough for the CLI's
  create/show/list/delete to run (no LRO polling). `Microsoft.ServiceBus` is
  exercised end to end by `az servicebus`; the broker (`sbstore`) is separate
  and auto-creates entities on first data-plane use, so the RP does not
  pre-provision them. To learn the exact paths a new CLI command needs, run the
  emulator with request logging and watch the `service=arm` log lines.
- **The cli control-plane test isolates `AZURE_CONFIG_DIR`** to a temp dir so it
  never touches the developer's real clouds/logins, but points
  `AZURE_EXTENSION_DIR` at the real extension dir for the `log-analytics`
  command (skips if that extension is absent).
