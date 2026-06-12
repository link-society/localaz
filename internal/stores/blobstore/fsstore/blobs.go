package fsstore

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"localaz.dev/internal/stores/blobstore"
)

func (s *Store) PutBlob(acct, cname, name string, data io.Reader, props blobstore.BlobProps) (blobstore.BlobInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.lookupContainer(acct, cname)
	if err != nil {
		return blobstore.BlobInfo{}, err
	}
	// Stream the payload to its data file, computing MD5 + byte count en route.
	if err := os.MkdirAll(filepath.Join(s.containerDir(acct, cname), dataDir), 0o755); err != nil {
		return blobstore.BlobInfo{}, err
	}
	sum, n, err := streamToFile(s.blobDataPath(acct, cname, name), data)
	if err != nil {
		return blobstore.BlobInfo{}, err
	}
	// The single-shot Put Blob path records the payload MD5 so Get/Head Blob can
	// echo it back, matching Azure's behavior.
	props.ContentMD5 = sum
	return s.commitWrittenBlobLocked(acct, c, name, n, props)
}

// commitWrittenBlobLocked records a blob whose data file has already been
// streamed to disk (with byte count n). The caller must hold s.mu.
func (s *Store) commitWrittenBlobLocked(acct string, c *container, name string, n int64, props blobstore.BlobProps) (blobstore.BlobInfo, error) {
	now := time.Now().UTC()
	props.Metadata = cloneMeta(props.Metadata)
	info := blobstore.BlobInfo{
		Name:          name,
		ContainerName: c.info.Name,
		ContentLength: n,
		ETag:          newETag(now),
		LastModified:  now,
		BlobType:      "BlockBlob",
		Props:         props,
	}
	b := &blobEntry{info: info}
	if err := s.persistBlobMeta(acct, c.info.Name, b); err != nil {
		return blobstore.BlobInfo{}, err
	}
	c.blobs[name] = b
	return info, nil
}

func (s *Store) GetBlob(acct, cname, name string) (io.ReadCloser, blobstore.BlobInfo, error) {
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
	f, err := os.Open(s.blobDataPath(acct, cname, name))
	if err != nil {
		return nil, blobstore.BlobInfo{}, err
	}
	info := b.info
	info.Props.Metadata = cloneMeta(b.info.Props.Metadata)
	return f, info, nil
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

// maxListResults is Azure's default and maximum page size for List Blobs.
const maxListResults = 5000

func (s *Store) ListBlobs(acct, cname, prefix, delimiter string, maxResults int, marker string) ([]blobstore.BlobInfo, []string, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, err := s.lookupContainer(acct, cname)
	if err != nil {
		return nil, nil, "", err
	}

	// Clamp the page size to Azure's bounds; <=0 (or unset) means the max/default.
	if maxResults <= 0 || maxResults > maxListResults {
		maxResults = maxListResults
	}
	// The marker is an opaque base64url token naming the entry to resume AFTER. A
	// malformed marker is treated as empty (list from the beginning).
	resumeAfter := decodeMarker(marker)

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
	count := 0
	nextMarker := ""
	for _, name := range names {
		// Collapse delimiter-bearing names into their virtual directory prefix.
		key := name
		isPrefix := false
		if delimiter != "" {
			rest := name[len(prefix):]
			if idx := strings.Index(rest, delimiter); idx >= 0 {
				key = prefix + rest[:idx+len(delimiter)]
				isPrefix = true
				if _, seen := prefixSet[key]; seen {
					// Already emitted this virtual directory; do not double-count.
					continue
				}
			}
		}
		// Skip everything up to and including the resume point.
		if resumeAfter != "" && key <= resumeAfter {
			if isPrefix {
				prefixSet[key] = struct{}{}
			}
			continue
		}
		// Page is full: more entries remain, so emit a continuation token.
		if count >= maxResults {
			nextMarker = encodeMarker(resumeAfter)
			break
		}
		if isPrefix {
			prefixSet[key] = struct{}{}
			prefixes = append(prefixes, key)
		} else {
			info := c.blobs[name].info
			info.Props.Metadata = cloneMeta(c.blobs[name].info.Props.Metadata)
			blobs = append(blobs, info)
		}
		count++
		resumeAfter = key
	}
	sort.Strings(prefixes)
	return blobs, prefixes, nextMarker, nil
}
