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

This tutorial creates a table, inserts an entity, and queries it back with a
`$filter`.

> **Prerequisites:** localaz is running and `AZURE_STORAGE_CONNECTION_STRING` is
> exported — see [Get Started](../../get-started/). Install the SDK with
> `go get github.com/Azure/azure-sdk-for-go/sdk/data/aztables`.

```go
import (
    "context"
    "encoding/json"
    "os"

    "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
    "github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
)

connStr := os.Getenv("AZURE_STORAGE_CONNECTION_STRING")
svc, _ := aztables.NewServiceClientFromConnectionString(connStr, nil)
table := svc.NewClient("people")

ctx := context.Background()

// 1. Create the table.
table.CreateTable(ctx, nil)

// 2. Insert an entity keyed by (PartitionKey, RowKey).
entity := aztables.EDMEntity{
    Entity:     aztables.Entity{PartitionKey: "team", RowKey: "alice"},
    Properties: map[string]any{"Name": "Alice"},
}
b, _ := json.Marshal(entity)
table.AddEntity(ctx, b, nil)

// 3. Query the partition back. Iterate the pager with pager.More() /
//    pager.NextPage(ctx) to read the matching entities.
pager := table.NewListEntitiesPager(&aztables.ListEntitiesOptions{
    Filter: to.Ptr("PartitionKey eq 'team'"),
})
```

## Example: Azure CLI

Export `AZURE_STORAGE_CONNECTION_STRING` first — see
[Get Started](../../get-started/).

```bash
# 1. Create a table.
az storage table create --name people

# 2. Insert an entity (PartitionKey + RowKey + properties).
az storage entity insert --table-name people \
  --entity PartitionKey=team RowKey=alice Name=Alice

# 3. Query entities in the partition with an OData $filter.
az storage entity query --table-name people --filter "PartitionKey eq 'team'"
```
