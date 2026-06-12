package sdk

import (
	"encoding/json"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
)

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
