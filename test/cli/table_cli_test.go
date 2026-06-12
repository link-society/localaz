//go:build cli

package cli

import (
	"fmt"
	"testing"
)

func TestTableLifecycle(t *testing.T) {
	requireTable(t)
	name := uniqueName("clitable")

	az(t, "storage", "table", "create", "--name", name, "--output", "none")

	if got := az(t, "storage", "table", "exists", "--name", name, "--query", "exists", "-o", "tsv"); got != "true" {
		t.Fatalf("table exists = %q, want true", got)
	}
	if got := az(t, "storage", "table", "list", "--query", fmt.Sprintf("[?name=='%s'] | length(@)", name), "-o", "tsv"); got != "1" {
		t.Fatalf("table listing count = %q, want 1", got)
	}

	az(t, "storage", "table", "delete", "--name", name, "--output", "none")
}

func TestTableEntityRoundTrip(t *testing.T) {
	requireTable(t)
	name := uniqueName("clientity")
	az(t, "storage", "table", "create", "--name", name, "--output", "none")
	t.Cleanup(func() {
		az(t, "storage", "table", "delete", "--name", name, "--output", "none")
	})

	az(t, "storage", "entity", "insert", "--table-name", name,
		"--entity", "PartitionKey=team", "RowKey=alice", "Name=Alice", "--output", "none")

	if got := az(t, "storage", "entity", "show", "--table-name", name,
		"--partition-key", "team", "--row-key", "alice", "--query", "Name", "-o", "tsv"); got != "Alice" {
		t.Fatalf("entity Name = %q, want Alice", got)
	}

	if got := az(t, "storage", "entity", "query", "--table-name", name,
		"--query", "items[?RowKey=='alice'] | length(@)", "-o", "tsv"); got != "1" {
		t.Fatalf("entity query count = %q, want 1", got)
	}

	az(t, "storage", "entity", "delete", "--table-name", name,
		"--partition-key", "team", "--row-key", "alice", "--output", "none")
}
