package fsstore

import (
	"fmt"
	"time"
)

// newETag derives an Azure-style ETag from a modification timestamp.
func newETag(t time.Time) string {
	return fmt.Sprintf("\"0x%X\"", t.UnixNano())
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
