package sdk

import (
	"sort"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
)

// TestListContainersPaged drives the service client's NewListContainersPager
// with a small page size over several containers and asserts the pager walks
// the whole set across multiple pages. Against the pre-fix code (which ignored
// maxresults and returned everything in one page with an empty NextMarker) the
// pager stops after the first page, so the multi-page assertion fails.
func TestListContainersPaged(t *testing.T) {
	client := newClient(t)
	c := ctx(t)

	want := []string{"alpha", "bravo", "charlie", "delta", "echo"}
	for _, name := range want {
		if _, err := client.CreateContainer(c, name, nil); err != nil {
			t.Fatalf("create container %s: %v", name, err)
		}
	}

	svc := client.ServiceClient()
	pager := svc.NewListContainersPager(&service.ListContainersOptions{
		MaxResults: to.Ptr(int32(2)),
	})

	var got []string
	pages := 0
	for pager.More() {
		page, err := pager.NextPage(c)
		if err != nil {
			t.Fatalf("list containers page %d: %v", pages, err)
		}
		pages++
		if len(page.ContainerItems) > 2 {
			t.Fatalf("page %d exceeded MaxResults: %d items", pages, len(page.ContainerItems))
		}
		for _, item := range page.ContainerItems {
			if item.Name != nil {
				got = append(got, *item.Name)
			}
		}
	}

	if pages < 2 {
		t.Fatalf("expected the listing to span multiple pages, got %d page(s) for %d containers", pages, len(want))
	}

	sort.Strings(got)
	if len(got) != len(want) {
		t.Fatalf("paged listing size: got %d (%v) want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("paged listing mismatch at %d: got %q want %q (full %v)", i, got[i], want[i], got)
		}
	}
}
