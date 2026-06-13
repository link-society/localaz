---
title: "Table Storage"
description: "Entities, $filter queries, and ETag concurrency."
weight: 3
---

Azure Table Storage emulation (api-version `2019-02-02`) over the OData JSON
wire format. Compatible with the `aztables` SDK and the `az storage table` /
`az storage entity` CLI commands.

## Endpoint

| | |
| --- | --- |
| URL | `http://127.0.0.1:10002/devstoreaccount1` |
| Protocol | HTTP / REST (OData JSON) |
| Persisted | Yes — state lives under `/data` |

## Configuration

| Flag | Environment variable | Default |
| ---- | -------------------- | ------- |
| `-table-addr` | `LOCALAZ_TABLE_ADDR` | `:10002` |
| `-data` | `LOCALAZ_DATA_DIR` | `/data` |

## Supported operations

| Operation | REST surface |
| --------- | ------------ |
| Query / Create / Delete Table | `GET` / `POST /{account}/Tables`, `DELETE /{account}/Tables('{name}')` |
| Insert Entity | `POST /{account}/{table}` |
| Query Entities | `GET /{account}/{table}()` |
| Get Entity | `GET /{account}/{table}(PartitionKey='{pk}',RowKey='{rk}')` |
| Update (replace) | `PUT /{account}/{table}(...)` |
| Merge | `PATCH` / `MERGE /{account}/{table}(...)` |
| Delete Entity | `DELETE /{account}/{table}(...)` |

Supported semantics: server-managed `Timestamp` and weak `odata.etag`,
optimistic concurrency via `If-Match` (`*` matches any), upsert (insert when
absent), `Prefer: return-no-content`, and the `$filter`, `$select`, `$top`
query options.

`$filter` supports the comparison operators (`eq`, `ne`, `gt`, `ge`, `lt`, `le`)
over string/number/bool literals, combined with `and`/`or` and parentheses.

**Not yet implemented:** batch transactions, typed `$filter` literals and OData
functions, continuation tokens, and Shared Key signature verification.

## Example: Go SDK

**Prerequisites:** export `AZURE_STORAGE_CONNECTION_STRING` so the SDK points at
the Table endpoint above — see the [Get Started guide](../../get-started/).

`AddEntity` takes the entity as raw JSON bytes, so build an `aztables.EDMEntity`
and `json.Marshal` it before sending. This walks through create, insert, read
back, and a filtered list:

```go
connStr := os.Getenv("AZURE_STORAGE_CONNECTION_STRING")
svc, _ := aztables.NewServiceClientFromConnectionString(connStr, nil)
table := svc.NewClient("people")

ctx := context.Background()
table.CreateTable(ctx, nil)

entity := aztables.EDMEntity{
    Entity:     aztables.Entity{PartitionKey: "team", RowKey: "alice"},
    Properties: map[string]any{"Name": "Alice", "Count": 3},
}
b, _ := json.Marshal(entity)
table.AddEntity(ctx, b, nil)

// Read the entity back by its PartitionKey / RowKey.
resp, _ := table.GetEntity(ctx, "team", "alice", nil)
var got aztables.EDMEntity
json.Unmarshal(resp.Value, &got)
// got.Properties["Name"] == "Alice", got.Properties["Count"] == 3

pager := table.NewListEntitiesPager(&aztables.ListEntitiesOptions{
    Filter: to.Ptr("PartitionKey eq 'team'"),
})
```

The filtered pager returns the single `alice` entity inserted above.

## Example: Azure CLI

**Prerequisites:** export `AZURE_STORAGE_CONNECTION_STRING` so the CLI targets
the Table endpoint above — see the [Get Started guide](../../get-started/).

```bash
az storage table create --name people
az storage entity insert --table-name people \
  --entity PartitionKey=team RowKey=alice Name=Alice
az storage entity query --table-name people --filter "PartitionKey eq 'team'"
```

The final `query` lists the entities matching the filter — here the single
`alice` row in the `team` partition.
