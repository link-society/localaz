---
title: "Monitor Logs"
description: "Logs ingestion plus a documented KQL-subset query."
weight: 7
---

Azure Monitor Logs emulation: two data planes on one port — **Logs Ingestion**
and the **Log Analytics query**. Compatible with the `monitor/ingestion/azlogs`
and `monitor/query/azlogs` SDKs.

**Prerequisites:** start localaz with `-tls-auto` so Monitor serves HTTPS; for
the CLI example, also register localaz as a cloud and sign in (see
[Control plane](../control-plane/)).

## Endpoint

| | |
| --- | --- |
| URL | `https://127.0.0.1:10005` |
| Protocol | HTTP / REST |
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

The two SDK packages are both named `azlogs`, so alias them (for example
`azingest` and `azquery`). The emulator never validates the bearer token, so
`cred` can be any `azcore.TokenCredential` (for example
`azidentity.NewAzureCLICredential(nil)`). The client must also trust localaz's
TLS certificate — point `SSL_CERT_FILE` at `<data>/tls/localaz.crt` or supply a
custom `Transport` (see `test/sdk` for the in-process pattern). `ctx` is a
`context.Context` such as `context.Background()`.

```go
import (
    "encoding/json"

    "github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
    "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
    azingest "github.com/Azure/azure-sdk-for-go/sdk/monitor/ingestion/azlogs"
    azquery "github.com/Azure/azure-sdk-for-go/sdk/monitor/query/azlogs"
)

const endpoint = "https://127.0.0.1:10005"

// Ingestion — the ingestion client takes the endpoint directly.
ingest, _ := azingest.NewClient(endpoint, cred, nil)
records := []map[string]any{{"Message": "hello", "Level": "Info"}}
body, _ := json.Marshal(records)
ingest.Upload(ctx, "dcr-id", "Custom-AppLogs_CL", body, nil) // 204 No Content

// Query — the query client takes NO endpoint argument; set it via Cloud.
qOpts := &azquery.ClientOptions{}
qOpts.Cloud = cloud.Configuration{
    Services: map[cloud.ServiceName]cloud.ServiceConfiguration{
        azquery.ServiceName: {Endpoint: endpoint, Audience: endpoint},
    },
}
query, _ := azquery.NewClient(cred, qOpts)
res, _ := query.QueryWorkspace(ctx, "workspace-id",
    azquery.QueryBody{Query: to.Ptr("AppLogs_CL | where Level == 'Info' | count")}, nil)
```

The query returns one table named `PrimaryResult`; the `count` above yields a
single row holding the number of matching records.

## Example: Azure CLI

After registering localaz as a cloud and signing in (see
[Control plane](../control-plane/)):

```bash
az monitor log-analytics query \
  --workspace <workspace-id> \
  --analytics-query "AppLogs_CL | where Level == 'Info' | project Message | take 10"
```
