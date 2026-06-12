# Supported APIs

## Azure Blob Storage

| Operation                        | REST surface                                              |
| -------------------------------- | -------------------------------------------------------- |
| List Containers                  | `GET /{account}?comp=list`                               |
| Create Container                 | `PUT /{account}/{container}?restype=container`           |
| Get Container Properties         | `GET/HEAD /{account}/{container}?restype=container`      |
| Delete Container                 | `DELETE /{account}/{container}?restype=container`        |
| List Blobs (flat + hierarchical) | `GET /{account}/{container}?restype=container&comp=list` |
| Put Blob (block blob)            | `PUT /{account}/{container}/{blob}`                      |
| Put Block                        | `PUT /{account}/{container}/{blob}?comp=block`           |
| Put Block List                   | `PUT /{account}/{container}/{blob}?comp=blocklist`       |
| Get Blob (+ range requests)      | `GET /{account}/{container}/{blob}`                      |
| Get Blob Properties              | `HEAD /{account}/{container}/{blob}`                     |
| Delete Blob                      | `DELETE /{account}/{container}/{blob}`                   |
| Get Service Properties           | `GET /{account}?restype=service&comp=properties`         |

### Supported semantics

- Container and blob metadata (`x-ms-meta-*`).
- Content settings: content type, encoding, language, disposition, cache-control.
- Content-MD5 computation and round-tripping.
- Virtual-directory listing via a delimiter (`BlobPrefix` results).
- Block blob uploads via both the single-shot and staged-block paths, so both
  small and large SDK/CLI uploads work.
- Single-range `GET` requests (`Range` / `x-ms-range`).

### Not yet implemented

- Page blobs and append blobs.
- Shared Key signature verification (the header is accepted but not validated).
- Leases, snapshots, versioning, soft delete, tags.
- SAS token generation/validation.

## Azure Event Grid (namespace topics, pull delivery)

Served on port `10001`, api-version `2024-06-01`, compatible with the
`aznamespaces` SDK.

| Operation              | REST surface                                                          |
| ---------------------- | --------------------------------------------------------------------- |
| Publish Cloud Event(s) | `POST /topics/{topic}:publish`                                        |
| Receive                | `POST /topics/{topic}/eventsubscriptions/{sub}:receive`               |
| Acknowledge            | `POST /topics/{topic}/eventsubscriptions/{sub}:acknowledge`           |
| Release                | `POST /topics/{topic}/eventsubscriptions/{sub}:release`               |
| Reject                 | `POST /topics/{topic}/eventsubscriptions/{sub}:reject`                |
| Renew Locks            | `POST /topics/{topic}/eventsubscriptions/{sub}:renewLock`             |

- CloudEvents are stored and echoed back verbatim (single and batch content
  types). Topics and subscriptions are created on first use.
- Pull delivery with lock tokens, delivery counts, acknowledge/release/reject.

## Azure Web PubSub

Served on port `10002`, compatible with the `azwebpubsub` SDK and the
`json.webpubsub.azure.v1` WebSocket subprotocol.

| Operation            | Surface                                                       |
| -------------------- | ------------------------------------------------------------- |
| Client connect       | `GET /client/hubs/{hub}` (WebSocket upgrade)                  |
| Send to all          | `POST /api/hubs/{hub}/:send`                                  |
| Send to group        | `POST /api/hubs/{hub}/groups/{group}/:send`                   |
| Send to user         | `POST /api/hubs/{hub}/users/{user}/:send`                     |
| Send to connection   | `POST /api/hubs/{hub}/connections/{conn}/:send`               |
| Add/remove from group | `PUT`/`DELETE /api/hubs/{hub}/groups/{group}/connections/{conn}` |
| Close connection     | `DELETE /api/hubs/{hub}/connections/{conn}`                   |

- Client access URLs are signed locally; group join/leave and acks over the
  WebSocket are supported. Hub state is in-memory.

## Azure Service Bus

Served on port `5672` over AMQP 1.0, compatible with the `azservicebus` SDK
when the connection string sets `UseDevelopmentEmulator=true` (plain TCP, SASL
ANONYMOUS, CBS tokens accepted without verification).

| Capability             | Notes                                                       |
| ---------------------- | ----------------------------------------------------------- |
| Queue send             | `client.NewSender(queue)` → `SendMessage`                   |
| Queue receive + settle | `client.NewReceiverForQueue(queue)` → `ReceiveMessages` / `CompleteMessage` |
| Topic send             | `client.NewSender(topic)` → `SendMessage`                   |
| Subscription receive   | `client.NewReceiverForSubscription(topic, sub)`             |

- Message bodies are relayed verbatim. Peek-lock delivery with disposition-based
  settlement. Topic sends fan out to registered subscriptions. State is
  in-memory.

### Not yet implemented (Service Bus)

- Dead-letter queues, scheduled/deferred messages, sessions.
- Message lock renewal and management operations beyond send/receive/settle.

## Roadmap

- Queue Storage
- Table Storage
- Optional Shared Key signature verification
