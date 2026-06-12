# Test suites

localaz has two complementary suites that together prove the emulator speaks the
native Azure protocols.

| Suite | Driver | Build tag | Needs | Run |
| ----- | ------ | --------- | ----- | --- |
| [`sdk/`](sdk/) | Official **Azure Go SDKs** | _(none)_ | nothing external | `task test:unit` / `go test ./...` |
| [`cli/`](cli/) | Real **Azure CLI** (`az`) subprocess | `cli` | `az` installed | `task test:cli` / `go test -tags cli ./test/cli/...` |

## How they relate

- **`sdk/` is the broad suite.** It starts an in-process emulator and exercises
  **every service** through the same client code a real Go application would
  use. It is fast, hermetic, and the canonical coverage for the pub/sub services
  (Event Grid, Web PubSub, Service Bus) and Monitor Logs, which have no CLI
  data-plane surface.
- **`cli/` is the interoperability signal.** It shells out to the real `az` CLI,
  which builds, signs, and parses requests exactly as it would against Azure.
  It is guarded by the `cli` build tag so it never runs under `go test ./...`,
  and skips itself cleanly when `az` is absent.

The `cli` suite covers the storage data-plane services that accept a custom
endpoint (Blob, Queue, Table) plus the control-plane flow (register localaz as a
cloud, `az login`, `az group`, `az monitor log-analytics query`, `az servicebus`).
Services the CLI cannot redirect to a local endpoint stay SDK-only.

## Why Go for the CLI suite

The CLI tests are Go tests that shell out to `az`, not a bash script — for
structured assertions via the `testing` package, robust lifecycle management
(build, free-port launch, readiness polling, guaranteed teardown), a single
toolchain (`go test`), and selective/parallel runs. The CLI itself is still the
system under test.

## Linting

```bash
task lint   # gofmt check + go vet
```

Each suite has its own `README.md` listing the concrete scenarios:
[`sdk/README.md`](sdk/README.md) and [`cli/README.md`](cli/README.md).
