// Package queuestore is an in-memory implementation of the Azure Queue service
// state, with write-through JSON persistence so a mounted data volume survives
// restarts (matching the Blob service's promise).
//
// The on-disk layout is a single snapshot file:
//
//	<root>/queue/queues.json
//
// The HTTP layer (internal/queueserver) depends only on this concrete store.
package queuestore

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"localaz.dev/internal/utils/atomicfile"
)

// Store holds every account's queues. It is safe for concurrent use.
type Store struct {
	mu       sync.Mutex
	path     string
	accounts map[string]map[string]*queue
}

// New creates a Store persisted under root, loading any prior snapshot.
func New(root string) (*Store, error) {
	dir := filepath.Join(root, "queue")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("queuestore: create dir: %w", err)
	}
	s := &Store{
		path:     filepath.Join(dir, "queues.json"),
		accounts: map[string]map[string]*queue{},
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// load reads the snapshot file into memory, tolerating a missing file.
func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("queuestore: read snapshot: %w", err)
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, &s.accounts)
}

// persistLocked writes the current state crash-safely (temp file + fsync +
// rename + parent-dir fsync via atomicfile). The caller must hold s.mu. Errors
// are best-effort: durability, not error propagation, is the goal here.
func (s *Store) persistLocked() {
	data, err := json.Marshal(s.accounts)
	if err != nil {
		return
	}
	_ = atomicfile.Write(s.path, data, 0o644)
}

// lookup returns the named queue or ErrQueueNotFound, evicting expired messages
// as a side effect. The caller must hold s.mu.
func (s *Store) lookupLocked(account, name string) (*queue, error) {
	qs, ok := s.accounts[account]
	if !ok {
		return nil, ErrQueueNotFound
	}
	q, ok := qs[name]
	if !ok {
		return nil, ErrQueueNotFound
	}
	q.evictExpired(time.Now().UTC())
	return q, nil
}

// evictExpired drops messages past their TTL.
func (q *queue) evictExpired(now time.Time) {
	kept := q.Messages[:0]
	for _, m := range q.Messages {
		if !m.expired(now) {
			kept = append(kept, m)
		}
	}
	q.Messages = kept
}

// newID returns a random RFC 4122 v4 UUID string for a message id.
func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// newPopReceipt returns an opaque, URL-safe pop receipt token.
func newPopReceipt() string {
	var b [24]byte
	_, _ = rand.Read(b[:])
	return base64.StdEncoding.EncodeToString(b[:])
}
