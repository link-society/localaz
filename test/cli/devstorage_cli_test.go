//go:build cli

package cli

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestUseDevelopmentStorageShorthand verifies the exact connection string the
// Get Started guide tells users to export —
// AZURE_STORAGE_CONNECTION_STRING="UseDevelopmentStorage=true" — actually
// drives the Azure CLI against localaz end to end.
//
// The CLI/SDK expand that shorthand internally to the fixed development storage
// endpoints on ports 10000/10001/10002, so this test runs a dedicated emulator
// bound to those ports (every other service goes to an ephemeral port to avoid
// colliding with the main suite's emulator). It skips when any of the three
// dev ports is already in use — e.g. another emulator or another localaz — so it
// never fights for a busy port.
func TestUseDevelopmentStorageShorthand(t *testing.T) {
	for _, port := range []int{10000, 10001, 10002} {
		if !portFree(port) {
			t.Skipf("port %d is in use; skipping UseDevelopmentStorage smoke test", port)
		}
	}

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "localaz")
	build := exec.Command("go", "build", "-o", bin, "./cmd/localaz")
	build.Dir = repoRoot
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("build emulator: %v", err)
	}

	srv := exec.Command(bin,
		"-addr", "127.0.0.1:10000",
		"-queue-addr", "127.0.0.1:10001",
		"-table-addr", "127.0.0.1:10002",
		"-eventgrid-addr", "127.0.0.1:0",
		"-webpubsub-addr", "127.0.0.1:0",
		"-monitor-addr", "127.0.0.1:0",
		"-aad-addr", "127.0.0.1:0",
		"-arm-addr", "127.0.0.1:0",
		"-servicebus-addr", "127.0.0.1:0",
		"-data", filepath.Join(tmp, "data"))
	srv.Stdout, srv.Stderr = os.Stdout, os.Stderr
	if err := srv.Start(); err != nil {
		t.Fatalf("start emulator: %v", err)
	}
	t.Cleanup(func() {
		_ = srv.Process.Kill()
		_, _ = srv.Process.Wait()
	})

	if err := waitForReady("http://127.0.0.1:10000/devstoreaccount1?comp=list"); err != nil {
		t.Fatal(err)
	}

	// Point the CLI at the emulator using exactly the string the docs publish.
	t.Setenv("AZURE_STORAGE_CONNECTION_STRING", "UseDevelopmentStorage=true")

	name := uniqueName("cli-devstore")
	az(t, "storage", "container", "create", "--name", name, "--output", "none")
	if got := az(t, "storage", "container", "exists", "--name", name, "--query", "exists", "-o", "tsv"); got != "true" {
		t.Fatalf("container exists = %q, want true", got)
	}
	if got := az(t, "storage", "container", "list", "--query", fmt.Sprintf("[?name=='%s'] | length(@)", name), "-o", "tsv"); got != "1" {
		t.Fatalf("container listing count = %q, want 1", got)
	}
	az(t, "storage", "container", "delete", "--name", name, "--output", "none")
}

// portFree reports whether a loopback TCP port can be bound right now.
func portFree(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}
