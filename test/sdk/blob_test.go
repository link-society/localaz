// Package sdk contains the integration test suite for the localaz Blob service.
//
// These tests exercise the emulator through the official Azure Go SDK
// (azblob), which is the strongest guarantee that real client applications
// interoperate with localaz: every request is built, signed and parsed by the
// same code a production application would use.
package sdk

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"

	"localaz.dev/internal/blobserver"
	"localaz.dev/internal/blobstore/fsstore"
)

// newClient spins up an in-process emulator backed by a temporary data
// directory and returns an azblob client pointed at it.
func newClient(t *testing.T) *azblob.Client {
	t.Helper()
	store, err := fsstore.New(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	ts := httptest.NewServer(blobserver.New(store))
	t.Cleanup(ts.Close)

	client, err := azblob.NewClientWithNoCredential(ts.URL+"/devstoreaccount1", nil)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	return client
}

func ctx(t *testing.T) context.Context {
	t.Helper()
	return context.Background()
}

// metadataValue looks up a metadata key case-insensitively, returning the empty
// string if it is absent.
func metadataValue(m map[string]*string, key string) string {
	for k, v := range m {
		if strings.EqualFold(k, key) && v != nil {
			return *v
		}
	}
	return ""
}

func TestContainerLifecycle(t *testing.T) {
	client := newClient(t)
	c := ctx(t)

	if _, err := client.CreateContainer(c, "lifecycle", nil); err != nil {
		t.Fatalf("create container: %v", err)
	}

	// Creating the same container again must report the Azure conflict code.
	_, err := client.CreateContainer(c, "lifecycle", nil)
	if !bloberror.HasCode(err, bloberror.ContainerAlreadyExists) {
		t.Fatalf("expected ContainerAlreadyExists, got %v", err)
	}

	// The container must show up in the account listing.
	found := false
	pager := client.NewListContainersPager(nil)
	for pager.More() {
		page, err := pager.NextPage(c)
		if err != nil {
			t.Fatalf("list containers: %v", err)
		}
		for _, item := range page.ContainerItems {
			if item.Name != nil && *item.Name == "lifecycle" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("created container not present in listing")
	}

	if _, err := client.DeleteContainer(c, "lifecycle", nil); err != nil {
		t.Fatalf("delete container: %v", err)
	}
}

func TestBlobUploadDownload(t *testing.T) {
	client := newClient(t)
	c := ctx(t)
	const containerName, blobName = "data", "hello.txt"
	payload := []byte("hello, localaz!")

	if _, err := client.CreateContainer(c, containerName, nil); err != nil {
		t.Fatalf("create container: %v", err)
	}
	_, err := client.UploadBuffer(c, containerName, blobName, payload, &azblob.UploadBufferOptions{
		HTTPHeaders: &blob.HTTPHeaders{BlobContentType: to.Ptr("text/plain")},
	})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}

	resp, err := client.DownloadStream(c, containerName, blobName, nil)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	_ = resp.Body.Close()
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload mismatch: got %q want %q", got, payload)
	}
	if resp.ContentType == nil || *resp.ContentType != "text/plain" {
		t.Fatalf("content type mismatch: %v", resp.ContentType)
	}
	if resp.ContentLength == nil || *resp.ContentLength != int64(len(payload)) {
		t.Fatalf("content length mismatch: %v", resp.ContentLength)
	}

	if _, err := client.DeleteBlob(c, containerName, blobName, nil); err != nil {
		t.Fatalf("delete blob: %v", err)
	}
	_, err = client.DownloadStream(c, containerName, blobName, nil)
	if !bloberror.HasCode(err, bloberror.BlobNotFound) {
		t.Fatalf("expected BlobNotFound after delete, got %v", err)
	}
}

func TestBlobMetadataAndProperties(t *testing.T) {
	client := newClient(t)
	c := ctx(t)
	const containerName, blobName = "meta", "obj"
	if _, err := client.CreateContainer(c, containerName, nil); err != nil {
		t.Fatalf("create container: %v", err)
	}
	_, err := client.UploadBuffer(c, containerName, blobName, []byte("payload"), &azblob.UploadBufferOptions{
		Metadata: map[string]*string{"author": to.Ptr("link")},
	})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}

	blobClient := client.ServiceClient().NewContainerClient(containerName).NewBlobClient(blobName)
	props, err := blobClient.GetProperties(c, nil)
	if err != nil {
		t.Fatalf("get properties: %v", err)
	}
	// Azure treats metadata keys case-insensitively, and HTTP header
	// canonicalization may alter the case returned to the client, so look the
	// value up case-insensitively.
	if got := metadataValue(props.Metadata, "author"); got != "link" {
		t.Fatalf("metadata mismatch: %#v", props.Metadata)
	}
	if props.ContentLength == nil || *props.ContentLength != int64(len("payload")) {
		t.Fatalf("content length mismatch: %v", props.ContentLength)
	}
}

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

func TestLargeBlobBlockUpload(t *testing.T) {
	client := newClient(t)
	c := ctx(t)
	const containerName, blobName = "blocks", "big.bin"
	if _, err := client.CreateContainer(c, containerName, nil); err != nil {
		t.Fatalf("create container: %v", err)
	}

	// Force the staged block upload path with a small block size.
	payload := bytes.Repeat([]byte("localaz-"), 1<<16) // 512 KiB
	_, err := client.UploadBuffer(c, containerName, blobName, payload, &azblob.UploadBufferOptions{
		BlockSize:   64 * 1024,
		Concurrency: 4,
	})
	if err != nil {
		t.Fatalf("block upload: %v", err)
	}

	resp, err := client.DownloadStream(c, containerName, blobName, nil)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	_ = resp.Body.Close()
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload mismatch: got %d bytes want %d bytes", len(got), len(payload))
	}
}

func TestOperationsOnMissingContainer(t *testing.T) {
	client := newClient(t)
	c := ctx(t)
	_, err := client.UploadBuffer(c, "ghost", "x", []byte("y"), nil)
	if !bloberror.HasCode(err, bloberror.ContainerNotFound) {
		t.Fatalf("expected ContainerNotFound, got %v", err)
	}
}
