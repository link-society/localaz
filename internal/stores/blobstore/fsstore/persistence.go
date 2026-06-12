package fsstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"localaz.dev/internal/stores/blobstore"
)

// load rebuilds the in-memory index from whatever state is persisted under the
// store root.
func (s *Store) load() error {
	accts, err := os.ReadDir(s.root)
	if err != nil {
		return fmt.Errorf("fsstore: read root: %w", err)
	}
	for _, ae := range accts {
		if !ae.IsDir() {
			continue
		}
		acct := &account{containers: map[string]*container{}}
		s.accounts[ae.Name()] = acct
		conts, err := os.ReadDir(s.accountDir(ae.Name()))
		if err != nil {
			return err
		}
		for _, ce := range conts {
			if !ce.IsDir() {
				continue
			}
			c, err := s.loadContainer(ae.Name(), ce.Name())
			if err != nil {
				return err
			}
			acct.containers[ce.Name()] = c
		}
	}
	return nil
}

func (s *Store) loadContainer(acct, name string) (*container, error) {
	c := &container{blobs: map[string]*blobEntry{}}
	metaPath := filepath.Join(s.containerDir(acct, name), containerMetaFile)
	raw, err := os.ReadFile(metaPath)
	if err != nil {
		// A directory without container metadata is not a valid container.
		return nil, fmt.Errorf("fsstore: read container meta %q: %w", name, err)
	}
	if err := json.Unmarshal(raw, &c.info); err != nil {
		return nil, err
	}
	metaFiles, err := os.ReadDir(filepath.Join(s.containerDir(acct, name), metaDir))
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return nil, err
	}
	for _, mf := range metaFiles {
		if mf.IsDir() || !strings.HasSuffix(mf.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(s.containerDir(acct, name), metaDir, mf.Name()))
		if err != nil {
			return nil, err
		}
		var info blobstore.BlobInfo
		if err := json.Unmarshal(raw, &info); err != nil {
			return nil, err
		}
		c.blobs[info.Name] = &blobEntry{info: info, blocks: map[string][]byte{}}
	}
	return c, nil
}

func (s *Store) persistContainer(acct string, c *container) error {
	dir := s.containerDir(acct, c.info.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(c.info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, containerMetaFile), raw, 0o644)
}

func (s *Store) persistBlob(acct, cname string, b *blobEntry, data []byte) error {
	cdir := s.containerDir(acct, cname)
	for _, d := range []string{dataDir, metaDir} {
		if err := os.MkdirAll(filepath.Join(cdir, d), 0o755); err != nil {
			return err
		}
	}
	if data != nil {
		if err := os.WriteFile(s.blobDataPath(acct, cname, b.info.Name), data, 0o644); err != nil {
			return err
		}
	}
	raw, err := json.MarshalIndent(b.info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.blobMetaPath(acct, cname, b.info.Name), raw, 0o644)
}
