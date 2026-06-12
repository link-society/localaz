//go:build cli

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestContainerLifecycle(t *testing.T) {
	name := uniqueName("cli-life")

	az(t, "storage", "container", "create", "--name", name, "--output", "none")

	if got := az(t, "storage", "container", "exists", "--name", name, "--query", "exists", "-o", "tsv"); got != "true" {
		t.Fatalf("container exists = %q, want true", got)
	}
	if got := az(t, "storage", "container", "list", "--query", fmt.Sprintf("[?name=='%s'] | length(@)", name), "-o", "tsv"); got != "1" {
		t.Fatalf("container listing count = %q, want 1", got)
	}

	az(t, "storage", "container", "delete", "--name", name, "--output", "none")
}

func TestBlobRoundTrip(t *testing.T) {
	container := uniqueName("cli-blob")
	az(t, "storage", "container", "create", "--name", container, "--output", "none")
	t.Cleanup(func() {
		az(t, "storage", "container", "delete", "--name", container, "--output", "none")
	})

	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	payload := "hello from the azure cli cli suite\n"
	if err := os.WriteFile(src, []byte(payload), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	az(t, "storage", "blob", "upload",
		"--container-name", container, "--name", "hello.txt",
		"--file", src, "--content-type", "text/plain", "--overwrite", "--output", "none")

	if got := az(t, "storage", "blob", "exists", "--container-name", container, "--name", "hello.txt", "--query", "exists", "-o", "tsv"); got != "true" {
		t.Fatalf("blob exists = %q, want true", got)
	}
	if got := az(t, "storage", "blob", "show", "--container-name", container, "--name", "hello.txt", "--query", "properties.contentSettings.contentType", "-o", "tsv"); got != "text/plain" {
		t.Fatalf("content type = %q, want text/plain", got)
	}
	if got := az(t, "storage", "blob", "list", "--container-name", container, "--query", "[?name=='hello.txt'] | length(@)", "-o", "tsv"); got != "1" {
		t.Fatalf("blob listing count = %q, want 1", got)
	}

	az(t, "storage", "blob", "download", "--container-name", container, "--name", "hello.txt", "--file", dst, "--output", "none")
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if string(got) != payload {
		t.Fatalf("downloaded content mismatch:\n got %q\nwant %q", string(got), payload)
	}

	az(t, "storage", "blob", "delete", "--container-name", container, "--name", "hello.txt", "--output", "none")
	if got := az(t, "storage", "blob", "exists", "--container-name", container, "--name", "hello.txt", "--query", "exists", "-o", "tsv"); got != "false" {
		t.Fatalf("blob exists after delete = %q, want false", got)
	}
}
