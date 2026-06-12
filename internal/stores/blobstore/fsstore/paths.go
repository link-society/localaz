package fsstore

import (
	"encoding/base64"
	"path/filepath"
)

// key encodes a blob name into a filesystem-safe filename.
func key(name string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(name))
}

func (s *Store) accountDir(acct string) string { return filepath.Join(s.root, acct) }

func (s *Store) containerDir(acct, c string) string {
	return filepath.Join(s.accountDir(acct), c)
}

func (s *Store) blobDataPath(acct, c, name string) string {
	return filepath.Join(s.containerDir(acct, c), dataDir, key(name))
}

func (s *Store) blobMetaPath(acct, c, name string) string {
	return filepath.Join(s.containerDir(acct, c), metaDir, key(name)+".json")
}

// blockDir is the per-blob staging directory holding uncommitted blocks.
func (s *Store) blockDir(acct, c, name string) string {
	return filepath.Join(s.containerDir(acct, c), blocksDir, key(name))
}

// blockPath is the on-disk file for a single staged block. The block id is
// encoded so arbitrary client-supplied ids map to safe filenames.
func (s *Store) blockPath(acct, c, name, blockID string) string {
	return filepath.Join(s.blockDir(acct, c, name), key(blockID))
}
