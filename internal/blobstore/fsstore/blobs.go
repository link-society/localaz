package fsstore

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"localaz.dev/internal/blobstore"
)

func (s *Store) PutBlob(acct, cname, name string, data []byte, props blobstore.BlobProps) (blobstore.BlobInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.lookupContainer(acct, cname)
	if err != nil {
		return blobstore.BlobInfo{}, err
	}
	return s.writeBlobLocked(acct, c, name, data, props)
}

// writeBlobLocked persists a full blob payload. The caller must hold s.mu.
func (s *Store) writeBlobLocked(acct string, c *container, name string, data []byte, props blobstore.BlobProps) (blobstore.BlobInfo, error) {
	now := time.Now().UTC()
	props.Metadata = cloneMeta(props.Metadata)
	info := blobstore.BlobInfo{
		Name:          name,
		ContainerName: c.info.Name,
		ContentLength: int64(len(data)),
		ETag:          newETag(now),
		LastModified:  now,
		BlobType:      "BlockBlob",
		Props:         props,
	}
	b := &blobEntry{info: info, blocks: map[string][]byte{}}
	if err := s.persistBlob(acct, c.info.Name, b, data); err != nil {
		return blobstore.BlobInfo{}, err
	}
	c.blobs[name] = b
	return info, nil
}

func (s *Store) GetBlob(acct, cname, name string) ([]byte, blobstore.BlobInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, err := s.lookupContainer(acct, cname)
	if err != nil {
		return nil, blobstore.BlobInfo{}, err
	}
	b, ok := c.blobs[name]
	if !ok {
		return nil, blobstore.BlobInfo{}, blobstore.ErrBlobNotFound
	}
	data, err := os.ReadFile(s.blobDataPath(acct, cname, name))
	if err != nil {
		return nil, blobstore.BlobInfo{}, err
	}
	info := b.info
	info.Props.Metadata = cloneMeta(b.info.Props.Metadata)
	return data, info, nil
}

func (s *Store) StatBlob(acct, cname, name string) (blobstore.BlobInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, err := s.lookupContainer(acct, cname)
	if err != nil {
		return blobstore.BlobInfo{}, err
	}
	b, ok := c.blobs[name]
	if !ok {
		return blobstore.BlobInfo{}, blobstore.ErrBlobNotFound
	}
	info := b.info
	info.Props.Metadata = cloneMeta(b.info.Props.Metadata)
	return info, nil
}

func (s *Store) DeleteBlob(acct, cname, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.lookupContainer(acct, cname)
	if err != nil {
		return err
	}
	if _, ok := c.blobs[name]; !ok {
		return blobstore.ErrBlobNotFound
	}
	_ = os.Remove(s.blobDataPath(acct, cname, name))
	_ = os.Remove(s.blobMetaPath(acct, cname, name))
	_ = os.RemoveAll(filepath.Join(s.containerDir(acct, cname), blocksDir, key(name)))
	delete(c.blobs, name)
	return nil
}

func (s *Store) ListBlobs(acct, cname, prefix, delimiter string) ([]blobstore.BlobInfo, []string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, err := s.lookupContainer(acct, cname)
	if err != nil {
		return nil, nil, err
	}
	var names []string
	for name := range c.blobs {
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	var blobs []blobstore.BlobInfo
	prefixSet := map[string]struct{}{}
	var prefixes []string
	for _, name := range names {
		if delimiter != "" {
			rest := name[len(prefix):]
			if idx := strings.Index(rest, delimiter); idx >= 0 {
				vp := prefix + rest[:idx+len(delimiter)]
				if _, seen := prefixSet[vp]; !seen {
					prefixSet[vp] = struct{}{}
					prefixes = append(prefixes, vp)
				}
				continue
			}
		}
		info := c.blobs[name].info
		info.Props.Metadata = cloneMeta(c.blobs[name].info.Props.Metadata)
		blobs = append(blobs, info)
	}
	sort.Strings(prefixes)
	return blobs, prefixes, nil
}
