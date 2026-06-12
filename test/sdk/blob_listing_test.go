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
