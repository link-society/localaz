// Package fsstore is a filesystem-backed implementation of blobstore.Store.
//
// Metadata is held in memory for fast listing and lookups, and every mutation
// is written through to disk so that a container restart (with a mounted data
// volume) preserves state. Blob payloads live on disk and are read on demand.
//
// The on-disk layout is:
//
//	<root>/<account>/<container>/_container.json    container metadata
//	<root>/<account>/<container>/data/<key>         blob payload
//	<root>/<account>/<container>/meta/<key>.json    blob metadata
//	<root>/<account>/<container>/blocks/<key>/<id>  staged, uncommitted block
//
// where <key> is the URL-safe base64 encoding of the blob name.
package fsstore

import (
	"fmt"
	"os"
	"sync"

	"localaz.dev/internal/stores/blobstore"
)

// Store is a filesystem-backed blob store. It is safe for concurrent use.
type Store struct {
	mu       sync.RWMutex
	root     string
	accounts map[string]*account
}

// New creates a Store rooted at the given directory, loading any state that was
// previously persisted there.
func New(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("fsstore: create root: %w", err)
	}
	s := &Store{root: root, accounts: map[string]*account{}}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// getAccount returns the account, creating its in-memory record on first use.
func (s *Store) getAccount(name string) *account {
	a, ok := s.accounts[name]
	if !ok {
		a = &account{containers: map[string]*container{}}
		s.accounts[name] = a
	}
	return a
}

// lookupContainer returns the named container or blobstore.ErrContainerNotFound.
func (s *Store) lookupContainer(acct, name string) (*container, error) {
	a, ok := s.accounts[acct]
	if !ok {
		return nil, blobstore.ErrContainerNotFound
	}
	c, ok := a.containers[name]
	if !ok {
		return nil, blobstore.ErrContainerNotFound
	}
	return c, nil
}
