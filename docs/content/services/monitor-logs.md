---
title: "Monitor Logs"
description: "Logs ingestion plus a documented KQL-subset query."
weight: 7
---

Azure Monitor Logs: two data planes on one port — **Logs Ingestion** and the
**Log Analytics query** — served at `https://127.0.0.1:10005` and compatible with
the `monitor/ingestion/azlogs` and `monitor/query/azlogs` SDKs. Tables are
in-memory.

Both SDKs send bearer tokens, which azcore refuses over plain HTTP, so run
localaz with `-tls-auto` and trust the generated certificate. See the
[Control plane](/services/control-plane) sign-in recipe and
[Configuration](/configuration).

## Supported operations

| Operation | REST surface |
| --------- | ------------ |
| Upload logs | `POST /dataCollectionRules/{ruleId}/streams/{stream}` |
| Query workspace | `POST /v1/workspaces/{workspaceId}/query` |

Ingestion accepts a JSON array of records (optionally gzip-encoded). The stream
name selects the destination table, with a leading `Custom-` prefix stripped
(`Custom-AppLogs_CL` → `AppLogs_CL`). The query runs a documented KQL subset:

```text
TableName
| where <col> <op> <literal> [and|or ...]   (== != < <= > >=)
| project <col> [, <col> ...]
| sort by <col> [asc|desc]
| take <n>
| count
```

**Not implemented:** `summarize`, `join`, `extend`, `distinct`, parentheses,
scalar functions, timespan filtering, and cross-workspace queries.

## Azure CLI

After registering localaz as a cloud and signing in (see
[Control plane](/services/control-plane)):

```bash
az monitor log-analytics query \
  -w 33333333-3333-3333-3333-333333333333 \
  --analytics-query "AppLogs_CL | where Level == 'error' and Source == 'worker' | project Message"

az monitor log-analytics query \
  -w 33333333-3333-3333-3333-333333333333 \
  --analytics-query "AppLogs_CL | where Code >= 500 | sort by Code desc | take 1"

az monitor log-analytics query \
  -w 33333333-3333-3333-3333-333333333333 \
  --analytics-query "AppLogs_CL | count"
```

## Go SDK

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	azingest "github.com/Azure/azure-sdk-for-go/sdk/monitor/ingestion/azlogs"
	azquery "github.com/Azure/azure-sdk-for-go/sdk/monitor/query/azlogs"
)

func main() {
	const endpoint = "https://127.0.0.1:10005"
	ctx := context.Background()

	c := cloud.Configuration{
		ActiveDirectoryAuthorityHost: "https://127.0.0.1:10006/",
		Services: map[cloud.ServiceName]cloud.ServiceConfiguration{
			azingest.ServiceNameIngestion: {Audience: "https://monitor.azure.com", Endpoint: endpoint},
			azquery.ServiceName:           {Audience: "https://api.loganalytics.io", Endpoint: endpoint},
		},
	}
	opts := azcore.ClientOptions{Cloud: c}

	cred, err := azidentity.NewClientSecretCredential("adfs", "<app-id>", "<secret>",
		&azidentity.ClientSecretCredentialOptions{ClientOptions: opts, DisableInstanceDiscovery: true})
	if err != nil {
		panic(err)
	}

	ingest, _ := azingest.NewClient(endpoint, cred, &azingest.ClientOptions{ClientOptions: opts})
	records, _ := json.Marshal([]map[string]any{
		{"Level": "info", "Message": "started", "Code": 200},
		{"Level": "error", "Message": "boom", "Code": 500},
	})
	ingest.Upload(ctx, "dcr-localaz", "Custom-AppLogs_CL", records, nil)

	query, _ := azquery.NewClient(cred, &azquery.ClientOptions{ClientOptions: opts})
	resp, err := query.QueryWorkspace(ctx, "workspace-localaz",
		azquery.QueryBody{Query: to.Ptr("AppLogs_CL | where Level == 'error' | count")}, nil)
	if err != nil {
		panic(err)
	}
	fmt.Println(resp.Tables[0].Rows)
}
```
