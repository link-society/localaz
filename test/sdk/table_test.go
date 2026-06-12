package sdk

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"

	"localaz.dev/internal/tableserver"
	"localaz.dev/internal/tablestore"
)

// newTableServiceClient spins up an in-process Table emulator and returns a
// service client pointed at it.
func newTableServiceClient(t *testing.T) *aztables.ServiceClient {
	t.Helper()
	store, err := tablestore.New(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	ts := httptest.NewServer(tableserver.New(store))
	t.Cleanup(ts.Close)

	client, err := aztables.NewServiceClientWithNoCredential(ts.URL+"/devstoreaccount1", nil)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	return client
}

// tableEntity is a minimal entity shape for marshalling test payloads.
type tableEntity struct {
	PartitionKey string `json:"PartitionKey"`
	RowKey       string `json:"RowKey"`
	Name         string `json:"Name,omitempty"`
	Count        int    `json:"Count,omitempty"`
}

func marshalEntity(t *testing.T, e tableEntity) []byte {
	t.Helper()
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal entity: %v", err)
	}
	return b
}

func TestTableInsertGetDelete(t *testing.T) {
	svc := newTableServiceClient(t)
	c := ctx(t)

	if _, err := svc.CreateTable(c, "people", nil); err != nil {
		t.Fatalf("create table: %v", err)
	}
	client := svc.NewClient("people")

	if _, err := client.AddEntity(c, marshalEntity(t, tableEntity{
		PartitionKey: "team",
		RowKey:       "alice",
		Name:         "Alice",
		Count:        3,
	}), nil); err != nil {
		t.Fatalf("add entity: %v", err)
	}

	resp, err := client.GetEntity(c, "team", "alice", nil)
	if err != nil {
		t.Fatalf("get entity: %v", err)
	}
	var got tableEntity
	if err := json.Unmarshal(resp.Value, &got); err != nil {
		t.Fatalf("unmarshal entity: %v", err)
	}
	if got.Name != "Alice" || got.Count != 3 {
		t.Fatalf("entity = %+v, want Name=Alice Count=3", got)
	}

	if _, err := client.DeleteEntity(c, "team", "alice", nil); err != nil {
		t.Fatalf("delete entity: %v", err)
	}
	if _, err := client.GetEntity(c, "team", "alice", nil); err == nil {
		t.Fatal("expected error getting deleted entity, got nil")
	}
}

func TestTableUpsertReplaceMerge(t *testing.T) {
	svc := newTableServiceClient(t)
	c := ctx(t)

	if _, err := svc.CreateTable(c, "records", nil); err != nil {
		t.Fatalf("create table: %v", err)
	}
	client := svc.NewClient("records")

	// Upsert with replace semantics creates the entity when missing.
	if _, err := client.UpsertEntity(c, marshalEntity(t, tableEntity{
		PartitionKey: "p1",
		RowKey:       "r1",
		Name:         "first",
		Count:        1,
	}), &aztables.UpsertEntityOptions{UpdateMode: aztables.UpdateModeReplace}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Merge updates Count while preserving Name.
	if _, err := client.UpdateEntity(c, marshalEntity(t, tableEntity{
		PartitionKey: "p1",
		RowKey:       "r1",
		Count:        9,
	}), &aztables.UpdateEntityOptions{UpdateMode: aztables.UpdateModeMerge}); err != nil {
		t.Fatalf("merge update: %v", err)
	}

	resp, err := client.GetEntity(c, "p1", "r1", nil)
	if err != nil {
		t.Fatalf("get entity: %v", err)
	}
	var got tableEntity
	if err := json.Unmarshal(resp.Value, &got); err != nil {
		t.Fatalf("unmarshal entity: %v", err)
	}
	if got.Name != "first" {
		t.Fatalf("Name = %q after merge, want preserved %q", got.Name, "first")
	}
	if got.Count != 9 {
		t.Fatalf("Count = %d after merge, want 9", got.Count)
	}
}

func TestTableListWithFilter(t *testing.T) {
	svc := newTableServiceClient(t)
	c := ctx(t)

	if _, err := svc.CreateTable(c, "items", nil); err != nil {
		t.Fatalf("create table: %v", err)
	}
	client := svc.NewClient("items")

	entities := []tableEntity{
		{PartitionKey: "a", RowKey: "1", Count: 1},
		{PartitionKey: "a", RowKey: "2", Count: 2},
		{PartitionKey: "b", RowKey: "1", Count: 3},
	}
	for _, e := range entities {
		if _, err := client.AddEntity(c, marshalEntity(t, e), nil); err != nil {
			t.Fatalf("add entity %s/%s: %v", e.PartitionKey, e.RowKey, err)
		}
	}

	pager := client.NewListEntitiesPager(&aztables.ListEntitiesOptions{
		Filter: ptr("PartitionKey eq 'a'"),
	})
	var count int
	for pager.More() {
		page, err := pager.NextPage(c)
		if err != nil {
			t.Fatalf("list page: %v", err)
		}
		count += len(page.Entities)
	}
	if count != 2 {
		t.Fatalf("filtered entities = %d, want 2", count)
	}
}

func TestTableListTables(t *testing.T) {
	svc := newTableServiceClient(t)
	c := ctx(t)

	for _, name := range []string{"alpha", "beta"} {
		if _, err := svc.CreateTable(c, name, nil); err != nil {
			t.Fatalf("create table %s: %v", name, err)
		}
	}

	pager := svc.NewListTablesPager(nil)
	found := map[string]bool{}
	for pager.More() {
		page, err := pager.NextPage(c)
		if err != nil {
			t.Fatalf("list tables page: %v", err)
		}
		for _, tbl := range page.Tables {
			if tbl.Name != nil {
				found[*tbl.Name] = true
			}
		}
	}
	if !found["alpha"] || !found["beta"] {
		t.Fatalf("listed tables = %v, want alpha and beta", found)
	}
}
