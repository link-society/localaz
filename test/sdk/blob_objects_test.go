package sdk

import (
	"bytes"
	"io"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
)

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
	if got := metadataValue(props.Metadata, "author"); got != "link" {
		t.Fatalf("metadata mismatch: %#v", props.Metadata)
	}
	if props.ContentLength == nil || *props.ContentLength != int64(len("payload")) {
		t.Fatalf("content length mismatch: %v", props.ContentLength)
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
