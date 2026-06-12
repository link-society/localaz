# AGENTS.md

Guidance for AI agents and contributors working on localaz.

## Project overview

localaz is a local **Azure emulator**: a single Go process (shipped as one
Docker container) that speaks the native Azure Storage REST protocols so the
Azure CLI and the official SDKs work against it unchanged. The first service is
Azure Blob Storage. See [ARCHITECTURE.md](ARCHITECTURE.md) for the full design.

- Module path: `localaz.dev`
- Go version: 1.26
- The main binary has **zero third-party dependencies**. The `azblob` SDK is
  used only in the test suite.

## Repository layout

```
cmd/localaz            entrypoint / process wiring
internal/blobserver    Azure Blob REST protocol (routing, XML, headers, errors)
internal/blobstore     storage abstraction (the Store interface) + types
  └── fsstore          filesystem-backed implementation
internal/azerr         faithful Azure error responses
test/sdk               integration tests via the Azure Go SDK
test/e2e               end-to-end tests via the Azure CLI (build tag: e2e)
docker/                localaz.dockerfile, docker-compose.dev.yml
docs/                  configuration, supported APIs, testing
```

## Conventions

- **One structure and its methods per file.** Keep files focused and short.
  Put shared helpers in a `helpers.go` (or similarly named) file. `fsstore` is
  the reference for this: `store.go`, `types.go`, `paths.go`, `helpers.go`,
  `persistence.go`, `containers.go`, `blobs.go`, `blocks.go`.
- **The protocol layer depends only on the `blobstore.Store` interface.** Never
  let `internal/blobserver` reach into a concrete backend.
- **Stay faithful to the Azure wire format.** XML element names, header casing,
  and date formats must match what the SDKs expect (e.g. `<Etag>` not `<ETag>`,
  `Last-Modified` as RFC1123 GMT). When in doubt, check what the Go SDK or CLI
  actually sends/parses rather than guessing.
- Run `task fmt` and keep `task lint` (gofmt + `go vet`) clean.

## Build, run, and test

Use Task (see [Taskfile.yml](Taskfile.yml)):

```bash
task build        # build the binary
task run          # run locally
task test:unit    # go test ./...  (Go SDK suite, self-contained)
task test:e2e     # go test -tags e2e ./test/e2e/...  (requires az)
task lint         # gofmt check + go vet
task docker:build # build the container image
task docker:up    # docker compose -f docker/docker-compose.dev.yml up --build
```

The E2E suite shells out to the real `az` CLI and is guarded by the `e2e` build
tag so it never runs under `task test:unit`. `test/e2e/doc.go` is an untagged
package doc file so `go test ./...` does not fail with "build constraints
exclude all Go files".

## Adding a new service

1. Define `internal/<svc>store` with a `Store` interface and shared types.
2. Implement the REST protocol in `internal/<svc>server`, depending only on that
   interface.
3. Mount the handler in `cmd/localaz` on the same process, matching Azure's
   endpoint conventions.
4. Add a Go SDK suite in `test/sdk` and CLI coverage in `test/e2e`.

Keep the single-container promise: anything a backend needs is embedded in the
same image.

## Gotchas

- **Auth is not verified.** The Shared Key header is accepted but its signature
  is not validated (standard for local emulators). Do not add verification
  without making it opt-in.
- **Docker `/data` permissions.** The runtime image is distroless `nonroot`
  (uid 65532). The Dockerfile creates `/data` in the build stage and copies it
  with `--chown=65532:65532` so the non-root user can write to it.
- **Persistence.** State lives under `/data`; mount a volume to keep it across
  restarts. `fsstore` rebuilds its in-memory index from disk on startup.
- **Blob name encoding.** Blob names may contain `/`. On disk they are stored
  under a URL-safe base64 key; do not assume a 1:1 path mapping.
