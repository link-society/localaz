package armstore

import (
	"fmt"
	"testing"
)

// TestPutResourceCap verifies the store refuses to grow past maxResources for
// distinct IDs (guarding against unbounded memory growth from a PUT loop),
// while still allowing in-place replacement of already-stored IDs.
func TestPutResourceCap(t *testing.T) {
	s := New(Config{})

	for i := 0; i < maxResources; i++ {
		id := fmt.Sprintf("/subscriptions/x/resourceGroups/rg/providers/p/t/n%d", i)
		if !s.PutResource(id, map[string]any{"type": "p/t"}) {
			t.Fatalf("PutResource(%d) refused before reaching the cap", i)
		}
	}

	// One more distinct ID must be refused and must not grow the map.
	over := "/subscriptions/x/resourceGroups/rg/providers/p/t/overflow"
	if s.PutResource(over, map[string]any{"type": "p/t"}) {
		t.Fatalf("PutResource accepted a new ID past the cap of %d", maxResources)
	}
	if _, ok := s.GetResource(over); ok {
		t.Fatalf("overflow resource was stored despite the cap")
	}
	if got := s.count(); got != maxResources {
		t.Fatalf("resource count grew past the cap: got %d want %d", got, maxResources)
	}

	// Replacing an existing ID at the cap is still allowed.
	existing := "/subscriptions/x/resourceGroups/rg/providers/p/t/n0"
	if !s.PutResource(existing, map[string]any{"type": "p/t", "v": "2"}) {
		t.Fatalf("PutResource refused to replace an existing ID at the cap")
	}
}

// count returns the number of stored resources (test helper).
func (s *Store) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.resources)
}
