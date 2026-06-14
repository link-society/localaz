//go:build cli

package cli

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"localaz.dev/internal/utils/devcert"
)

const account = "devstoreaccount1"

// accountKey is a random, syntactically valid Shared Key generated fresh for
// each run. localaz does not verify the Shared Key signature, so the value is
// irrelevant beyond being valid base64 — we deliberately never hardcode a real
// or well-known key.
var accountKey = randomAccountKey()

func randomAccountKey() string {
	b := make([]byte, 64)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(b)
}

// blobEndpoint, queueEndpoint and tableEndpoint are the storage endpoints under
// test, resolved by TestMain. caCertPath is the PEM the CLI trusts for them.
var (
	blobEndpoint  string
	queueEndpoint string
	tableEndpoint string
	caCertPath    string
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
	if caCertPath != "" {
		os.Setenv("REQUESTS_CA_BUNDLE", caCertPath)
		os.Setenv("SSL_CERT_FILE", caCertPath)
	}
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
		caCertPath = os.Getenv("LOCALAZ_CLI_CA")
		if err := waitForReadyClient(insecureClient(), blobEndpoint+"?comp=list"); err != nil {
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

	// localaz serves every HTTP service over TLS (the Azure CLI refuses to send
	// credentials otherwise), so generate the cert ourselves: that way we know
	// its path up front and can point the CLI's trust store at it.
	certPath, keyPath, err := writeDevCert(tmp)
	if err != nil {
		os.RemoveAll(tmp)
		return nil, err
	}
	caCertPath = certPath

	blobAddr, queueAddr, tableAddr, err := threeFreeAddrs()
	if err != nil {
		os.RemoveAll(tmp)
		return nil, err
	}
	blobEndpoint = fmt.Sprintf("https://%s/%s", blobAddr, account)
	queueEndpoint = fmt.Sprintf("https://%s/%s", queueAddr, account)
	tableEndpoint = fmt.Sprintf("https://%s/%s", tableAddr, account)

	managementAddr, err := oneFreeAddr()
	if err != nil {
		os.RemoveAll(tmp)
		return nil, err
	}

	// Bind every service we do not exercise to an ephemeral port so the suite
	// never collides with a real storage emulator, the control-plane suite, or a
	// previous run.
	srv := exec.Command(bin,
		"-blob-addr", blobAddr,
		"-queue-addr", queueAddr,
		"-table-addr", tableAddr,
		"-eventgrid-addr", "127.0.0.1:0",
		"-webpubsub-addr", "127.0.0.1:0",
		"-monitor-addr", "127.0.0.1:0",
		"-aad-addr", "127.0.0.1:0",
		"-arm-addr", "127.0.0.1:0",
		"-servicebus-addr", "127.0.0.1:0",
		"-keyvault-addr", "127.0.0.1:0",
		"-management-addr", managementAddr,
		"-tls-cert", certPath,
		"-tls-key", keyPath,
		"-data", filepath.Join(tmp, "data"))
	srv.Stdout, srv.Stderr = os.Stdout, os.Stderr
	if err := srv.Start(); err != nil {
		os.RemoveAll(tmp)
		return nil, fmt.Errorf("start emulator: %w", err)
	}

	// Wait (with timeout) for the emulator's health endpoint to report ready
	// before issuing any CLI command. The management server is plain HTTP, so
	// this needs no TLS trust material.
	if err := waitForHealth("http://"+managementAddr+"/health", 15*time.Second); err != nil {
		_ = srv.Process.Kill()
		os.RemoveAll(tmp)
		return nil, err
	}

	client := insecureClient()
	for _, ready := range []string{
		blobEndpoint + "?comp=list",
		queueEndpoint + "?comp=list",
		tableEndpoint + "/Tables",
	} {
		if err := waitForReadyClient(client, ready); err != nil {
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

// writeDevCert generates the self-signed TLS material the emulator serves with
// and writes it under tmp/tls, returning the certificate and key paths.
func writeDevCert(tmp string) (certPath, keyPath string, err error) {
	certPEM, keyPEM, err := devcert.Generate("127.0.0.1")
	if err != nil {
		return "", "", fmt.Errorf("generate dev cert: %w", err)
	}
	tlsDir := filepath.Join(tmp, "tls")
	if err := os.MkdirAll(tlsDir, 0o755); err != nil {
		return "", "", err
	}
	certPath = filepath.Join(tlsDir, "localaz.crt")
	keyPath = filepath.Join(tlsDir, "localaz.key")
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return "", "", err
	}
	return certPath, keyPath, nil
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

// oneFreeAddr reserves a single loopback address with a free port.
func oneFreeAddr() (string, error) {
	p, err := freePort()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("127.0.0.1:%d", p), nil
}

func connectionString() string {
	return fmt.Sprintf(
		"DefaultEndpointsProtocol=https;AccountName=%s;AccountKey=%s;BlobEndpoint=%s;QueueEndpoint=%s;TableEndpoint=%s;",
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

// waitForHealth polls the plain-HTTP health endpoint until it reports ready
// (200) or the timeout elapses.
func waitForHealth(url string, timeout time.Duration) error {
	client := &http.Client{Timeout: 3 * time.Second}
	deadline := time.Now().Add(timeout)
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
	return fmt.Errorf("health endpoint did not report ready: %s", url)
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
