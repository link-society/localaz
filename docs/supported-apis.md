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
- The **management plane** is served by the control-plane ARM resource provider
  (`Microsoft.ServiceBus`), so `az servicebus` namespace/queue/topic/subscription
  commands work too; the data-plane broker auto-creates entities on first use
  and is not pre-provisioned by ARM.

### Not yet implemented (Service Bus)

- Dead-letter queues, scheduled/deferred messages, sessions.
- Message lock renewal and management operations beyond send/receive/settle.

## Azure Monitor Logs

Served on port `10005`, compatible with the `monitor/ingestion/azlogs`
(Logs Ingestion) and `monitor/query/azlogs` (Log Analytics query) SDKs.

| Operation        | REST surface                                                         |
| ---------------- | -------------------------------------------------------------------- |
| Upload logs      | `POST /dataCollectionRules/{ruleId}/streams/{stream}` (api-version `2023-01-01`) |
| Query workspace  | `POST /v1/workspaces/{workspaceId}/query`                            |

- Ingestion accepts a JSON array of records (optionally gzip-encoded) and
  returns `204 No Content`. The data-collection rule id is accepted but not
  validated; the stream name selects the destination table, with a leading
  `Custom-` prefix stripped (`Custom-AppLogs_CL` → table `AppLogs_CL`). A
  synthetic `TimeGenerated` column is added when a record omits one.
- Queries run a documented **KQL subset** and return a single `PrimaryResult`
  table with inferred column types. State is in-memory (logs are transient and
  never persisted).

The supported KQL grammar is:

```
TableName
| where <col> <op> <literal> [and|or <col> <op> <literal> ...]
| project <col> [, <col> ...]
| sort by <col> [asc|desc]      (also spelled "order by")
| take <n>                      (also spelled "limit")
| count
```

where `<op>` is one of `==` `!=` `<` `<=` `>` `>=` and literals are quoted
strings, numbers, or `true`/`false`.

### Not yet implemented (Monitor Logs)

- The full KQL language: `summarize`, `join`, `extend`, `distinct`,
  parentheses, OData/scalar functions, and timespan filtering.
- Data-collection-rule schema/transform validation; cross-workspace queries.

## Azure Entra ID + Resource Manager (control plane)

Served on ports `10006` (Entra ID / AAD) and `10007` (Resource Manager / ARM),
over HTTPS when TLS is enabled. Together they let the Azure CLI and the Azure
SDKs treat localaz as a custom cloud: register it, `az login` with a service
principal, and have data-plane commands routed to localaz. See
[configuration.md](configuration.md) for the registration recipe.

### Entra ID (AAD)

| Operation                 | REST surface                                            |
| ------------------------- | ------------------------------------------------------- |
| OpenID configuration      | `GET /{tenant}/.well-known/openid-configuration` (and `/{tenant}/v2.0/...`) |
| JWKS (signing keys)       | `GET /{tenant}/discovery/keys`                          |
| Token                     | `POST /{tenant}/oauth2/token` (and `/{tenant}/oauth2/v2.0/token`) |

- Tokens are minted as hand-rolled **RS256** JWTs and verify against the
  published JWKS. The audience is derived from the requested scope or `resource`
  parameter; client id, secret and assertions are accepted but not validated.
- Refresh and id tokens are issued for non-`client_credentials` grants. Sign in
  with `--tenant adfs` so MSAL skips public instance discovery.

### Resource Manager (ARM)

| Operation                 | REST surface                                            |
| ------------------------- | ------------------------------------------------------- |
| Cloud metadata            | `GET /metadata/endpoints`                               |
| List tenants              | `GET /tenants`                                          |
| List subscriptions        | `GET /subscriptions`                                    |
| Get subscription          | `GET /subscriptions/{id}`                               |
| List resource groups      | `GET /subscriptions/{id}/resourcegroups`                |
| Create/replace group      | `PUT /subscriptions/{id}/resourcegroups/{name}`         |
| Get resource group        | `GET /subscriptions/{id}/resourcegroups/{name}`         |
| Delete resource group     | `DELETE /subscriptions/{id}/resourcegroups/{name}`      |

- A single fixed subscription and tenant are configured at startup; resource
  groups are created at runtime. State is in-memory and never persisted.
- The `/metadata/endpoints` document advertises the login, resource-manager and
  Log Analytics endpoints, keyed for the CLI's cloud-metadata resolution (the
  Log Analytics host is exposed under the extension's
  `logAnalyticslogAnalyticsResourceId` index). Its `name` field must match the
  registered cloud name (`-arm-cloud-name`).

### Resource providers

A generic resource-provider surface backs the typed management commands. Any
`.../providers/{namespace}/{type}/{name}` resource is stored and echoed back
with a terminal `provisioningState`, which is enough for the CLI's
create/show/list/delete commands to run against localaz.

| Operation                 | REST surface                                                                 |
| ------------------------- | ---------------------------------------------------------------------------- |
| Create/replace resource   | `PUT .../providers/{ns}/{type}/{name}`                                       |
| Get resource              | `GET .../providers/{ns}/{type}/{name}`                                       |
| List resources            | `GET .../providers/{ns}/{type}` (subscription- or group-scoped)              |
| Delete resource           | `DELETE .../providers/{ns}/{type}/{name}`                                    |
| Name availability         | `POST .../providers/{ns}/checkNameAvailability`                              |
| List keys                 | `POST .../providers/{ns}/.../authorizationRules/{rule}/listKeys`             |

This is exercised end to end by `az servicebus` (`Microsoft.ServiceBus`):
namespace, queue, topic and topic-subscription create/show/list/delete all
resolve to localaz through the emulated Resource Manager.

### Not yet implemented (control plane)

- Token signature/scope validation and RBAC; device-code/interactive logins.
- Long-running-operation polling (resources provision synchronously) and
  resource-specific validation or data-plane wiring (e.g. an ARM-created Service
  Bus queue is not pre-provisioned in the AMQP broker, which auto-creates it on
  first use).
- Subscription or tenant management operations.

## Roadmap

- Optional Shared Key signature verification
