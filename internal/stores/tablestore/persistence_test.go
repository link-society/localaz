package tablestore_test

import (
	"encoding/json"
	"testing"

	"localaz.dev/internal/stores/tablestore"
)

// TestEntitySurvivesRestart inserts an entity, then opens a brand new store over
// the same directory and asserts the table and entity are read back.
func TestEntitySurvivesRestart(t *testing.T) {
	root := t.TempDir()

	s1, err := tablestore.New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := s1.CreateTable("acct", "tbl"); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	props := map[string]json.RawMessage{"Name": json.RawMessage(`"alice"`)}
	if _, err := s1.InsertEntity("acct", "tbl", "pk1", "rk1", props); err != nil {
		t.Fatalf("InsertEntity: %v", err)
	}

	s2, err := tablestore.New(root)
	if err != nil {
		t.Fatalf("reopen New: %v", err)
	}
	e, err := s2.GetEntity("acct", "tbl", "pk1", "rk1")
	if err != nil {
		t.Fatalf("GetEntity after restart: %v", err)
	}
	if e.PartitionKey != "pk1" || e.RowKey != "rk1" {
		t.Fatalf("keys = %q/%q, want pk1/rk1", e.PartitionKey, e.RowKey)
	}
	if string(e.Props["Name"]) != `"alice"` {
		t.Fatalf("Name prop = %s, want \"alice\"", e.Props["Name"])
	}
}
