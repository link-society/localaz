package fsstore

import "localaz.dev/internal/stores/blobstore"

// account holds the containers belonging to a single storage account.
type account struct {
	containers map[string]*container
}

// container holds a container's metadata and its in-memory blob index.
type container struct {
	info  blobstore.ContainerInfo
	blobs map[string]*blobEntry
}

// blobEntry is the in-memory record for a single blob. Staged but uncommitted
// blocks live on disk under the container's blocks/ directory, not in memory.
type blobEntry struct {
	info blobstore.BlobInfo
}

// On-disk layout constants.
const (
	containerMetaFile = "_container.json"
	dataDir           = "data"
	metaDir           = "meta"
	blocksDir         = "blocks"
)
