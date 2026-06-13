# SDK suite (`test/sdk`)

Integration tests driven by the official **Azure Go SDKs**. Each test starts an
in-process emulator and exercises it through the real client libraries. No
Docker or external tools required.

```bash
task test:unit        # or: go test ./test/sdk/...
```

`helpers_test.go` holds the shared helpers (in-process emulator startup, client
constructors, a fake `TokenCredential`, and small utilities).

## Scenarios

### Blob (`azblob`)

| Test | Scenario |
| ---- | -------- |
| `TestContainerLifecycle` | Create, get properties, list and delete a container |
| `TestOperationsOnMissingContainer` | Correct errors for operations on a missing container |
| `TestListBlobsFlat` | Flat blob listing |
| `TestListBlobsHierarchy` | Hierarchical listing with a delimiter (`BlobPrefix`) |
| `TestBlobUploadDownload` | Round-trip upload/download of a block blob |
| `TestBlobMetadataAndProperties` | Metadata (`x-ms-meta-*`) and content settings |
| `TestLargeBlobBlockUpload` | Staged Put Block + Put Block List for a large blob |

### Queue (`azqueue`)

| Test | Scenario |
| ---- | -------- |
| `TestQueueListAndMetadata` | List queues; get/set queue metadata |
| `TestQueueSendReceiveDelete` | Enqueue, dequeue, and delete with pop receipt |
| `TestQueueVisibilityTimeout` | Dequeued messages become invisible until timeout |

### Table (`aztables`)

| Test | Scenario |
| ---- | -------- |
| `TestTableListTables` | Create and list tables |
| `TestTableInsertGetDelete` | Insert, get and delete an entity |
| `TestTableUpsertReplaceMerge` | Upsert, replace, and merge semantics + ETag |
| `TestTableListWithFilter` | `$filter` query over entities |

### Event Grid (`aznamespaces`)

| Test | Scenario |
| ---- | -------- |
| `TestEventGridPublishReceiveAcknowledge` | Publish a CloudEvent, pull-receive, acknowledge |
| `TestEventGridReleaseRedelivers` | Released events are redelivered |

### Web PubSub (`azwebpubsub`)

| Test | Scenario |
| ---- | -------- |
| `TestWebPubSubBroadcast` | REST send-to-all reaches a connected WebSocket client |
| `TestWebPubSubPublishToGroup` | Group join + send-to-group delivery |

### Service Bus (`azservicebus`)

| Test | Scenario |
| ---- | -------- |
| `TestServiceBusQueueSendReceive` | Queue send, peek-lock receive, complete |
| `TestServiceBusTopicSubscription` | Topic send fans out to a subscription |

### Monitor Logs (`monitor/ingestion`, `monitor/query`)

| Test | Scenario |
| ---- | -------- |
| `TestMonitorIngestAndQuery` | Ingest records, then query the resulting table |
| `TestMonitorWhereAndProject` | KQL `where` + `project` |
| `TestMonitorNumericFilterAndCount` | Numeric `where` comparison and `count` |
| `TestMonitorTakeAndSort` | `sort by` and `take` |

### Control plane (`azidentity`, `armsubscriptions`, `armresources`)

| Test | Scenario |
| ---- | -------- |
| `TestAADClientSecretCredential` | Client-secret credential mints a token against the emulated AAD |
| `TestARMListSubscriptions` | List the fixed subscription/tenant |
| `TestARMResourceGroupLifecycle` | Create, get, list and delete a resource group |

### Key Vault (`azsecrets`)

| Test | Scenario |
| ---- | -------- |
| `TestKeyVaultSecretSetGet` | Set a secret with tags, then get it back |
| `TestKeyVaultSecretUpdate` | Update attributes/content type; value stays immutable |
| `TestKeyVaultListSecretsAndVersions` | List secrets and a secret's versions |
| `TestKeyVaultDeleteSecret` | Delete a secret, then confirm it is gone |
