package fsstore

import (
	"os"
	"sort"
	"strings"
	"time"

	"localaz.dev/internal/stores/blobstore"
)

func (s *Store) CreateContainer(acct, name string, metadata map[string]string) (blobstore.ContainerInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.getAccount(acct)
	if _, exists := a.containers[name]; exists {
		return blobstore.ContainerInfo{}, blobstore.ErrContainerExists
	}
	now := time.Now().UTC()
	info := blobstore.ContainerInfo{
		Name:         name,
		ETag:         newETag(now),
		LastModified: now,
		Metadata:     cloneMeta(metadata),
	}
	c := &container{info: info, blobs: map[string]*blobEntry{}}
	if err := s.persistContainer(acct, c); err != nil {
		return blobstore.ContainerInfo{}, err
	}
	a.containers[name] = c
	return info, nil
}

func (s *Store) DeleteContainer(acct, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[acct]
	if !ok {
		return blobstore.ErrContainerNotFound
	}
	if _, ok := a.containers[name]; !ok {
		return blobstore.ErrContainerNotFound
	}
	if err := os.RemoveAll(s.containerDir(acct, name)); err != nil {
		return err
	}
	delete(a.containers, name)
	return nil
}

func (s *Store) GetContainer(acct, name string) (blobstore.ContainerInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, err := s.lookupContainer(acct, name)
	if err != nil {
		return blobstore.ContainerInfo{}, err
	}
	info := c.info
	info.Metadata = cloneMeta(c.info.Metadata)
	return info, nil
}

func (s *Store) ListContainers(acct, prefix string) ([]blobstore.ContainerInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.accounts[acct]
	if !ok {
		return nil, nil
	}
	var out []blobstore.ContainerInfo
	for name, c := range a.containers {
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}
		info := c.info
		info.Metadata = cloneMeta(c.info.Metadata)
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
