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

// blobEntry is the in-memory record for a single blob, including any staged but
// uncommitted blocks.
type blobEntry struct {
	info   blobstore.BlobInfo
	blocks map[string][]byte // staged blocks, keyed by block id
}

// On-disk layout constants.
const (
	containerMetaFile = "_container.json"
	dataDir           = "data"
	metaDir           = "meta"
	blocksDir         = "blocks"
)
