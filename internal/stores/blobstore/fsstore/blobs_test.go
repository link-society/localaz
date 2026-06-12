package fsstore

import (
	"bytes"
	"crypto/md5"
	"io"
	"testing"

	"localaz.dev/internal/stores/blobstore"
)

// TestPutGetBlobLargeRoundTrip streams an 8 MiB blob through the store and
// verifies the bytes, the reported content length and the stored MD5 are all
// correct — the data path never buffers the whole payload in memory.
func TestPutGetBlobLargeRoundTrip(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	const acct, cname, name = "devstoreaccount1", "data", "big.bin"
	if _, err := s.CreateContainer(acct, cname, nil); err != nil {
		t.Fatalf("create container: %v", err)
	}

	payload := bytes.Repeat([]byte("localaz!"), 1<<20) // 8 MiB
	want := md5.Sum(payload)

	info, err := s.PutBlob(acct, cname, name, bytes.NewReader(payload), blobstore.BlobProps{
		ContentType: "application/octet-stream",
	})
	if err != nil {
		t.Fatalf("put blob: %v", err)
	}
	if info.ContentLength != int64(len(payload)) {
		t.Fatalf("content length: got %d want %d", info.ContentLength, len(payload))
	}
	if !bytes.Equal(info.Props.ContentMD5, want[:]) {
		t.Fatalf("stored md5 mismatch: got %x want %x", info.Props.ContentMD5, want)
	}

	rc, getInfo, err := s.GetBlob(acct, cname, name)
	if err != nil {
		t.Fatalf("get blob: %v", err)
	}
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read blob: %v", err)
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("close blob reader: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload mismatch: got %d bytes want %d bytes", len(got), len(payload))
	}
	if getInfo.ContentLength != int64(len(payload)) {
		t.Fatalf("get content length: got %d want %d", getInfo.ContentLength, len(payload))
	}
}
