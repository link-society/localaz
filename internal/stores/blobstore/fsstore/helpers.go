package fsstore

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// streamToFile copies src into a freshly created temp file alongside dst,
// computing the MD5 digest and byte count of the data as it streams, then
// atomically renames the temp file onto dst. The payload is never buffered in
// memory. The parent directory of dst must already exist.
func streamToFile(dst string, src io.Reader) (sum []byte, n int64, err error) {
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".tmp-*")
	if err != nil {
		return nil, 0, err
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()
	h := md5.New()
	n, err = io.Copy(io.MultiWriter(tmp, h), src)
	if err != nil {
		return nil, 0, err
	}
	if err = tmp.Sync(); err != nil {
		return nil, 0, err
	}
	if err = tmp.Close(); err != nil {
		return nil, 0, err
	}
	if err = os.Rename(tmpName, dst); err != nil {
		return nil, 0, err
	}
	// Fsync the parent directory so the rename itself survives a crash
	// (best-effort; some platforms disallow syncing a directory).
	if dir, derr := os.Open(filepath.Dir(dst)); derr == nil {
		_ = dir.Sync()
		dir.Close()
	}
	return h.Sum(nil), n, nil
}

// newETag derives an Azure-style ETag from a modification timestamp.
func newETag(t time.Time) string {
	return fmt.Sprintf("\"0x%X\"", t.UnixNano())
}

// encodeMarker turns a resume-after key into the opaque base64url continuation
// token handed back to clients as <NextMarker>.
func encodeMarker(name string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(name))
}

// decodeMarker reverses encodeMarker. A malformed token decodes to the empty
// string, which the caller treats as "list from the beginning".
func decodeMarker(marker string) string {
	if marker == "" {
		return ""
	}
	raw, err := base64.RawURLEncoding.DecodeString(marker)
	if err != nil {
		return ""
	}
	return string(raw)
}

// cloneMeta returns a copy of the metadata map. The result is never nil.
func cloneMeta(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
