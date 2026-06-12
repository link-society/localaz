// Package cli contains the end-to-end test suite for localaz.
//
// These tests drive the real Azure CLI (`az`) as a subprocess against a running
// emulator, asserting on the structured output the CLI produces. They are
// guarded by the "cli" build tag so they do not run as part of the default
// `go test ./...` invocation; use `task test:cli` (or
// `go test -tags cli ./test/cli/...`).
//
// By default the suite builds and launches localaz on a free local port. Set
// LOCALAZ_CLI_ENDPOINT to a blob endpoint (for example
// http://127.0.0.1:10000/devstoreaccount1) to run against an already-running
// instance, such as the Docker container.
package cli
