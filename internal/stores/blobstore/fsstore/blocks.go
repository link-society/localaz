package fsstore

import (
	"io"
	"os"
	"path/filepath"

	"localaz.dev/internal/stores/blobstore"
)

func (s *Store) StageBlock(acct, cname, name, blockID string, data io.Reader) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.lookupContainer(acct, cname)
	if err != nil {
		return err
	}
	if _, ok := c.blobs[name]; !ok {
		// Staging is allowed before the blob is committed; track a placeholder.
		// Placeholder entries are not persisted until committed.
		c.blobs[name] = &blobEntry{
			info: blobstore.BlobInfo{Name: name, ContainerName: cname, BlobType: "BlockBlob"},
		}
	}
	// Stream the staged block to its own file on disk, never buffering it.
	if err := os.MkdirAll(s.blockDir(acct, cname, name), 0o755); err != nil {
		return err
	}
	if _, _, err := streamToFile(s.blockPath(acct, cname, name, blockID), data); err != nil {
		return err
	}
	return nil
}

func (s *Store) CommitBlockList(acct, cname, name string, blockIDs []string, props blobstore.BlobProps) (blobstore.BlobInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.lookupContainer(acct, cname)
	if err != nil {
		return blobstore.BlobInfo{}, err
	}
	if _, ok := c.blobs[name]; !ok {
		return blobstore.BlobInfo{}, blobstore.ErrInvalidBlockList
	}
	// Validate every referenced block exists before assembling anything.
	for _, id := range blockIDs {
		if _, err := os.Stat(s.blockPath(acct, cname, name, id)); err != nil {
			return blobstore.BlobInfo{}, blobstore.ErrInvalidBlockList
		}
	}
	// Assemble the destination by streaming the staged block files in order into
	// a temp file, then placing it atomically — never concatenating in memory.
	if err := os.MkdirAll(filepath.Join(s.containerDir(acct, cname), dataDir), 0o755); err != nil {
		return blobstore.BlobInfo{}, err
	}
	n, err := s.assembleBlocks(acct, cname, name, blockIDs)
	if err != nil {
		return blobstore.BlobInfo{}, err
	}
	info, err := s.commitWrittenBlobLocked(acct, c, name, n, props)
	if err != nil {
		return blobstore.BlobInfo{}, err
	}
	// Staged blocks are consumed on commit.
	_ = os.RemoveAll(s.blockDir(acct, cname, name))
	return info, nil
}

// assembleBlocks concatenates the named staged block files, in order, into the
// blob's data file using a temp-file-then-rename, returning the byte count.
func (s *Store) assembleBlocks(acct, cname, name string, blockIDs []string) (int64, error) {
	dst := s.blobDataPath(acct, cname, name)
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".tmp-*")
	if err != nil {
		return 0, err
	}
	tmpName := tmp.Name()
	var total int64
	for _, id := range blockIDs {
		src, err := os.Open(s.blockPath(acct, cname, name, id))
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
			return 0, err
		}
		n, err := io.Copy(tmp, src)
		_ = src.Close()
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
			return 0, err
		}
		total += n
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return 0, err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		_ = os.Remove(tmpName)
		return 0, err
	}
	return total, nil
}
