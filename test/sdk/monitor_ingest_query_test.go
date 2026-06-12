package sdk

import "testing"

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
