package fsstore

import (
	"testing"

	"localaz.dev/internal/stores/blobstore"
)

// TestListContainersPagination walks the container set in fixed-size pages using
// the opaque NextMarker continuation token. It asserts that the union of all
// pages equals the full set, with no overlaps and in lexicographic order, and
// that a malformed marker restarts the listing from the beginning.
func TestListContainersPagination(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	const acct = "devstoreaccount1"
	all := []string{"alpha", "bravo", "charlie", "delta", "echo"}
	for _, name := range all {
		if _, err := s.CreateContainer(acct, name, nil); err != nil {
			t.Fatalf("create container %s: %v", name, err)
		}
	}

	// Page through the set two at a time.
	var got []string
	marker := ""
	pages := 0
	for {
		items, next, err := s.ListContainers(acct, "", 2, marker)
		if err != nil {
			t.Fatalf("list containers (marker %q): %v", marker, err)
		}
		pages++
		if pages > len(all)+1 {
			t.Fatalf("pagination did not terminate; got %d pages", pages)
		}
		if len(items) > 2 {
			t.Fatalf("page exceeded maxResults: got %d items", len(items))
		}
		for _, it := range items {
			got = append(got, it.Name)
		}
		if next == "" {
			if len(items) == 0 && pages == 1 {
				t.Fatal("empty first page")
			}
			break
		}
		if len(items) != 2 {
			t.Fatalf("non-final page should be full: got %d items, next %q", len(items), next)
		}
		marker = next
	}

	// The union across all pages must equal the full set, in order, no overlaps.
	if len(got) != len(all) {
		t.Fatalf("paged union size: got %d (%v) want %d (%v)", len(got), got, len(all), all)
	}
	for i := range all {
		if got[i] != all[i] {
			t.Fatalf("paged result %d: got %q want %q (full %v)", i, got[i], all[i], got)
		}
	}

	// A malformed (non-base64url) marker must restart from the beginning.
	items, _, err := s.ListContainers(acct, "", 2, "!!!not-base64!!!")
	if err != nil {
		t.Fatalf("list with malformed marker: %v", err)
	}
	if len(items) != 2 || items[0].Name != all[0] {
		t.Fatalf("malformed marker should restart from beginning: got %v", names(items))
	}
}

func names(items []blobstore.ContainerInfo) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Name
	}
	return out
}
