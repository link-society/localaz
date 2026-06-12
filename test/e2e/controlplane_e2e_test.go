//go:build e2e

package e2e

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestControlPlaneCLI drives the emulated Entra ID + Resource Manager control
// plane through the real Azure CLI: it registers localaz as a custom cloud,
// signs in with a service principal, manages a resource group, and finally
// runs a Log Analytics query whose data-plane host is discovered from the ARM
// cloud metadata. The whole flow runs against an isolated AZURE_CONFIG_DIR so
// it never disturbs the developer's real clouds, logins or active cloud.
func TestControlPlaneCLI(t *testing.T) {
	if _, err := exec.LookPath("az"); err != nil {
		t.Skip("the Azure CLI (az) is not installed")
	}

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	tmp := t.TempDir()
	bin := filepath.Join(tmp, "localaz")
	build := exec.Command("go", "build", "-o", bin, "./cmd/localaz")
	build.Dir = repoRoot
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("build emulator: %v", err)
	}

	monitorAddr := mustFreeAddr(t)
	aadAddr := mustFreeAddr(t)
	armAddr := mustFreeAddr(t)
	dataDir := filepath.Join(tmp, "data")
	const cloudName = "localaze2e"

	srv := exec.Command(bin,
		"-tls-auto",
		"-arm-cloud-name", cloudName,
		"-monitor-addr", monitorAddr,
		"-aad-addr", aadAddr,
		"-arm-addr", armAddr,
		"-addr", "127.0.0.1:0",
		"-queue-addr", "127.0.0.1:0",
		"-table-addr", "127.0.0.1:0",
		"-eventgrid-addr", "127.0.0.1:0",
		"-webpubsub-addr", "127.0.0.1:0",
		"-servicebus-addr", "127.0.0.1:0",
		"-data", dataDir,
	)
	srv.Stdout, srv.Stderr = os.Stdout, os.Stderr
	if err := srv.Start(); err != nil {
		t.Fatalf("start emulator: %v", err)
	}
	t.Cleanup(func() {
		_ = srv.Process.Kill()
		_, _ = srv.Process.Wait()
	})

	armURL := "https://" + armAddr
	aadURL := "https://" + aadAddr
	monitorURL := "https://" + monitorAddr
	metadataURL := armURL + "/metadata/endpoints"

	client := insecureClient()
	if err := waitForReadyClient(client, metadataURL); err != nil {
		t.Fatalf("control plane did not become ready: %v", err)
	}

	cert := filepath.Join(dataDir, "tls", "localaz.crt")
	if _, err := os.Stat(cert); err != nil {
		t.Fatalf("generated TLS certificate not found: %v", err)
	}

	// Isolate the CLI so registering/setting a custom cloud and logging in
	// never touches the developer's real ~/.azure. t.Setenv restores the
	// previous values when the test ends. The installed extensions live under
	// the original config dir, so keep pointing the extension loader there:
	// the log-analytics extension supplies "az monitor log-analytics query".
	extDir := os.Getenv("AZURE_EXTENSION_DIR")
	if extDir == "" {
		base := os.Getenv("AZURE_CONFIG_DIR")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				t.Fatalf("resolve home dir: %v", err)
			}
			base = filepath.Join(home, ".azure")
		}
		extDir = filepath.Join(base, "cliextensions")
	}
	if _, err := os.Stat(filepath.Join(extDir, "log-analytics")); err != nil {
		t.Skipf("log-analytics CLI extension is not installed (looked in %s)", extDir)
	}

	t.Setenv("AZURE_EXTENSION_DIR", extDir)
	t.Setenv("AZURE_CONFIG_DIR", filepath.Join(tmp, "azure"))
	t.Setenv("REQUESTS_CA_BUNDLE", cert)
	t.Setenv("SSL_CERT_FILE", cert)
	t.Setenv("ARM_CLOUD_METADATA_URL", metadataURL)

	// Registering is belt-and-suspenders: ARM_CLOUD_METADATA_URL already lets
	// the CLI auto-discover the cloud and its endpoints by name, so an explicit
	// register may report the cloud as already known. Either outcome is fine.
	azTolerate(t, "already registered", "cloud", "register", "-n", cloudName,
		"--endpoint-active-directory", aadURL+"/",
		"--endpoint-resource-manager", armURL+"/",
		"--endpoint-management", armURL+"/",
		"--endpoint-active-directory-resource-id", armURL+"/",
		"--suffix-storage-endpoint", "127.0.0.1",
		"--skip-endpoint-discovery",
		"--output", "none")
	az(t, "cloud", "set", "-n", cloudName, "--output", "none")

	az(t, "login", "--service-principal",
		"-u", "11111111-1111-1111-1111-111111111111",
		"-p", "localaz-secret",
		"--tenant", "adfs",
		"--output", "none")

	if got := az(t, "account", "show", "--query", "name", "-o", "tsv"); got != "localaz" {
		t.Fatalf("account name = %q, want localaz", got)
	}
	if got := az(t, "account", "show", "--query", "tenantId", "-o", "tsv"); got != "adfs" {
		t.Fatalf("account tenant = %q, want adfs", got)
	}
	if got := az(t, "account", "show", "--query", "user.type", "-o", "tsv"); got != "servicePrincipal" {
		t.Fatalf("account user type = %q, want servicePrincipal", got)
	}

	const rg = "e2e-rg"
	if got := az(t, "group", "create", "-n", rg, "-l", "localaz",
		"--query", "properties.provisioningState", "-o", "tsv"); got != "Succeeded" {
		t.Fatalf("resource group provisioning state = %q, want Succeeded", got)
	}
	if got := az(t, "group", "list",
		"--query", fmt.Sprintf("[?name=='%s'] | length(@)", rg), "-o", "tsv"); got != "1" {
		t.Fatalf("resource group listing count = %q, want 1", got)
	}
	az(t, "group", "delete", "-n", rg, "-y", "--output", "none")
	if got := az(t, "group", "list",
		"--query", fmt.Sprintf("[?name=='%s'] | length(@)", rg), "-o", "tsv"); got != "0" {
		t.Fatalf("resource group listing count after delete = %q, want 0", got)
	}

	// Seed a handful of log records, then prove the CLI's log-analytics query
	// resolves its data-plane host from the cloud metadata, reaches localaz, and
	// that the emulator's KQL subset behaves as documented through the real CLI.
	seedLogs(t, client, monitorURL)

	const workspace = "33333333-3333-3333-3333-333333333333"
	logQuery := func(t *testing.T, kql, jmesPath string) string {
		t.Helper()
		return az(t, "monitor", "log-analytics", "query",
			"-w", workspace,
			"--analytics-query", kql,
			"--query", jmesPath, "-o", "tsv")
	}

	// Each case drives one KQL pipeline through `az monitor log-analytics query`
	// and reads a value back with a JMESPath --query, covering the where
	// operators (string/number, ==/!=/</>=), and/or, project, sort, take and
	// count stages.
	t.Run("MonitorLogs", func(t *testing.T) {
		cases := []struct {
			name string
			kql  string
			jmes string
			want string
		}{
			{"WhereStringEq", "AppLogs_CL | where Level == 'error'", "length(@)", "2"},
			{"WhereStringNe", "AppLogs_CL | where Level != 'info'", "length(@)", "3"},
			{"WhereAnd", "AppLogs_CL | where Level == 'error' and Source == 'worker'", "[0].Message", "kaput"},
			{"WhereOr", "AppLogs_CL | where Level == 'warning' or Level == 'error'", "length(@)", "3"},
			{"WhereNumericGe", "AppLogs_CL | where Code >= 500", "length(@)", "2"},
			{"WhereNumericLt", "AppLogs_CL | where Code < 300", "length(@)", "2"},
			{"SortProjectTake", "AppLogs_CL | sort by Code desc | project Message | take 1", "[0].Message", "kaput"},
			{"Take", "AppLogs_CL | take 2", "length(@)", "2"},
			{"Count", "AppLogs_CL | count", "[0].Count", "5"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if got := logQuery(t, tc.kql, tc.jmes); got != tc.want {
					t.Fatalf("query %q [%s] = %q, want %q", tc.kql, tc.jmes, got, tc.want)
				}
			})
		}
	})

	// Drive the Microsoft.ServiceBus ARM resource provider through the real
	// CLI: namespace, queue and topic/subscription management resolve to
	// localaz because the CLI talks to the emulated Resource Manager.
	t.Run("ServiceBus", func(t *testing.T) {
		const sbRG = "sb-e2e-rg"
		az(t, "group", "create", "-n", sbRG, "-l", "localaz", "--output", "none")
		t.Cleanup(func() {
			az(t, "group", "delete", "-n", sbRG, "-y", "--output", "none")
		})

		ns := fmt.Sprintf("sbe2e%d", time.Now().UnixNano())
		if got := az(t, "servicebus", "namespace", "create",
			"-g", sbRG, "-n", ns, "-l", "localaz", "--sku", "Standard",
			"--query", "provisioningState", "-o", "tsv"); got != "Succeeded" {
			t.Fatalf("namespace provisioningState = %q, want Succeeded", got)
		}
		if got := az(t, "servicebus", "namespace", "show",
			"-g", sbRG, "-n", ns, "--query", "name", "-o", "tsv"); got != ns {
			t.Fatalf("namespace show name = %q, want %q", got, ns)
		}
		if got := az(t, "servicebus", "namespace", "list",
			"-g", sbRG, "--query", "length(@)", "-o", "tsv"); got != "1" {
			t.Fatalf("namespace list count = %q, want 1", got)
		}

		az(t, "servicebus", "queue", "create",
			"-g", sbRG, "--namespace-name", ns, "-n", "q1", "--output", "none")
		if got := az(t, "servicebus", "queue", "list",
			"-g", sbRG, "--namespace-name", ns, "--query", "length(@)", "-o", "tsv"); got != "1" {
			t.Fatalf("queue list count = %q, want 1", got)
		}

		az(t, "servicebus", "topic", "create",
			"-g", sbRG, "--namespace-name", ns, "-n", "t1", "--output", "none")
		if got := az(t, "servicebus", "topic", "subscription", "create",
			"-g", sbRG, "--namespace-name", ns, "--topic-name", "t1", "-n", "s1",
			"--query", "name", "-o", "tsv"); got != "s1" {
			t.Fatalf("topic subscription name = %q, want s1", got)
		}

		az(t, "servicebus", "queue", "delete",
			"-g", sbRG, "--namespace-name", ns, "-n", "q1", "--output", "none")
		if got := az(t, "servicebus", "queue", "list",
			"-g", sbRG, "--namespace-name", ns, "--query", "length(@)", "-o", "tsv"); got != "0" {
			t.Fatalf("queue list count after delete = %q, want 0", got)
		}
	})
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
