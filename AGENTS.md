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
`docs/` Hugo site (published at [localaz.dev](https://localaz.dev)); see
[`README.md`](README.md) for a usage overview, [`CONTRIBUTING.md`](CONTRIBUTING.md)
for the contributor workflow, and [`CHANGELOG.md`](CHANGELOG.md) for the release
history.

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
internal/utils/atomicfile      crash-safe file writes (temp + fsync + rename + dir fsync)
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

The blob flag is still `-addr` (not `-blob-addr`) to match the well-known Azure
development-storage convention; do not rename it. Blob, Queue and Table
deliberately occupy the standard development-storage ports
(`10000`/`10001`/`10002`, account `devstoreaccount1`), which is why the
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
  are decoded. The hand-rolled decoder bounds list/map element counts against
  the remaining buffer, and `readFrame` rejects frames larger than the
  advertised 64 KiB max-frame-size (`maxFrameSize` in `frame.go`, the single
  source of truth shared with `conn.onOpen`); otherwise a ~12-byte crafted frame
  could trigger an unauthenticated multi-GB allocation / OOM. **Sender credit is
  replenished** as transfers are consumed: on attach the server grants
  `senderInitialCredit` (`link.go`), and `onTransfer` re-grants a flow every
  `senderCreditReplenishThreshold` transfers with `delivery-count` = received
  count and `link-credit` = `senderInitialCredit`, so the go-amqp client's window
  returns to its initial size and never stalls after the first window. **Multi-frame
  transfers are reassembled** by `(channel, handle)`: `onTransfer` buffers the
  partial body while the transfer `more` flag (performative field 5) is true and
  only relays the message to the broker and settles once, on the final frame
  (`more` false/absent) — otherwise a body split across frames (large messages or
  a small max-frame-size) would be delivered as several corrupted fragments.
- **Web PubSub connections never block the hub.** Each `wsConn` owns a buffered
  outbound channel drained by a dedicated writer goroutine; `send` is a
  non-blocking enqueue (`select`/`default`). The hub holds only an `RLock` while
  fanning out, so a slow/stalled client can never stall a broadcast or block
  add/remove/join/leave. If the outbound buffer fills, that connection is
  dropped (`closeNow`) rather than slowing the producer. Inbound frames are
  size-limited via `SetReadLimit` (1 MiB). The socket is accessed through the
  `wsSocket` interface so it can be stubbed in tests.
- **Pub/sub state is in-memory.** Event Grid, Web PubSub, Service Bus, and
  Monitor Logs do not persist — their traffic is transient, so there is no
  `/data` format for them. The AAD/ARM control plane is in-memory too (one
  fixed subscription/tenant plus runtime resource groups).
- **Event Grid lock tokens expire.** A `Receive` (`egstore`) locks each event
  with a deadline of `now + lockDuration` (default 5 minutes; clock is the
  injectable `Store.now`). Locks are swept lazily on the *next* `Receive` of
  that subscription — no background goroutine, mirroring
  `queuestore.evictExpired`-on-lookup — so an event left unacknowledged by a
  disconnected consumer is returned to `available` and redelivered with a
  higher `DeliveryCount`. `RenewLocks` actually extends the deadline (and still
  fails unknown tokens); it is no longer a no-op.
- **Monitor Logs is two data planes on one port.** Ingestion
  (`POST /dataCollectionRules/{rule}/streams/{stream}`, returns `204`) and the
  Log Analytics query (`POST /v1/workspaces/{id}/query`, returns `200`) share
  port `10005`, routed by path prefix. The stream name is the destination table
  (a leading `Custom-` is stripped). Both SDKs send bearer tokens, which azcore
  refuses over plain HTTP, so the SDK tests use a TLS `httptest` server + a fake
  `TokenCredential`.
- **Monitor ingestion bodies are size-bounded.** Both the raw upload and the
  gunzipped stream are capped (`maxIngestBytes` = 64 MiB raw via
  `http.MaxBytesReader`, `maxDecompressedBytes` = 256 MiB after inflation via an
  `io.LimitReader`) so an oversized POST or a gzip bomb cannot expand unbounded
  in memory. An over-limit body answers `413 Request Entity Too Large`. The caps
  are package `var`s only so tests can lower them.
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
  supported — no OData functions, typed literals, or continuation tokens. A
  comparison against an absent property, or a cross-type comparison (the value's
  type is not comparable to the literal's type), evaluates to false for *all*
  operators including `ne`: a `$filter` selects only entities that have the
  property and satisfy the comparison, so `Status ne 'active'` does not match an
  entity that has no `Status` at all. Note that Azure itself is inconsistent/buggy
  on null/missing-property filters, so this is a deliberate, documented choice
  rather than verified Azure parity. The recursive-descent
  parser caps parenthesis nesting at `maxFilterDepth` (64) so attacker-supplied
  deeply nested `(` cannot overflow the goroutine stack; input past the limit
  returns `errFilter`. The `$filter` and KQL parsers now have direct in-package
  unit tests (`internal/servers/tableserver/filter_*_test.go` and
  `internal/servers/monitorserver/{kql,predicate}_test.go`), driven through their
  public entry points (`parseFilter`, `evalKQL`).
- **Table `$top` is enforced before appending.** `$top=N` returns at most `N`
  entities, and `$top=0` returns none. The limit check in `listEntities` runs
  before the entity is added to the result, so it is not off by one.
- **Table entity ETag uses the fixed seven-digit datetime form.** The weak
  `odata.etag` (built by `etagFor` in `tablestore/store.go`) embeds a
  `datetime'…'` literal formatted with `.0000000` — not Go's `.9…` layout —
  so trailing zeros are preserved and the ETag's datetime exactly matches the
  rendered `Timestamp` representation.
- **Queue `numofmessages` is clamped to 1–32.** Get Messages floors the value at
  1 and rejects anything above Azure's maximum of 32 with `400`
  `OutOfRangeQueryParameterValue` (`azerr.CodeOutOfRangeQueryParam`), matching
  the real service.
- **Docker `/data` permissions.** The runtime image is distroless `nonroot`
  (uid 65532). The Dockerfile creates `/data` in the build stage and copies it
  with `--chown=65532:65532` so the non-root user can write to it.
- **Persistence (Blob/Queue/Table).** State lives under `/data`; mount a volume
  to keep it across restarts. `fsstore` keeps only blob *metadata* in memory and
  rebuilds that index from disk on startup; `queuestore` and `tablestore` each
  load and rewrite a single JSON document (`<root>/queue/queues.json`,
  `<root>/table/tables.json`). Every on-disk write — the queue/table JSON
  snapshots plus the blob meta and container meta files — goes through
  `internal/utils/atomicfile.Write` (temp file → fsync → rename → parent-dir
  fsync), and the streamed blob data file is written the same crash-safe way
  (`streamToFile`/`assembleBlocks` fsync the temp file before renaming and fsync
  the parent dir after). A crash therefore leaves either the old file or the
  fully written new one, never a truncated file or blob data without its meta.
  The `persistLocked` helpers keep their void signature and treat write errors as
  best-effort; durability is the guarantee.
- **Blob data is streamed, never buffered.** The `blobstore.Store` interface
  uses `io.Reader`/`io.ReadCloser` for blob bodies, not `[]byte`, so large or
  concurrent blobs do not OOM the server. `PutBlob` streams the request body to
  a temp file (computing MD5 + byte count via `io.MultiWriter`) then renames it
  into place; `GetBlob` opens the data file and returns it as an `io.ReadCloser`
  the caller (`blobserver`) `io.Copy`s to the response and then closes.
  `StageBlock` writes each staged block to its own file under
  `<container>/blocks/<key>/<blockID-key>` (no in-memory block map);
  `CommitBlockList` assembles the blob by `io.Copy`-ing those block files in
  order into a temp file then renaming — it never concatenates in memory. The
  single-shot Put Blob path records the payload MD5 so Get/Head echo it; the
  Put Block List path does not set one (unchanged wire behavior). The
  `io.LimitReader(r.Body, 5 GiB)` cap is kept as defense-in-depth around the
  reader handed to the store.
- **Blob name encoding.** Blob names may contain `/`. On disk they are stored
  under a URL-safe base64 key; do not assume a 1:1 path mapping. Staged block
  ids are encoded the same way.
- **Listener failures trigger graceful shutdown, not `os.Exit`.** `run` in
  `cmd/localaz/main.go` binds the AMQP `net.Listener` *before* starting any
  serve goroutine and the serve goroutines report a non-graceful
  `ListenAndServe`/`Serve` error on a buffered error channel instead of calling
  `os.Exit` inline. `run` selects on both `ctx.Done()` and that channel, then
  runs the single graceful-shutdown loop (`Shutdown` each `http.Server` with the
  10s timeout, close the AMQP listener, `wg.Wait`). It **returns an error** so a
  late bind failure on one listener drains every other listener; only `main`
  calls `os.Exit(1)` (on a non-nil return). Keep new listeners on this path —
  never `os.Exit` from inside a serve goroutine.
- **List Blobs is paginated.** The handler honors the `maxresults` (int) and
  `marker` query params and returns one page at a time, matching Azure instead of
  dumping the whole container in one response. `maxresults` is clamped to
  `1..5000` (the Azure default/cap; `<=0` means 5000). The `marker` is an opaque
  base64url continuation token naming the entry to resume AFTER (a malformed
  marker is treated as empty / start-from-the-beginning); the response echoes the
  REQUEST marker in `<Marker>` and returns the next page's token in
  `<NextMarker>` (empty when the listing is exhausted). Pages are emitted in
  lexicographic blob-name order, and a delimiter-collapsed virtual directory
  (`<BlobPrefix>`) counts toward `maxresults` the same as a blob. The
  `blobstore.Store` interface signature is
  `ListBlobs(account, container, prefix, delimiter string, maxResults int, marker string) (blobs []BlobInfo, prefixes []string, nextMarker string, err error)`.
- **List Containers pagination.** List Containers honors `maxresults` (default
  and cap 5000; `<=0` clamps to 5000) and an opaque `marker` continuation token,
  returning a `NextMarker` when more containers remain (empty on the last page).
  `fsstore` sorts containers lexicographically by name, treats `marker` as the
  base64url (`base64.RawURLEncoding`) of the container name to resume *after*
  (a malformed marker decodes to empty, restarting from the beginning), and
  emits `NextMarker` as the base64url token for the next page. The
  `blobstore.Store` interface reflects this:
  `ListContainers(account, prefix string, maxResults int, marker string) (containers []ContainerInfo, nextMarker string, err error)`.
  The `EnumerationResults` XML echoes the request `Marker`/`MaxResults` and
  renders the returned `NextMarker`, matching Azure's element order.
- **Access-log middleware recovers panics.** `logRequests` (`cmd/localaz`) wraps
  every service handler. It now recovers a panic from the handler, logs it via
  slog at Error level (method, path, recovered value), and — if the handler had
  not written a header yet — returns a `500`. It wraps the `ResponseWriter` in a
  small `statusRecorder` to track whether a header was written and to log the
  status code. So a single handler panic can no longer drop the request without
  a response.

## Control plane (AAD + ARM) gotchas

- **TLS is mandatory for the control plane.** MSAL and azure-core refuse to
  send bearer tokens over plain HTTP, so AAD/ARM/Monitor must serve HTTPS. Use
  `-tls-auto` (writes `<data>/tls/localaz.{crt,key}`) or supply
  `-tls-cert`/`-tls-key`. `http://` authorities are rejected outright by MSAL.
- **Auto cert SANs include `-advertise-host`.** `devcert.Generate(hosts...)`
  starts from the loopback defaults (`localhost`, `127.0.0.1`, `::1`) and adds
  the configured `-advertise-host`, so a hostname or non-loopback IP advertised
  by ARM still validates. The generated `localaz.crt` is written `0600` (like
  the key). `IsCA` stays `true` on purpose: clients trust this cert explicitly
  via `REQUESTS_CA_BUNDLE`/`SSL_CERT_FILE`, and flipping the CA basic
  constraint would break the documented az/MSAL trust flow.
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
- **The RS256 signing key is persisted** to `<data>/aad/signing-key.pem`
  (PKCS#8 PEM, `0o600`). `aadserver.New(dataDir)` loads it when present and
  generates+writes it otherwise; the kid is derived from the modulus, so it is
  **stable across restarts** (it used to be regenerated per process, which
  broke tokens minted before a restart and stale cached JWKS). With an empty
  data dir the key is generated in-memory only.
- **User flows carry a user-derived `sub`/`oid`.** When the token request
  supplies a `username` (ROPC/password, auth code), the emitted access and id
  tokens set `sub` to the username and `oid` to a stable, GUID-shaped value
  derived from `sha256(username)` — deterministic across calls and never the
  client id, so an OIDC consumer keying on `sub`/`oid` distinguishes the user
  from the app. `appid` is always the client id. `client_credentials` (no
  username) keeps `sub`/`oid` set to the app/client id.
- **ARM resource providers are generic.** `armserver/provider.go` stores any
  `.../providers/{ns}/{type}/{name}` body in `armstore` and echoes it back with
  a terminal `provisioningState=Succeeded`, which is enough for the CLI's
  create/show/list/delete to run (no LRO polling). `Microsoft.ServiceBus` is
  exercised end to end by `az servicebus`; the broker (`sbstore`) is separate
  and auto-creates entities on first data-plane use, so the RP does not
  pre-provision them. To learn the exact paths a new CLI command needs, run the
  emulator with request logging and watch the `service=arm` log lines.
- **ARM provider state is bounded.** Provider PUT bodies are size-limited via
  `http.MaxBytesReader` (1 MiB); an oversized body is rejected with 400 instead
  of being decoded. `armstore` caps distinct generic resources at
  `maxResources` (10000): `PutResource` returns `false` for a new ID once the
  cap is hit (replacing an existing ID is always allowed), and the handler
  surfaces that as 409. This keeps a PUT loop with distinct IDs from exhausting
  the in-memory store.
- **Resource DELETE returns 204 when absent.** `handleProviderItem` mirrors the
  resource-group handler: DELETE of an existing resource ⇒ 200, DELETE of an
  absent one ⇒ 204 (it no longer always returns 200).
- **The cli control-plane test isolates `AZURE_CONFIG_DIR`** to a temp dir so it
  never touches the developer's real clouds/logins, but points
  `AZURE_EXTENSION_DIR` at the real extension dir for the `log-analytics`
  command (skips if that extension is absent).
