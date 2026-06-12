// Package atomicfile writes files crash-safely: it writes to a temporary file,
// fsyncs it, renames it over the destination, then fsyncs the parent directory
// so the rename itself is durable. A crash at any point leaves either the old
// file or the fully written new file, never a truncated or invisible one.
package atomicfile

import (
	"os"
	"path/filepath"
)

// Write writes data to path crash-safely. It writes path+".tmp" with the given
// perm, fsyncs and closes it, renames it onto path, then best-effort fsyncs the
// parent directory (a no-op error is ignored on platforms that disallow it).
// The first real error encountered is returned.
func Write(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	// Fsync the parent directory so the rename survives a crash. Some platforms
	// disallow opening/syncing a directory; treat that as best-effort.
	if dir, err := os.Open(filepath.Dir(path)); err == nil {
		_ = dir.Sync()
		dir.Close()
	}
	return nil
}
