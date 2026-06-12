package fsstore_test

import (
	"bytes"
	"io"
	"testing"

	"localaz.dev/internal/stores/blobstore"
	"localaz.dev/internal/stores/blobstore/fsstore"
)

// TestBlobSurvivesRestart writes a container and blob, then constructs a brand
// new store over the same directory and asserts the prior state is read back.
func TestBlobSurvivesRestart(t *testing.T) {
	root := t.TempDir()

	s1, err := fsstore.New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := s1.CreateContainer("acct", "cont", map[string]string{"k": "v"}); err != nil {
		t.Fatalf("CreateContainer: %v", err)
	}
	want := []byte("persisted blob payload")
	if _, err := s1.PutBlob("acct", "cont", "dir/blob.txt", bytes.NewReader(want), blobstore.BlobProps{ContentType: "text/plain"}); err != nil {
		t.Fatalf("PutBlob: %v", err)
	}

	s2, err := fsstore.New(root)
	if err != nil {
		t.Fatalf("reopen New: %v", err)
	}
	if _, err := s2.GetContainer("acct", "cont"); err != nil {
		t.Fatalf("GetContainer after restart: %v", err)
	}
	rc, info, err := s2.GetBlob("acct", "cont", "dir/blob.txt")
	if err != nil {
		t.Fatalf("GetBlob after restart: %v", err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read blob after restart: %v", err)
	}
	if !bytes.Equal(data, want) {
		t.Fatalf("blob data = %q, want %q", data, want)
	}
	if info.Props.ContentType != "text/plain" {
		t.Fatalf("content type = %q, want text/plain", info.Props.ContentType)
	}
}
