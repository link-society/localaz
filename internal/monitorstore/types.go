package monitorstore

// columnTimeGenerated is the synthetic ingestion-time column added to every
// record that does not already provide one, mirroring the Log Analytics
// TimeGenerated column.
const columnTimeGenerated = "TimeGenerated"

// Row is a single ingested log record: a set of named fields decoded from the
// JSON object the client uploaded. Values are the result of json.Unmarshal
// (string, float64, bool, nil, []any, or map[string]any).
type Row map[string]any
