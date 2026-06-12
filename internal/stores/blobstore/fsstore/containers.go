package fsstore

import (
	"encoding/base64"
	"os"
	"sort"
	"strings"
	"time"

	"localaz.dev/internal/stores/blobstore"
)

// maxListContainersResults mirrors Azure's default and ceiling for the
// maxresults parameter of List Containers.
const maxListContainersResults = 5000

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

func (s *Store) ListContainers(acct, prefix string, maxResults int, marker string) ([]blobstore.ContainerInfo, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if maxResults <= 0 || maxResults > maxListContainersResults {
		maxResults = maxListContainersResults
	}
	a, ok := s.accounts[acct]
	if !ok {
		return nil, "", nil
	}
	var all []blobstore.ContainerInfo
	for name, c := range a.containers {
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}
		info := c.info
		info.Metadata = cloneMeta(c.info.Metadata)
		all = append(all, info)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })

	// marker is the base64url of the container name to resume AFTER; a malformed
	// marker decodes to empty, restarting from the beginning.
	after := decodeContainerMarker(marker)
	var out []blobstore.ContainerInfo
	var nextMarker string
	for _, info := range all {
		if after != "" && info.Name <= after {
			continue
		}
		if len(out) == maxResults {
			nextMarker = encodeContainerMarker(out[len(out)-1].Name)
			break
		}
		out = append(out, info)
	}
	return out, nextMarker, nil
}

func encodeContainerMarker(name string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(name))
}

func decodeContainerMarker(marker string) string {
	if marker == "" {
		return ""
	}
	b, err := base64.RawURLEncoding.DecodeString(marker)
	if err != nil {
		return ""
	}
	return string(b)
}
