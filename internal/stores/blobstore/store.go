// Package blobstore defines the storage abstraction used by the Azure Blob
// service emulator. The HTTP layer depends only on the Store interface so that
// alternative backends (filesystem, PostgreSQL, in-memory) can be swapped in
// without touching the protocol implementation.
package blobstore

import (
	"errors"
	"io"
	"time"
)

// Sentinel errors returned by Store implementations. The HTTP layer maps these
// onto the appropriate Azure error responses.
var (
	ErrContainerNotFound = errors.New("blobstore: container not found")
	ErrContainerExists   = errors.New("blobstore: container already exists")
	ErrBlobNotFound      = errors.New("blobstore: blob not found")
	ErrInvalidBlockList  = errors.New("blobstore: invalid block list")
)

// ContainerInfo describes a single container.
type ContainerInfo struct {
	Name         string
	ETag         string
	LastModified time.Time
	Metadata     map[string]string
}

// BlobProps carries the mutable, caller-supplied properties of a blob.
type BlobProps struct {
	ContentType        string
	ContentEncoding    string
	ContentLanguage    string
	ContentDisposition string
	CacheControl       string
	ContentMD5         []byte
	Metadata           map[string]string
}

// BlobInfo describes a stored blob (its properties plus server-managed fields).
type BlobInfo struct {
	Name          string
	ContainerName string
	ContentLength int64
	ETag          string
	LastModified  time.Time
	BlobType      string
	Props         BlobProps
}

// Store is the persistence contract for the Blob service emulator.
//
// Implementations must be safe for concurrent use by multiple goroutines.
type Store interface {
	// Containers.
	CreateContainer(account, container string, metadata map[string]string) (ContainerInfo, error)
	DeleteContainer(account, container string) error
	GetContainer(account, container string) (ContainerInfo, error)
	ListContainers(account, prefix string) ([]ContainerInfo, error)

	// Blobs. The blob data path streams to and from disk so that large or
	// concurrent blobs do not buffer their payload in memory.
	//
	// PutBlob streams data to disk while computing its MD5 and byte count; the
	// returned BlobInfo carries ContentLength (bytes copied) and Props.ContentMD5.
	PutBlob(account, container, name string, data io.Reader, props BlobProps) (BlobInfo, error)
	// GetBlob returns a reader over the blob payload; the caller must Close it.
	GetBlob(account, container, name string) (io.ReadCloser, BlobInfo, error)
	StatBlob(account, container, name string) (BlobInfo, error)
	DeleteBlob(account, container, name string) error
	// ListBlobs returns the blobs (optionally filtered by prefix) and, when a
	// delimiter is supplied, the set of virtual directory prefixes.
	ListBlobs(account, container, prefix, delimiter string) (blobs []BlobInfo, prefixes []string, err error)

	// Block blob staging. Staged blocks stream to per-block files on disk.
	StageBlock(account, container, name, blockID string, data io.Reader) error
	CommitBlockList(account, container, name string, blockIDs []string, props BlobProps) (BlobInfo, error)
}
