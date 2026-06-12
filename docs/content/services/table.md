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

```go
connStr := os.Getenv("AZURE_STORAGE_CONNECTION_STRING")
svc, _ := aztables.NewServiceClientFromConnectionString(connStr, nil)
table := svc.NewClient("people")

ctx := context.Background()
table.CreateTable(ctx, nil)

entity := aztables.EDMEntity{
    Entity:     aztables.Entity{PartitionKey: "team", RowKey: "alice"},
    Properties: map[string]any{"Name": "Alice"},
}
b, _ := json.Marshal(entity)
table.AddEntity(ctx, b, nil)

pager := table.NewListEntitiesPager(&aztables.ListEntitiesOptions{
    Filter: to.Ptr("PartitionKey eq 'team'"),
})
```

## Example: Azure CLI

```bash
eval "$(task env:conn-string)"

az storage table create --name people
az storage entity insert --table-name people \
  --entity PartitionKey=team RowKey=alice Name=Alice
az storage entity query --table-name people --filter "PartitionKey eq 'team'"
```
