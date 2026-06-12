package fsstore

import (
	"bytes"
	"crypto/md5"
	"fmt"
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

// TestListBlobsPagination walks a container in maxResults-sized pages using the
// returned nextMarker as an opaque continuation token, and asserts the union of
// the pages equals the full set exactly once, in lexicographic order. This
// guards the pagination fix: with the pre-fix code (which ignored maxResults and
// returned everything with an empty nextMarker) the first page would carry all
// five blobs and the overlap/order assertions below would fail.
func TestListBlobsPagination(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	const acct, cname = "devstoreaccount1", "data"
	if _, err := s.CreateContainer(acct, cname, nil); err != nil {
		t.Fatalf("create container: %v", err)
	}

	// Insert out of order to prove the store returns lexicographic order.
	all := []string{"e.txt", "b.txt", "d.txt", "a.txt", "c.txt"}
	for _, name := range all {
		if _, err := s.PutBlob(acct, cname, name, bytes.NewReader([]byte(name)), blobstore.BlobProps{}); err != nil {
			t.Fatalf("put %s: %v", name, err)
		}
	}
	want := []string{"a.txt", "b.txt", "c.txt", "d.txt", "e.txt"}

	var got []string
	marker := ""
	pages := 0
	for {
		blobs, _, next, err := s.ListBlobs(acct, cname, "", "", 2, marker)
		if err != nil {
			t.Fatalf("list blobs (marker %q): %v", marker, err)
		}
		pages++
		if pages > len(all)+1 {
			t.Fatalf("pagination did not terminate: too many pages")
		}
		if len(blobs) > 2 {
			t.Fatalf("page returned %d blobs, want <= maxResults 2", len(blobs))
		}
		for _, b := range blobs {
			got = append(got, b.Name)
		}
		if next == "" {
			break
		}
		marker = next
	}

	if pages < 2 {
		t.Fatalf("expected multiple pages, got %d", pages)
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("paginated union mismatch: got %v want %v", got, want)
	}
}
