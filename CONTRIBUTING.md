# Contributing to localaz

Thanks for your interest in improving **localaz**, a single-process, single-container
local Azure emulator. This guide covers how to set up your environment and how to
get a change merged.

## Table of contents

- [Prerequisites](#prerequisites)
- [Getting started](#getting-started)
- [Build, run, and test](#build-run-and-test)
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

## Pull request process

1. Fork the repository and create a branch from `main`.
2. Make your change as small and focused as possible.
3. Add or update tests — a Go SDK suite entry in `test/sdk` and, where it makes
   sense, CLI coverage in `test/cli`.
4. Run `task lint` and `task test:unit`; both must pass.
5. Update the documentation if behavior changed (the `docs/` site).
6. Open the pull request against `main`. CI (`build` workflow) runs `gofmt`,
   `go vet`, and the unit suite.

## Adding a new service

The single-container promise — anything a backend needs is embedded in the same
image — must always hold. At a high level:

1. Add a state store and a protocol implementation, then mount the service in the
   same process.
2. Add a Go SDK suite in `test/sdk` and CLI coverage in `test/cli`.
3. Document it: a page under `docs/content/services/` and an entry in the
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
