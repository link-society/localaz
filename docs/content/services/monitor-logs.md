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
| URL | `http://127.0.0.1:10005` (HTTPS with TLS) |
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

```go
// Ingestion
ingest, _ := azlogs.NewClient("https://127.0.0.1:10005", cred, nil)
records := []map[string]any{{"Message": "hello", "Level": "Info"}}
body, _ := json.Marshal(records)
ingest.Upload(ctx, "dcr-id", "Custom-AppLogs_CL", body, nil)

// Query
query, _ := azlogs.NewClient("https://127.0.0.1:10005", cred, nil)
res, _ := query.QueryWorkspace(ctx, "workspace-id",
    azlogs.QueryBody{Query: to.Ptr("AppLogs_CL | where Level == 'Info' | count")}, nil)
```

## Example: Azure CLI

After registering localaz as a cloud and signing in (see
[Control plane](../control-plane/)):

```bash
az monitor log-analytics query \
  --workspace <workspace-id> \
  --analytics-query "AppLogs_CL | where Level == 'Info' | project Message | take 10"
```
