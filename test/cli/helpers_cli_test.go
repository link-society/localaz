//go:build cli

package cli

import (
	"bytes"
	"crypto/tls"
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

// blobEndpoint, queueEndpoint and tableEndpoint are the storage endpoints under
// test, resolved by TestMain.
var (
	blobEndpoint  string
	queueEndpoint string
	tableEndpoint string
)

func TestMain(m *testing.M) {
	if _, err := exec.LookPath("az"); err != nil {
		fmt.Println("cli: skipping, the Azure CLI (az) is not installed")
		os.Exit(0)
	}

	stop, err := setup()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cli: setup failed: %v\n", err)
		os.Exit(1)
	}

	os.Setenv("AZURE_STORAGE_CONNECTION_STRING", connectionString())
	code := m.Run()
	stop()
	os.Exit(code)
}

// setup resolves the endpoints, launching a local emulator when one is not
// supplied via LOCALAZ_CLI_ENDPOINT, and returns a teardown function.
func setup() (func(), error) {
	if ep := os.Getenv("LOCALAZ_CLI_ENDPOINT"); ep != "" {
		blobEndpoint = ep
		queueEndpoint = os.Getenv("LOCALAZ_CLI_QUEUE_ENDPOINT")
		tableEndpoint = os.Getenv("LOCALAZ_CLI_TABLE_ENDPOINT")
		if err := waitForReady(blobEndpoint + "?comp=list"); err != nil {
			return nil, err
		}
		return func() {}, nil
	}

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "localaz-cli-")
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

	blobAddr, queueAddr, tableAddr, err := threeFreeAddrs()
	if err != nil {
		os.RemoveAll(tmp)
		return nil, err
	}
	blobEndpoint = fmt.Sprintf("http://%s/%s", blobAddr, account)
	queueEndpoint = fmt.Sprintf("http://%s/%s", queueAddr, account)
	tableEndpoint = fmt.Sprintf("http://%s/%s", tableAddr, account)

	// Bind the services we do not exercise to ephemeral ports so the suite
	// never collides with a real Azurite or a previous run.
	srv := exec.Command(bin,
		"-addr", blobAddr,
		"-queue-addr", queueAddr,
		"-table-addr", tableAddr,
		"-eventgrid-addr", "127.0.0.1:0",
		"-webpubsub-addr", "127.0.0.1:0",
		"-servicebus-addr", "127.0.0.1:0",
		"-data", filepath.Join(tmp, "data"))
	srv.Stdout, srv.Stderr = os.Stdout, os.Stderr
	if err := srv.Start(); err != nil {
		os.RemoveAll(tmp)
		return nil, fmt.Errorf("start emulator: %w", err)
	}

	for _, ready := range []string{
		blobEndpoint + "?comp=list",
		queueEndpoint + "?comp=list",
		tableEndpoint + "/Tables",
	} {
		if err := waitForReady(ready); err != nil {
			_ = srv.Process.Kill()
			os.RemoveAll(tmp)
			return nil, err
		}
	}

	return func() {
		_ = srv.Process.Kill()
		_, _ = srv.Process.Wait()
		os.RemoveAll(tmp)
	}, nil
}

// threeFreeAddrs reserves three distinct loopback ports for the blob, queue and
// table listeners.
func threeFreeAddrs() (blob, queue, table string, err error) {
	ports := make([]int, 0, 3)
	for len(ports) < 3 {
		p, perr := freePort()
		if perr != nil {
			return "", "", "", perr
		}
		ports = append(ports, p)
	}
	return fmt.Sprintf("127.0.0.1:%d", ports[0]),
		fmt.Sprintf("127.0.0.1:%d", ports[1]),
		fmt.Sprintf("127.0.0.1:%d", ports[2]), nil
}

func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func waitForReady(url string) error {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("emulator did not become ready at %s", url)
}

func connectionString() string {
	return fmt.Sprintf(
		"DefaultEndpointsProtocol=http;AccountName=%s;AccountKey=%s;BlobEndpoint=%s;QueueEndpoint=%s;TableEndpoint=%s;",
		account, accountKey, blobEndpoint, queueEndpoint, tableEndpoint,
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

// azTolerate runs an Azure CLI command, treating a failure whose stderr
// contains allowedErr as success. Any other failure fails the test.
func azTolerate(t *testing.T, allowedErr string, args ...string) {
	t.Helper()
	cmd := exec.Command("az", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), allowedErr) {
			return
		}
		t.Fatalf("az %s\n  error: %v\n  stderr: %s", strings.Join(args, " "), err, stderr.String())
	}
}

func uniqueName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// requireQueue skips the test when no queue endpoint is configured.
func requireQueue(t *testing.T) {
	t.Helper()
	if queueEndpoint == "" {
		t.Skip("queue endpoint not configured")
	}
}

// requireTable skips the test when no table endpoint is configured.
func requireTable(t *testing.T) {
	t.Helper()
	if tableEndpoint == "" {
		t.Skip("table endpoint not configured")
	}
}

// mustFreeAddr reserves a loopback address with a free port.
func mustFreeAddr(t *testing.T) string {
	t.Helper()
	p, err := freePort()
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	return fmt.Sprintf("127.0.0.1:%d", p)
}

// insecureClient returns an HTTP client that trusts the emulator's self-signed
// TLS certificate. It is used only by the test harness; the Azure CLI verifies
// the certificate via REQUESTS_CA_BUNDLE/SSL_CERT_FILE.
func insecureClient() *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

// waitForReadyClient polls url until it returns 200 or the deadline elapses.
func waitForReadyClient(client *http.Client, url string) error {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("endpoint did not become ready: %s", url)
}

// seedLogs ingests a few records into the AppLogs_CL custom table through the
// Logs Ingestion API, giving the KQL subtests a dataset with both string and
// numeric columns to filter, sort and count over.
func seedLogs(t *testing.T, client *http.Client, monitorURL string) {
	t.Helper()
	body := []byte(`[` +
		`{"Level":"info","Message":"hi","Code":200,"Source":"api"},` +
		`{"Level":"error","Message":"boom","Code":500,"Source":"api"},` +
		`{"Level":"warning","Message":"slow","Code":300,"Source":"worker"},` +
		`{"Level":"error","Message":"kaput","Code":503,"Source":"worker"},` +
		`{"Level":"info","Message":"ok","Code":201,"Source":"api"}` +
		`]`)
	url := monitorURL + "/dataCollectionRules/dcr1/streams/Custom-AppLogs_CL"
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("seed logs: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("seed logs status = %d, want 204", resp.StatusCode)
	}
}
