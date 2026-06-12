//go:build e2e

package e2e

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	account = "devstoreaccount1"
	// Azurite's well-known development account key.
	accountKey = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="
)

// blobEndpoint is the endpoint under test, resolved by TestMain.
var blobEndpoint string

func TestMain(m *testing.M) {
	if _, err := exec.LookPath("az"); err != nil {
		fmt.Println("e2e: skipping, the Azure CLI (az) is not installed")
		os.Exit(0)
	}

	stop, err := setup()
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: setup failed: %v\n", err)
		os.Exit(1)
	}

	os.Setenv("AZURE_STORAGE_CONNECTION_STRING", connectionString(blobEndpoint))
	code := m.Run()
	stop()
	os.Exit(code)
}

// setup resolves the endpoint, launching a local emulator when one is not
// supplied via LOCALAZ_E2E_ENDPOINT, and returns a teardown function.
func setup() (func(), error) {
	if ep := os.Getenv("LOCALAZ_E2E_ENDPOINT"); ep != "" {
		blobEndpoint = ep
		if err := waitForReady(blobEndpoint); err != nil {
			return nil, err
		}
		return func() {}, nil
	}

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "localaz-e2e-")
	if err != nil {
		return nil, err
	}

	bin := filepath.Join(tmp, "localaz")
	build := exec.Command("go", "build", "-o", bin, "./cmd/localaz")
	build.Dir = repoRoot
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		os.RemoveAll(tmp)
		return nil, fmt.Errorf("build emulator: %w", err)
	}

	port, err := freePort()
	if err != nil {
		os.RemoveAll(tmp)
		return nil, err
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	blobEndpoint = fmt.Sprintf("http://%s/%s", addr, account)

	srv := exec.Command(bin, "-addr", addr, "-data", filepath.Join(tmp, "data"))
	srv.Stdout, srv.Stderr = os.Stdout, os.Stderr
	if err := srv.Start(); err != nil {
		os.RemoveAll(tmp)
		return nil, fmt.Errorf("start emulator: %w", err)
	}

	if err := waitForReady(blobEndpoint); err != nil {
		_ = srv.Process.Kill()
		os.RemoveAll(tmp)
		return nil, err
	}

	return func() {
		_ = srv.Process.Kill()
		_, _ = srv.Process.Wait()
		os.RemoveAll(tmp)
	}, nil
}

func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func waitForReady(endpoint string) error {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(endpoint + "?comp=list")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("emulator did not become ready at %s", endpoint)
}

func connectionString(endpoint string) string {
	return fmt.Sprintf(
		"DefaultEndpointsProtocol=http;AccountName=%s;AccountKey=%s;BlobEndpoint=%s;",
		account, accountKey, endpoint,
	)
}

// az runs an Azure CLI command and returns its trimmed stdout, failing the test
// on a non-zero exit code.
func az(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("az", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("az %s\n  error: %v\n  stderr: %s", strings.Join(args, " "), err, stderr.String())
	}
	return strings.TrimSpace(stdout.String())
}

func TestContainerLifecycle(t *testing.T) {
	name := uniqueName("e2e-life")

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
	container := uniqueName("e2e-blob")
	az(t, "storage", "container", "create", "--name", container, "--output", "none")
	t.Cleanup(func() {
		az(t, "storage", "container", "delete", "--name", container, "--output", "none")
	})

	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	payload := "hello from the azure cli e2e suite\n"
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

func uniqueName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}
