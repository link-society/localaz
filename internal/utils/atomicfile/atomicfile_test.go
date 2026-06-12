package atomicfile_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"localaz.dev/internal/utils/atomicfile"
)

func TestWriteCreatesFileWithContentAndPerm(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.bin")
	want := []byte("hello durable world")

	if err := atomicfile.Write(path, want, 0o644); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("content = %q, want %q", got, want)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o644 {
		t.Fatalf("perm = %o, want %o", perm, 0o644)
	}

	// No leftover temp file should remain after a successful write.
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file still present: %v", err)
	}
}

func TestWriteOverwritesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.bin")

	if err := atomicfile.Write(path, []byte("first contents, longer"), 0o600); err != nil {
		t.Fatalf("first Write: %v", err)
	}
	want := []byte("second")
	if err := atomicfile.Write(path, want, 0o600); err != nil {
		t.Fatalf("second Write: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("content = %q, want %q (overwrite must fully replace)", got, want)
	}
}

func TestWriteContentFullyPresent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.bin")
	want := bytes.Repeat([]byte("durability-"), 4096)

	if err := atomicfile.Write(path, want, 0o644); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (content must be fully present after Write returns)", len(got), len(want))
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("content mismatch after Write returned")
	}
}
