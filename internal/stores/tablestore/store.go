// Package tablestore is an in-memory implementation of the Azure Table service
// state, with write-through JSON persistence so a mounted data volume survives
// restarts (matching the Blob service's promise).
//
// The on-disk layout is a single snapshot file:
//
//	<root>/table/tables.json
//
// The HTTP layer (internal/tableserver) depends only on this concrete store.
package tablestore

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"localaz.dev/internal/utils/atomicfile"
)

// Store holds every account's tables. It is safe for concurrent use.
type Store struct {
	mu       sync.Mutex
	path     string
	accounts map[string]map[string]*table
}

// New creates a Store persisted under root, loading any prior snapshot.
func New(root string) (*Store, error) {
	dir := filepath.Join(root, "table")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("tablestore: create dir: %w", err)
	}
	s := &Store{
		path:     filepath.Join(dir, "tables.json"),
		accounts: map[string]map[string]*table{},
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("tablestore: read snapshot: %w", err)
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

// lookupLocked returns the named table or ErrTableNotFound. Caller holds s.mu.
func (s *Store) lookupLocked(account, name string) (*table, error) {
	ts, ok := s.accounts[account]
	if !ok {
		return nil, ErrTableNotFound
	}
	t, ok := ts[name]
	if !ok {
		return nil, ErrTableNotFound
	}
	return t, nil
}

// etagDatetimeFmt is the datetime layout embedded in the entity ETag. It uses
// the fixed seven-digit fractional form ('0' digits) so trailing zeros are not
// dropped, keeping the ETag consistent with the rendered Timestamp.
const etagDatetimeFmt = "2006-01-02T15:04:05.0000000Z"

// etagFor builds an Azure-style weak ETag from a timestamp. The value is opaque
// to clients and used for If-Match concurrency.
func etagFor(ts time.Time) string {
	return fmt.Sprintf("W/\"datetime'%s'\"", url.QueryEscape(ts.UTC().Format(etagDatetimeFmt)))
}
