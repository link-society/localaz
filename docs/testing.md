# Testing

localaz has two complementary suites.

## Unit / integration suite (`test/sdk`)

Driven by the official **Azure Go SDK** (`azblob`). Each test starts an
in-process emulator backed by a temporary directory and exercises it through the
same client code a real Go application would use. This is fast, hermetic, and
needs no Docker or external tools.

```bash
task test:unit
# or
go test ./...
```

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
