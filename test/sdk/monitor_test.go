package sdk

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	azingest "github.com/Azure/azure-sdk-for-go/sdk/monitor/ingestion/azlogs"
	azquery "github.com/Azure/azure-sdk-for-go/sdk/monitor/query/azlogs"

	"localaz.dev/internal/monitorserver"
	"localaz.dev/internal/monitorstore"
)

// fakeCredential satisfies azcore.TokenCredential. The emulator never validates
// bearer tokens, so any non-empty token works.
type fakeCredential struct{}

func (fakeCredential) GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: "localaz-dev-token", ExpiresOn: time.Now().Add(time.Hour)}, nil
}

// newMonitor spins up an in-process Monitor Logs emulator and returns ingestion
// and query clients pointed at it.
func newMonitor(t *testing.T) (*azingest.Client, *azquery.Client) {
	t.Helper()
	// The Monitor SDKs send bearer credentials, which azcore refuses to do over
	// plain HTTP, so the emulator is exposed over TLS and the SDK is pointed at
	// the test server's trusting client.
	ts := httptest.NewTLSServer(monitorserver.New(monitorstore.New()))
	t.Cleanup(ts.Close)

	cfg := cloud.Configuration{
		Services: map[cloud.ServiceName]cloud.ServiceConfiguration{
			azingest.ServiceNameIngestion: {Audience: "https://monitor.azure.com", Endpoint: ts.URL},
			azquery.ServiceName:           {Audience: "https://api.loganalytics.io", Endpoint: ts.URL},
		},
	}

	ingOpts := &azingest.ClientOptions{}
	ingOpts.Cloud = cfg
	ingOpts.Transport = ts.Client()
	ingest, err := azingest.NewClient(ts.URL, fakeCredential{}, ingOpts)
	if err != nil {
		t.Fatalf("create ingestion client: %v", err)
	}

	qOpts := &azquery.ClientOptions{}
	qOpts.Cloud = cfg
	qOpts.Transport = ts.Client()
	query, err := azquery.NewClient(fakeCredential{}, qOpts)
	if err != nil {
		t.Fatalf("create query client: %v", err)
	}
	return ingest, query
}

// upload marshals records to JSON and uploads them to the given stream.
func upload(t *testing.T, c *azingest.Client, stream string, records []map[string]any) {
	t.Helper()
	body, err := json.Marshal(records)
	if err != nil {
		t.Fatalf("marshal records: %v", err)
	}
	if _, err := c.Upload(ctx(t), "dcr-localaz", stream, body, nil); err != nil {
		t.Fatalf("upload logs: %v", err)
	}
}

// queryRows runs a KQL query and returns the single PrimaryResult table.
func queryRows(t *testing.T, c *azquery.Client, kql string) azquery.Table {
	t.Helper()
	resp, err := c.QueryWorkspace(ctx(t), "workspace-localaz", azquery.QueryBody{Query: to.Ptr(kql)}, nil)
	if err != nil {
		t.Fatalf("query workspace: %v", err)
	}
	if len(resp.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(resp.Tables))
	}
	return resp.Tables[0]
}

func TestMonitorIngestAndQuery(t *testing.T) {
	ingest, query := newMonitor(t)

	upload(t, ingest, "Custom-AppLogs_CL", []map[string]any{
		{"Level": "info", "Message": "started", "Code": float64(200)},
		{"Level": "error", "Message": "boom", "Code": float64(500)},
		{"Level": "error", "Message": "kaboom", "Code": float64(503)},
	})

	tbl := queryRows(t, query, "AppLogs_CL")
	if len(tbl.Rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(tbl.Rows))
	}
	if tbl.Name == nil || *tbl.Name != "PrimaryResult" {
		t.Fatalf("table name = %v, want PrimaryResult", tbl.Name)
	}
}

func TestMonitorWhereAndProject(t *testing.T) {
	ingest, query := newMonitor(t)
	upload(t, ingest, "Custom-AppLogs_CL", []map[string]any{
		{"Level": "info", "Message": "started", "Code": float64(200)},
		{"Level": "error", "Message": "boom", "Code": float64(500)},
		{"Level": "error", "Message": "kaboom", "Code": float64(503)},
	})

	tbl := queryRows(t, query, "AppLogs_CL | where Level == 'error' | project Message, Level")
	if len(tbl.Rows) != 2 {
		t.Fatalf("expected 2 error rows, got %d", len(tbl.Rows))
	}
	if len(tbl.Columns) != 2 || tbl.Columns[0].Name == nil || *tbl.Columns[0].Name != "Message" {
		t.Fatalf("unexpected columns: %+v", tbl.Columns)
	}
	for _, row := range tbl.Rows {
		if got, ok := row[1].(string); !ok || got != "error" {
			t.Errorf("Level = %v, want error", row[1])
		}
	}
}

func TestMonitorNumericFilterAndCount(t *testing.T) {
	ingest, query := newMonitor(t)
	upload(t, ingest, "Custom-AppLogs_CL", []map[string]any{
		{"Level": "info", "Code": float64(200)},
		{"Level": "error", "Code": float64(500)},
		{"Level": "error", "Code": float64(503)},
	})

	tbl := queryRows(t, query, "AppLogs_CL | where Code >= 500 | count")
	if len(tbl.Rows) != 1 || len(tbl.Columns) != 1 {
		t.Fatalf("expected single count cell, got %d rows / %d cols", len(tbl.Rows), len(tbl.Columns))
	}
	if tbl.Columns[0].Name == nil || *tbl.Columns[0].Name != "Count" {
		t.Fatalf("count column name = %v, want Count", tbl.Columns[0].Name)
	}
	count, ok := tbl.Rows[0][0].(float64)
	if !ok || count != 2 {
		t.Fatalf("count = %v, want 2", tbl.Rows[0][0])
	}
}

func TestMonitorTakeAndSort(t *testing.T) {
	ingest, query := newMonitor(t)
	upload(t, ingest, "Custom-Metrics_CL", []map[string]any{
		{"Name": "a", "Value": float64(3)},
		{"Name": "b", "Value": float64(1)},
		{"Name": "c", "Value": float64(2)},
	})

	tbl := queryRows(t, query, "Metrics_CL | sort by Value asc | project Name, Value | take 2")
	if len(tbl.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(tbl.Rows))
	}
	if got := tbl.Rows[0][0]; got != "b" {
		t.Errorf("first row Name = %v, want b", got)
	}
	if got := tbl.Rows[1][0]; got != "c" {
		t.Errorf("second row Name = %v, want c", got)
	}
}
