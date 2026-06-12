// Package monitorstore is an in-memory store backing the localaz Azure Monitor
// Logs emulator. Ingested log records are appended to a table derived from the
// data-collection stream name; queries read those tables back. The state is
// transient (never persisted to disk), matching Azure Monitor's append-only
// log ingestion model.
package monitorstore

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// Store holds all log tables keyed by table name. It is safe for concurrent
// use.
type Store struct {
	mu     sync.Mutex
	tables map[string][]Row
}

// New constructs an empty Store.
func New() *Store {
	return &Store{tables: make(map[string][]Row)}
}

// tableName derives the destination table name from a data-collection stream
// name. Azure custom streams are conventionally named "Custom-<Table>", whose
// records land in the "<Table>" table, so the leading "Custom-" prefix (if
// present) is stripped.
func tableName(stream string) string {
	return strings.TrimPrefix(stream, "Custom-")
}

// Ingest appends the given records to the table backing stream. Each record
// gains a synthetic TimeGenerated column (the ingestion time) when it does not
// already carry one.
func (s *Store) Ingest(stream string, records []Row) {
	now := time.Now().UTC().Format(time.RFC3339Nano)

	s.mu.Lock()
	defer s.mu.Unlock()

	name := tableName(stream)
	for _, rec := range records {
		if rec == nil {
			rec = Row{}
		}
		if _, ok := rec[columnTimeGenerated]; !ok {
			rec[columnTimeGenerated] = now
		}
		s.tables[name] = append(s.tables[name], rec)
	}
}

// Rows returns a copy of every record stored in the named table. The boolean
// reports whether the table exists.
func (s *Store) Rows(name string) ([]Row, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, ok := s.tables[name]
	if !ok {
		return nil, false
	}
	out := make([]Row, len(rows))
	for i, r := range rows {
		dup := make(Row, len(r))
		for k, v := range r {
			dup[k] = v
		}
		out[i] = dup
	}
	return out, true
}

// Tables returns the sorted names of every table that has received records.
func (s *Store) Tables() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	names := make([]string, 0, len(s.tables))
	for name := range s.tables {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
