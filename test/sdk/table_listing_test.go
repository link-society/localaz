package sdk

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
)

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
