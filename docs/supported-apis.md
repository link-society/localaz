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

## Azure Queue Storage

Served on port `10001`, compatible with the `azqueue` SDK and the
`az storage queue` / `az storage message` CLI commands.

| Operation              | REST surface                                          |
| ---------------------- | ----------------------------------------------------- |
| List Queues            | `GET /{account}?comp=list`                            |
| Create Queue           | `PUT /{account}/{queue}`                              |
| Delete Queue           | `DELETE /{account}/{queue}`                           |
| Get Queue Metadata     | `GET /{account}/{queue}?comp=metadata`                |
| Set Queue Metadata     | `PUT /{account}/{queue}?comp=metadata`                |
| Put Message            | `POST /{account}/{queue}/messages`                    |
| Get Messages           | `GET /{account}/{queue}/messages`                     |
| Peek Messages          | `GET /{account}/{queue}/messages?peekonly=true`       |
| Update Message         | `PUT /{account}/{queue}/messages/{messageid}`         |
| Delete Message         | `DELETE /{account}/{queue}/messages/{messageid}`      |
| Clear Messages         | `DELETE /{account}/{queue}/messages`                  |

### Supported semantics

- Queue metadata (`x-ms-meta-*`) and the approximate message count header.
- Visibility timeouts and pop receipts: dequeued messages become invisible and
  must be deleted/updated with the matching pop receipt.
- Message time-to-live (including the infinite `-1` TTL); expired messages are
  evicted on access.
- Dequeue counts and message expiry/visibility timestamps.

### Not yet implemented (Queue)

- Shared Key signature verification (the header is accepted but not validated).
- SAS token generation/validation.

## Azure Table Storage

Served on port `10002`, api-version `2019-02-02`, compatible with the
`aztables` SDK and the `az storage table` / `az storage entity` CLI commands.
Responses and errors use the OData JSON wire format.

| Operation              | REST surface                                                   |
| ---------------------- | -------------------------------------------------------------- |
| Query Tables           | `GET /{account}/Tables`                                        |
| Create Table           | `POST /{account}/Tables`                                       |
| Delete Table           | `DELETE /{account}/Tables('{name}')`                           |
| Insert Entity          | `POST /{account}/{table}`                                      |
| Query Entities         | `GET /{account}/{table}()`                                     |
| Get Entity             | `GET /{account}/{table}(PartitionKey='{pk}',RowKey='{rk}')`    |
| Update Entity (replace)| `PUT /{account}/{table}(PartitionKey='{pk}',RowKey='{rk}')`    |
| Merge Entity           | `PATCH`/`MERGE /{account}/{table}(...)`                        |
| Delete Entity          | `DELETE /{account}/{table}(PartitionKey='{pk}',RowKey='{rk}')` |

### Supported semantics

- OData JSON entities with the server-managed `Timestamp` and `odata.etag`.
- Optimistic concurrency via `If-Match` (a weak ETag, or `*` to match any).
- Upsert: `PUT`/`MERGE` with no `If-Match` inserts when the entity is absent;
  replace overwrites all properties, merge updates the supplied ones.
- `Prefer: return-no-content` for inserts and table creation (204 + ETag).
- Query options: `$filter`, `$select` and `$top`.
- `$filter` supports the comparison operators (`eq`, `ne`, `gt`, `ge`, `lt`,
  `le`) over string, numeric and boolean literals, combined with `and`/`or` and
  parentheses.

### Not yet implemented (Table)

- Batch / entity-group transactions.
- Typed `$filter` literals (datetime, guid, binary) and OData functions.
- Continuation tokens for large result sets.
- Shared Key signature verification (the header is accepted but not validated).

## Azure Event Grid (namespace topics, pull delivery)

Served on port `10003`, api-version `2024-06-01`, compatible with the
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

Served on port `10004`, compatible with the `azwebpubsub` SDK and the
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

- Optional Shared Key signature verification
