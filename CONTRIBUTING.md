# Contributing to localaz

localaz is a local Azure emulator: a single Go process (shipped as one Docker
container) that speaks the native Azure service protocols so the Azure CLI and
the official SDKs work against it unchanged. Contributions are welcome —
bug fixes, new service coverage, better wire-format fidelity, and docs.

This guide covers how to get set up and the conventions we follow. For the full
architecture and a catalogue of gotchas, see [AGENTS.md](AGENTS.md).

## Prerequisites

- **Go 1.26** — the module targets this toolchain.
- **Task** — the task runner ([taskfile.dev](https://taskfile.dev)).
- **Docker** — to build and run the container image.
- **Azure CLI (`az`)** — only needed for the CLI end-to-end test suite.

## Build, run, and test

All workflows go through Task ([Taskfile.yml](Taskfile.yml)):

```bash
task build        # build the binary into ./out/localaz
task run          # build and run localaz locally
task test:unit    # go test ./...  (Go SDK suite, self-contained, no Docker)
task test:cli     # go test -tags cli ./test/cli/...  (requires the az CLI)
task lint         # gofmt check + go vet
task fmt          # format all Go code
task docker:up    # start the dev container (docker compose, --build, detached)
```

The Go SDK suite is self-contained and needs no Docker. The CLI suite shells
out to the real `az` CLI and is guarded by the `cli` build tag, so it never runs
under `task test:unit`. Run `task fmt` before committing and make sure
`task lint` is green.

## Code conventions

- **One structure and its methods per file.** Keep files focused and short; put
  shared helpers in a `helpers.go`-style file.
- **The protocol layer depends only on the store interface.** A `*server`
  package must depend on the `<svc>store.Store` interface only — never reach into
  a concrete backend (e.g. the filesystem-backed store).
- **Stay faithful to the Azure wire format.** XML element names, header casing,
  date formats, and JSON shapes must match what the SDKs and CLI expect. When in
  doubt, check what the Go SDK or `az` actually sends and parses rather than
  guessing.
- **Keep `task lint` (gofmt + `go vet`) green.** Formatting and vet must pass
  before a PR is mergeable.

## Commit convention

Commits follow [gitmoji](https://gitmoji.dev), matching the existing git
history. Prefix each message with the relevant emoji code, for example:

- `:memo:` — documentation
- `:sparkles:` — a new feature
- `:bug:` — a bug fix

## Pull requests

1. Branch off `main`.
2. Make your change, keeping diffs minimal and focused.
3. Open a pull request against `main`.
4. CI (the build workflow) must pass before the PR can be merged.

## Adding a new service

1. Define `internal/stores/<svc>store` with a `Store` interface and shared
   types.
2. Implement the protocol in `internal/servers/<svc>server`, depending only on
   that interface.
3. Mount it in `cmd/localaz` on the same process, matching Azure's endpoint
   conventions (HTTP services join the `services` slice; a non-HTTP service like
   Service Bus AMQP gets its own listener wired into the graceful-shutdown path).
4. Add a Go SDK suite in `test/sdk` and CLI coverage in `test/cli`.

Keep the single-container promise: anything a backend needs is embedded in the
same image.

---

For the full architecture and a catalogue of gotchas, see [AGENTS.md](AGENTS.md)
and the [architecture page](https://localaz.dev/architecture/) on the docs site.
