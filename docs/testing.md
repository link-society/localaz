# Testing

localaz has two complementary suites.

## Unit / integration suite (`test/sdk`)

Driven by the official **Azure Go SDKs** — `azblob` for Blob, `azqueue` for
Queue, `aztables` for Table, `aznamespaces` for Event Grid, `azwebpubsub` for
Web PubSub, `azservicebus` for Service Bus, the `monitor/ingestion/azlogs` /
`monitor/query/azlogs` clients for Monitor Logs, and `azidentity` plus
`armsubscriptions` / `armresources` for the Entra ID + Resource Manager control
plane. Each test starts an in-process emulator and exercises it through the
same client code a real Go application would use. This is fast, hermetic, and
needs no Docker or external tools.

```bash
task test:unit
# or
go test ./...
```

This is the suite that covers **every service**. The SDKs accept a custom
endpoint (or, for Service Bus, a `UseDevelopmentEmulator=true` connection
string), so they can be pointed at localaz and exercise the real wire protocol
end to end.

## End-to-end suite (`test/e2e`)

Driven by the real **Azure CLI** (`az`) as a subprocess. This is the strongest
interoperability signal: the CLI builds, signs, and parses requests exactly as
it would against Azure. The suite is guarded by the `e2e` build tag so it never
runs as part of `go test ./...`.

```bash
task test:e2e
# or
go test -tags e2e -count=1 -v ./test/e2e/...
```

Requirements: the `az` CLI must be installed. If it is not, the suite skips
itself cleanly.

By default the suite builds and launches localaz on a free local port, runs the
checks, and tears everything down. To run it against an already-running instance
(for example the Docker container), set `LOCALAZ_E2E_ENDPOINT`:

```bash
task docker:up
LOCALAZ_E2E_ENDPOINT=http://127.0.0.1:10000/devstoreaccount1 \
  go test -tags e2e -count=1 -v ./test/e2e/...
```

### Which services the E2E suite covers

The E2E suite exercises the storage data-plane services whose CLI commands can
be redirected to a local endpoint, and skips the rest:

- **Blob** — `az storage` supports a custom data-plane endpoint via
  `BlobEndpoint` in the connection string, so the CLI talks directly to localaz.
- **Queue** — `az storage queue` / `az storage message` honour `QueueEndpoint`
  in the connection string, so the CLI drives queue and message operations
  against localaz.
- **Table** — `az storage table` / `az storage entity` honour `TableEndpoint`
  in the connection string, so the CLI drives table and entity operations
  against localaz.
- **Event Grid** and **Service Bus** — `az eventgrid` and `az servicebus` are
  ARM *management-plane* commands only (they talk to `management.azure.com` to
  manage resources). There are no data-plane publish/send/receive commands to
  point anywhere.
- **Web PubSub** — `az webpubsub service` (extension) does have data-plane
  commands, but they resolve the target endpoint from ARM via `-n`/`-g`; there
  is no `--connection-string` or `--endpoint` override to redirect them to
  `127.0.0.1`.
- **Entra ID + Resource Manager (control plane)** and **Monitor Logs** — see
  the control-plane subsection below; the CLI signs in to the emulated AAD/ARM
  and runs `az monitor log-analytics query` against localaz end to end.

### Control plane: Entra ID + Resource Manager + Monitor query

`TestControlPlaneCLI` is the flagship end-to-end test. It launches localaz with
TLS (`-tls-auto`), registers it as a custom Azure cloud, signs in with a service
principal, manages a resource group, and runs `az monitor log-analytics query`
— all through the real `az` CLI:

1. Reserve free ports for Monitor/AAD/ARM and start localaz with `-tls-auto`
   and `-arm-cloud-name`; read the generated `<data>/tls/localaz.crt`.
2. Isolate the CLI with a throwaway `AZURE_CONFIG_DIR` (so the developer's real
   clouds, logins and active cloud are never touched), while pointing
   `AZURE_EXTENSION_DIR` at the real extension dir so the `log-analytics`
   extension is available. The test skips if that extension is not installed.
3. Set `REQUESTS_CA_BUNDLE` and `SSL_CERT_FILE` to the generated cert (the
   latter is needed for the CLI's metadata fetch), and
   `ARM_CLOUD_METADATA_URL` to the ARM `/metadata/endpoints` document.
4. Register and select the custom cloud (name must equal `-arm-cloud-name`),
   then `az login --service-principal --tenant adfs` (the ADFS authority makes
   MSAL skip public instance discovery).
5. Assert `az account show`, `az group create/list/delete`, seed log records
   via the Logs Ingestion API, and assert `az monitor log-analytics query`
   returns them.

This is exactly the flow a developer would use to point the Azure CLI at
localaz, proven against the real CLI on every run.

For the pub/sub services the **SDK suite** above is the equivalent end-to-end
signal: it drives the official clients against localaz over the real protocol,
which the CLI is simply unable to do locally. When launched by the suite, the
connection string includes the Blob, Queue and Table endpoints; in
`LOCALAZ_E2E_ENDPOINT` mode the queue and table tests additionally read
`LOCALAZ_E2E_QUEUE_ENDPOINT` and `LOCALAZ_E2E_TABLE_ENDPOINT` (skipping if
unset).

## Why Go for the E2E suite (instead of a shell script)

The end-to-end tests are written as Go tests that shell out to `az`, rather than
as a bash script. The trade-offs that motivated this:

- **Structured assertions and failure reporting** through the standard
  `testing` package, instead of hand-rolled `assert_eq` helpers.
- **Lifecycle management**: building, launching on a free port, readiness
  polling, and guaranteed teardown via `t.Cleanup` / `TestMain`.
- **One toolchain**: `go test` runs everything; no separate bash dependency, and
  it is portable across developer and CI environments.
- **Selective runs and parallelism** via the standard test runner and flags.

The CLI itself is still the system under test — we only use Go as the test
harness around it.

## Linting

```bash
task lint   # gofmt check + go vet
```
