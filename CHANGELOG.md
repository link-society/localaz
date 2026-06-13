# Changelog

All notable changes to **localaz** are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

_Maintenance only: dependency and GitHub Actions bumps since v0.1.0._

## [0.1.0] - 2026-06-12

Initial release: a single-process, single-container local Azure emulator that the
Azure CLI and the official SDKs work against unchanged.

### Added

- **Blob Storage** emulation over the native REST (XML) protocol — block blobs,
  container/blob metadata, content settings, Content-MD5, single-range `GET`,
  single-shot and staged-block uploads, and paginated List Blobs / List
  Containers (`maxresults` + opaque base64url continuation markers).
- **Queue Storage** emulation — messages, visibility timeouts, pop receipts,
  message TTL, and approximate message counts.
- **Table Storage** emulation over OData JSON — entities, a documented `$filter`
  subset, `$select`/`$top`, server-managed `Timestamp`, and weak-ETag optimistic
  concurrency.
- **Event Grid** emulation — namespace topics with pull delivery, lock tokens,
  delivery counts, and acknowledge/release/reject settlement.
- **Web PubSub** emulation — REST management surface plus WebSocket clients using
  the `json.webpubsub.azure.v1` subprotocol.
- **Service Bus** emulation over hand-rolled AMQP 1.0 — queue and topic/
  subscription send/receive with peek-lock settlement and multi-frame transfer
  reassembly.
- **Monitor Logs** emulation — Logs Ingestion plus a documented KQL-subset Log
  Analytics query, on a single TLS port.
- **Control plane** — an Entra ID (AAD) + Resource Manager (ARM) emulator that
  lets the CLI/SDKs register localaz as a custom Azure cloud, sign in, and route
  data-plane commands (e.g. `az servicebus`, `az monitor log-analytics query`)
  to it. Hand-rolled RS256 JWTs verify against a published JWKS; the signing key
  is persisted so issued tokens survive restarts.
- **Single container** — a multi-arch (`linux/amd64` + `linux/arm64`) distroless
  image, a Hugo documentation site, and CI / release workflows.

### Security

- Bounded AMQP list/map element counts and frame size in the Service Bus decoder.
- Streamed the Blob data path to and from disk to avoid out-of-memory on large
  blobs.
- Bounded Monitor ingestion raw and decompressed body sizes (gzip-bomb guard).
- Bounded ARM provider PUT bodies and the in-memory resource store.
- Capped the `$filter` parser depth to prevent stack overflow from deeply nested
  input.
- Made store persistence crash-safe via atomic temp + fsync + rename writes.
- Recovered panics in the access-log middleware so a handler crash returns `500`.
- Gracefully shut down all listeners on a serve failure instead of `os.Exit`.
- Included `-advertise-host` in the dev certificate SANs and wrote cert/key
  `0600`.

[Unreleased]: https://github.com/link-society/localaz/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/link-society/localaz/releases/tag/v0.1.0
