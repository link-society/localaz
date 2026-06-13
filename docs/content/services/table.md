---
title: "Table Storage"
description: "Entities, $filter queries, and ETag concurrency."
weight: 3
---

Azure Table Storage (api-version `2019-02-02`) over the OData JSON wire format,
served at `http://127.0.0.1:10002/devstoreaccount1` and compatible with the
`aztables` SDK and the `az storage table` / `az storage entity` commands. Table
state is persisted under `/data`. See [Configuration](/configuration) to change
ports, addresses, and the data directory.

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

Server-managed `Timestamp` and weak `odata.etag`, optimistic concurrency via
`If-Match`, upsert, and the `$filter` (`eq` `ne` `gt` `ge` `lt` `le`, `and`/`or`,
parentheses), `$select`, and `$top` query options are supported.

**Not implemented:** batch transactions, typed `$filter` literals and OData
functions, continuation tokens, and Shared Key signature verification.

## Azure CLI

```bash
export AZURE_STORAGE_CONNECTION_STRING="UseDevelopmentStorage=true"

az storage table create --name people
az storage entity insert --table-name people --entity PartitionKey=team RowKey=alice Name=Alice
az storage entity show --table-name people --partition-key team --row-key alice
az storage entity query --table-name people --filter "PartitionKey eq 'team'"
az storage entity delete --table-name people --partition-key team --row-key alice
```

## Go SDK

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
)

func main() {
	ctx := context.Background()
	svc, err := aztables.NewServiceClientWithNoCredential("http://127.0.0.1:10002/devstoreaccount1", nil)
	if err != nil {
		panic(err)
	}

	svc.CreateTable(ctx, "people", nil)
	table := svc.NewClient("people")

	entity, _ := json.Marshal(aztables.EDMEntity{
		Entity:     aztables.Entity{PartitionKey: "team", RowKey: "alice"},
		Properties: map[string]any{"Name": "Alice", "Count": 3},
	})
	table.AddEntity(ctx, entity, nil)

	resp, err := table.GetEntity(ctx, "team", "alice", nil)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(resp.Value))

	pager := table.NewListEntitiesPager(&aztables.ListEntitiesOptions{
		Filter: to.Ptr("PartitionKey eq 'team'"),
	})
	for pager.More() {
		page, _ := pager.NextPage(ctx)
		fmt.Println(len(page.Entities))
	}

	table.DeleteEntity(ctx, "team", "alice", nil)
}
```
