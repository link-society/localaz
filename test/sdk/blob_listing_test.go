package sdk

import (
	"fmt"
	"sort"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
)

func TestListBlobsFlat(t *testing.T) {
	client := newClient(t)
	c := ctx(t)
	const containerName = "flat"
	if _, err := client.CreateContainer(c, containerName, nil); err != nil {
		t.Fatalf("create container: %v", err)
	}
	want := []string{"a.txt", "b.txt", "c.txt"}
	for _, name := range want {
		if _, err := client.UploadBuffer(c, containerName, name, []byte(name), nil); err != nil {
			t.Fatalf("upload %s: %v", name, err)
		}
	}

	var got []string
	pager := client.NewListBlobsFlatPager(containerName, nil)
	for pager.More() {
		page, err := pager.NextPage(c)
		if err != nil {
			t.Fatalf("list blobs: %v", err)
		}
		for _, item := range page.Segment.BlobItems {
			got = append(got, *item.Name)
		}
	}
	sort.Strings(got)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("blob listing mismatch: got %v want %v", got, want)
	}
}

// TestListBlobsFlatPaged drives the ListBlobsFlat pager with a small page size
// (MaxResults 2) over several blobs and asserts the pager walks ALL of them
// across multiple pages. Against the pre-fix server (which ignored maxresults
// and returned every blob in a single page with an empty NextMarker) the pager
// stops after one page, so this either over-counts duplicates or — more
// importantly — never exercises >1 page; the multi-page assertion fails.
func TestListBlobsFlatPaged(t *testing.T) {
	client := newClient(t)
	c := ctx(t)
	const containerName = "paged"
	if _, err := client.CreateContainer(c, containerName, nil); err != nil {
		t.Fatalf("create container: %v", err)
	}
	want := []string{"a.txt", "b.txt", "c.txt", "d.txt", "e.txt"}
	for _, name := range want {
		if _, err := client.UploadBuffer(c, containerName, name, []byte(name), nil); err != nil {
			t.Fatalf("upload %s: %v", name, err)
		}
	}

	containerClient := client.ServiceClient().NewContainerClient(containerName)
	maxResults := int32(2)
	pager := containerClient.NewListBlobsFlatPager(&container.ListBlobsFlatOptions{
		MaxResults: &maxResults,
	})

	var got []string
	pages := 0
	for pager.More() {
		page, err := pager.NextPage(c)
		if err != nil {
			t.Fatalf("list blobs page %d: %v", pages, err)
		}
		pages++
		if len(page.Segment.BlobItems) > int(maxResults) {
			t.Fatalf("page %d returned %d blobs, want <= %d",
				pages, len(page.Segment.BlobItems), maxResults)
		}
		for _, item := range page.Segment.BlobItems {
			got = append(got, *item.Name)
		}
	}

	if pages < 2 {
		t.Fatalf("expected the pager to walk multiple pages, got %d", pages)
	}
	sort.Strings(got)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("paged listing mismatch: got %v want %v", got, want)
	}
}

func TestListBlobsHierarchy(t *testing.T) {
	client := newClient(t)
	c := ctx(t)
	const containerName = "hier"
	if _, err := client.CreateContainer(c, containerName, nil); err != nil {
		t.Fatalf("create container: %v", err)
	}
	names := []string{"root.txt", "dir1/a.txt", "dir1/b.txt", "dir2/c.txt"}
	for _, name := range names {
		if _, err := client.UploadBuffer(c, containerName, name, []byte(name), nil); err != nil {
			t.Fatalf("upload %s: %v", name, err)
		}
	}

	containerClient := client.ServiceClient().NewContainerClient(containerName)
	pager := containerClient.NewListBlobsHierarchyPager("/", &container.ListBlobsHierarchyOptions{})

	var blobs, prefixes []string
	for pager.More() {
		page, err := pager.NextPage(c)
		if err != nil {
			t.Fatalf("list hierarchy: %v", err)
		}
		for _, item := range page.Segment.BlobItems {
			blobs = append(blobs, *item.Name)
		}
		for _, p := range page.Segment.BlobPrefixes {
			prefixes = append(prefixes, *p.Name)
		}
	}
	sort.Strings(blobs)
	sort.Strings(prefixes)

	if fmt.Sprint(blobs) != fmt.Sprint([]string{"root.txt"}) {
		t.Fatalf("top-level blobs mismatch: %v", blobs)
	}
	if fmt.Sprint(prefixes) != fmt.Sprint([]string{"dir1/", "dir2/"}) {
		t.Fatalf("prefixes mismatch: %v", prefixes)
	}
}
