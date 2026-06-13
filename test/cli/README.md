# CLI suite (`test/cli`)

End-to-end tests that shell out to the real **Azure CLI** (`az`) as a
subprocess. Guarded by the `cli` build tag so they never run under
`go test ./...`; the suite skips cleanly when `az` is not installed.

```bash
task test:cli
# or
go test -tags cli -count=1 -v ./test/cli/...
```

By default the suite builds and launches localaz on free local ports and tears
it down afterwards. To target an already-running instance, set
`LOCALAZ_CLI_ENDPOINT` (and optionally `LOCALAZ_CLI_QUEUE_ENDPOINT` /
`LOCALAZ_CLI_TABLE_ENDPOINT`).

`helpers_cli_test.go` holds `TestMain` (lifecycle), the `az` invocation helpers,
and shared setup. `doc.go` is an untagged package-doc file so `go test ./...`
does not fail on build constraints.

## Scenarios

### Blob — `az storage` (`blob_cli_test.go`)

| Test | Scenario |
| ---- | -------- |
| `TestContainerLifecycle` | `container create` / `list` / `delete` |
| `TestBlobRoundTrip` | `blob upload` then `blob download`, asserting the payload |

### Queue — `az storage queue` / `message` (`queue_cli_test.go`)

| Test | Scenario |
| ---- | -------- |
| `TestQueueLifecycle` | `queue create` / `list` / `delete` |
| `TestQueueMessageRoundTrip` | `message put` then `message get` |

### Table — `az storage table` / `entity` (`table_cli_test.go`)

| Test | Scenario |
| ---- | -------- |
| `TestTableLifecycle` | `table create` / `list` / `delete` |
| `TestTableEntityRoundTrip` | `entity insert` then `entity query` / `show` |

### Control plane (`controlplane_cli_test.go`)

| Test | Scenario |
| ---- | -------- |
| `TestControlPlaneCLI` | The flagship end-to-end flow (see below) |

`TestControlPlaneCLI` launches localaz (which serves TLS by default, writing a
self-signed cert under `<data>/tls`), isolates a throwaway
`AZURE_CONFIG_DIR`, registers localaz as a custom cloud, signs in with a service
principal (`--tenant adfs`), and then:

- asserts `az account show` and `az group create/list/delete`;
- seeds log records via the Logs Ingestion API and runs a `MonitorLogs` table of
  subtests exercising the KQL subset through `az monitor log-analytics query`
  (string/numeric `where`, `and`/`or`, `project`, `sort`, `take`, `count`);
- runs a `ServiceBus` subtest driving the `Microsoft.ServiceBus` ARM resource
  provider through `az servicebus` (namespace, queue, topic, subscription).

It points `AZURE_EXTENSION_DIR` at the real extension dir for the
`log-analytics` extension and skips if that extension is absent.
