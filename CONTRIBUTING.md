# Contributing to localaz

Thanks for your interest in improving **localaz**, a single-process, single-container
local **Azure emulator**. This guide covers how to set up your environment, the
conventions the codebase follows, and how to get a change merged.

For the deep architectural reference (per-service internals, gotchas, and the
"why" behind design decisions), read [`AGENTS.md`](AGENTS.md) — it is the source
of truth for contributors and AI agents alike.

## Table of contents

- [Prerequisites](#prerequisites)
- [Getting started](#getting-started)
- [Build, run, and test](#build-run-and-test)
- [Repository layout](#repository-layout)
- [Coding conventions](#coding-conventions)
- [Commit conventions](#commit-conventions)
- [Pull request process](#pull-request-process)
- [Adding a new service](#adding-a-new-service)
- [Documentation](#documentation)

## Prerequisites

- **Go 1.26+** — the module targets `go 1.26` (see [`go.mod`](go.mod)).
- **[Task](https://taskfile.dev)** — the task runner used for all common
  commands (see [`Taskfile.yml`](Taskfile.yml)). Optional; every task maps to a
  plain `go`/`docker` command you can run by hand.
- **Docker** — to build and run the container image.
- **[Azure CLI](https://learn.microsoft.com/cli/azure/install-azure-cli)** (`az`)
  — only required for the end-to-end CLI test suite (`task test:cli`).
- **[Hugo](https://gohugo.io)** — only required to preview the documentation site.

## Getting started

```bash
git clone https://github.com/link-society/localaz
cd localaz
task build      # compile the binary to ./out/localaz
task run        # build and run locally against ./data
```

Then point the Azure CLI or an SDK at the emulator — see the
[README](README.md#quick-start) for connection details.

## Build, run, and test

Everything is driven through [Task](https://taskfile.dev). Run `task` with no
arguments to list every available command.

| Command | What it does |
| ------- | ------------ |
| `task build` | Build the `localaz` binary into `./out`. |
| `task run` | Build and run locally (`-data ./data`). |
| `task test:unit` | Go SDK integration suite — `go test ./...`, self-contained, no Docker needed. |
| `task test:cli` | Azure CLI end-to-end suite — `go test -tags cli ./test/cli/...` (requires `az`). |
| `task test` | Run both suites. |
| `task lint` | `gofmt` check + `go vet ./...`. |
| `task fmt` | Format all Go code with `gofmt -w`. |
| `task docker:build` | Build the container image. |
| `task docker:up` / `task docker:down` | Start / stop the dev Compose stack. |
| `task docs:serve` | Preview the documentation site with live reload. |

The CLI suite shells out to the real `az` CLI and is guarded by the `cli` build
tag, so it never runs under `task test:unit`. `test/cli/doc.go` is an untagged
package-doc file so `go test ./...` does not fail with "build constraints
exclude all Go files".

Before opening a pull request, make sure both of these pass:

```bash
task lint
task test:unit
```

## Repository layout

```
cmd/localaz                    entrypoint / process wiring (one listener per service)
internal/servers/<svc>server   Azure wire protocol for a service (routing, codec, errors)
internal/stores/<svc>store     state + persistence for a service (behind a Store interface)
internal/utils/...             shared helpers (httpx, azwire, azerr, atomicfile, devcert)
test/sdk                       integration tests via the Azure Go SDKs
test/cli                       end-to-end tests via the Azure CLI (build tag: cli)
docker/                        localaz.dockerfile, docker-compose.dev.yml
docs/                          Hugo documentation site (content/, layouts/)
```

See [`AGENTS.md`](AGENTS.md#repository-layout) for the full annotated tree.

## Coding conventions

These are the load-bearing rules; [`AGENTS.md`](AGENTS.md#conventions) has the
complete list.

- **One structure and its methods per file.** Keep files focused and short; put
  shared helpers in a `helpers.go`.
- **The protocol layer depends only on the store interface.** A `<svc>server`
  package must never reach into a concrete backend — depend on the
  `<svc>store.Store` type only.
- **Stay faithful to the Azure wire format.** XML element names, header casing,
  and date formats must match what the SDKs expect (e.g. `<Etag>` not `<ETag>`,
  `Last-Modified` as RFC1123 GMT). When in doubt, check what the Go SDK or CLI
  actually sends/parses rather than guessing.
- **Keep storage dependency-light.** Blob, Queue and Table avoid third-party SDK
  dependencies in non-test code; the pub/sub services may use `azservicebus` /
  `go-amqp`.
- **Run `task fmt` and keep `task lint` clean** (`gofmt` + `go vet`).
- **Make the smallest change that solves the problem.** Prefer focused edits over
  broad refactors.

## Commit conventions

Commits follow **[gitmoji](https://gitmoji.dev)**: a leading emoji code that
classifies the change, then a short imperative summary.

```
:sparkles: add Azure Monitor Logs emulator
:bug: expire Event Grid lock tokens and redeliver unacknowledged events
:lock: bound Monitor ingestion raw and decompressed body sizes
:memo: document the configuration reference
```

Common codes used in this repo: `:sparkles:` (feature), `:bug:` (fix),
`:lock:` (security), `:zap:` (performance), `:recycle:` (refactor),
`:white_check_mark:` (tests), `:memo:` (docs), `:construction_worker:` (CI),
`:arrow_up:` (dependency bump), `:art:` (formatting/structure).

Keep the subject line in the imperative mood and reference the PR/issue number
when relevant (`(#20)`).

## Pull request process

1. Fork the repository and create a branch from `main`
   (`docs/...`, `feat/...`, `fix/...`).
2. Make your change as small and focused as possible.
3. Add or update tests — a Go SDK suite entry in `test/sdk` and, where it makes
   sense, CLI coverage in `test/cli`.
4. Run `task lint` and `task test:unit`; both must pass.
5. Update the documentation if behavior changed (the `docs/` site and/or
   [`AGENTS.md`](AGENTS.md)).
6. Open the pull request against `main`. CI (`build` workflow) runs `gofmt`,
   `go vet`, and the unit suite.

## Adding a new service

The single-container promise — anything a backend needs is embedded in the same
image — must always hold. To add a service:

1. Define `internal/stores/<svc>store` with a `Store` interface and shared types.
2. Implement the protocol in `internal/servers/<svc>server`, depending only on
   that interface.
3. Mount it in `cmd/localaz` on the same process, matching Azure's endpoint
   conventions. HTTP services join the `services` slice; a non-HTTP service (like
   Service Bus AMQP) gets its own `net.Listen` accept loop wired into the same
   graceful-shutdown path.
4. Add a Go SDK suite in `test/sdk` and CLI coverage in `test/cli`.
5. Document it: a page under `docs/content/services/` and an entry in the
   [Configuration reference](docs/content/reference/_index.md).

## Documentation

The documentation lives in `docs/` as a [Hugo](https://gohugo.io) site
(`content/` Markdown, `layouts/` templates). Preview it locally with:

```bash
task docs:serve   # or: cd docs && hugo server
```

Architecture diagrams use fenced ` ```mermaid ` code blocks rendered by a
code-block render hook. The `docs/public/` and `docs/resources/` build artifacts
are git-ignored. The site is published to [localaz.dev](https://localaz.dev) by
the `docs` workflow.
