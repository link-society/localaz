package sdk

import "testing"

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
