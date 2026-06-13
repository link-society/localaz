---
title: "Monitor Logs"
description: "Logs ingestion plus a documented KQL-subset query."
weight: 7
---

Azure Monitor Logs emulation: two data planes on one port — **Logs Ingestion**
and the **Log Analytics query**. Compatible with the `monitor/ingestion/azlogs`
and `monitor/query/azlogs` SDKs.

## Endpoint

| | |
| --- | --- |
| URL | `https://127.0.0.1:10005` |
| Protocol | HTTPS / REST |
| Persisted | No — tables are in-memory |

Both SDKs send bearer tokens, which azcore refuses over plain HTTP — so Monitor
must serve HTTPS in practice (run localaz with `-tls-auto`). See
[Control plane](../control-plane/) for the TLS and sign-in recipe.

## Configuration

| Flag | Environment variable | Default |
| ---- | -------------------- | ------- |
| `-monitor-addr` | `LOCALAZ_MONITOR_ADDR` | `:10005` |

## Supported operations

| Operation | REST surface |
| --------- | ------------ |
| Upload logs | `POST /dataCollectionRules/{ruleId}/streams/{stream}` (api-version `2023-01-01`) |
| Query workspace | `POST /v1/workspaces/{workspaceId}/query` |

Ingestion accepts a JSON array of records (optionally gzip-encoded) and returns
`204 No Content`. The stream name selects the destination table, with a leading
`Custom-` prefix stripped (`Custom-AppLogs_CL` → table `AppLogs_CL`). A synthetic
`TimeGenerated` column is added when a record omits one.

### KQL subset

Queries run a documented KQL subset and return a single `PrimaryResult` table:

```text
TableName
| where <col> <op> <literal> [and|or <col> <op> <literal> ...]
| project <col> [, <col> ...]
| sort by <col> [asc|desc]      (also "order by")
| take <n>                      (also "limit")
| count
```

where `<op>` is one of `==` `!=` `<` `<=` `>` `>=` and literals are quoted
strings, numbers, or `true`/`false`.

**Not yet implemented:** `summarize`, `join`, `extend`, `distinct`,
parentheses, scalar functions, timespan filtering, and cross-workspace queries.

## Example: Go SDK

This tutorial ingests two records into a custom table, then queries them back
with the KQL subset. **Ingestion and query are two separate `azlogs` packages**
(`monitor/ingestion/azlogs` and `monitor/query/azlogs`); they are aliased below
to avoid the name clash, and both are pointed at the local endpoint with a custom
`cloud.Configuration`.

> **Prerequisites:** run localaz with `-tls-auto` so Monitor serves HTTPS (the
> SDK refuses bearer tokens over plain HTTP), and have the client trust the
> generated certificate — see [Control plane](../control-plane/). `cred` may be
> any `azcore.TokenCredential`; localaz does not validate it.

```go
import (
    "context"
    "encoding/json"

    "github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
    "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
    azingest "github.com/Azure/azure-sdk-for-go/sdk/monitor/ingestion/azlogs"
    azquery "github.com/Azure/azure-sdk-for-go/sdk/monitor/query/azlogs"
)

ctx := context.Background()
const endpoint = "https://127.0.0.1:10005"

// Point both clients at the local endpoint.
cfg := cloud.Configuration{
    Services: map[cloud.ServiceName]cloud.ServiceConfiguration{
        azingest.ServiceNameIngestion: {Audience: "https://monitor.azure.com", Endpoint: endpoint},
        azquery.ServiceName:           {Audience: "https://api.loganalytics.io", Endpoint: endpoint},
    },
}

// 1. Ingest records. The "Custom-" prefix is stripped, so they land in AppLogs_CL.
ingOpts := &azingest.ClientOptions{}
ingOpts.Cloud = cfg
ingest, _ := azingest.NewClient(endpoint, cred, ingOpts)
records := []map[string]any{{"Message": "hello", "Level": "Info"}}
body, _ := json.Marshal(records)
ingest.Upload(ctx, "dcr-id", "Custom-AppLogs_CL", body, nil)

// 2. Query the table back. The query client takes no endpoint — it reads it
//    from the cloud config above.
qOpts := &azquery.ClientOptions{}
qOpts.Cloud = cfg
query, _ := azquery.NewClient(cred, qOpts)
res, _ := query.QueryWorkspace(ctx, "workspace-id",
    azquery.QueryBody{Query: to.Ptr("AppLogs_CL | where Level == 'Info' | count")}, nil)
_ = res
```

## Example: Azure CLI

After registering localaz as a cloud and signing in (see
[Control plane](../control-plane/)):

```bash
az monitor log-analytics query \
  --workspace <workspace-id> \
  --analytics-query "AppLogs_CL | where Level == 'Info' | project Message | take 10"
```
