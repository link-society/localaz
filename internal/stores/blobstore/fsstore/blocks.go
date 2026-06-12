package fsstore

import "localaz.dev/internal/stores/blobstore"

func (s *Store) StageBlock(acct, cname, name, blockID string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.lookupContainer(acct, cname)
	if err != nil {
		return err
	}
	b, ok := c.blobs[name]
	if !ok {
		// Staging is allowed before the blob is committed; track a placeholder.
		// Placeholder entries are not persisted until committed.
		b = &blobEntry{
			info:   blobstore.BlobInfo{Name: name, ContainerName: cname, BlobType: "BlockBlob"},
			blocks: map[string][]byte{},
		}
		c.blobs[name] = b
	}
	if b.blocks == nil {
		b.blocks = map[string][]byte{}
	}
	buf := make([]byte, len(data))
	copy(buf, data)
	b.blocks[blockID] = buf
	return nil
}

func (s *Store) CommitBlockList(acct, cname, name string, blockIDs []string, props blobstore.BlobProps) (blobstore.BlobInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.lookupContainer(acct, cname)
	if err != nil {
		return blobstore.BlobInfo{}, err
	}
	b, ok := c.blobs[name]
	if !ok {
		return blobstore.BlobInfo{}, blobstore.ErrInvalidBlockList
	}
	var assembled []byte
	for _, id := range blockIDs {
		chunk, ok := b.blocks[id]
		if !ok {
			return blobstore.BlobInfo{}, blobstore.ErrInvalidBlockList
		}
		assembled = append(assembled, chunk...)
	}
	info, err := s.writeBlobLocked(acct, c, name, assembled, props)
	if err != nil {
		return blobstore.BlobInfo{}, err
	}
	// Staged blocks are consumed on commit.
	c.blobs[name].blocks = map[string][]byte{}
	return info, nil
}
