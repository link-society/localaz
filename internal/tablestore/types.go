package tablestore

import (
	"encoding/json"
	"errors"
	"time"
)

// Sentinel errors returned by the store. The HTTP layer maps these onto the
// appropriate Azure error responses.
var (
	ErrTableNotFound  = errors.New("tablestore: table not found")
	ErrTableExists    = errors.New("tablestore: table already exists")
	ErrEntityNotFound = errors.New("tablestore: entity not found")
	ErrEntityExists   = errors.New("tablestore: entity already exists")
	ErrETagMismatch   = errors.New("tablestore: etag mismatch")
)

// Entity is a stored table entity. Props holds every user-supplied property
// (including PartitionKey and RowKey) verbatim as raw JSON so typed values and
// their @odata.type annotations round-trip. Timestamp and ETag are
// server-managed.
type Entity struct {
	PartitionKey string                     `json:"pk"`
	RowKey       string                     `json:"rk"`
	Props        map[string]json.RawMessage `json:"props"`
	Timestamp    time.Time                  `json:"timestamp"`
	ETag         string                     `json:"etag"`
}

// table is the in-memory representation of a single table. Entities are keyed
// by partition key then row key.
type table struct {
	Name     string                        `json:"name"`
	Entities map[string]map[string]*Entity `json:"entities"`
}

func newTable(name string) *table {
	return &table{Name: name, Entities: map[string]map[string]*Entity{}}
}

func (t *table) get(pk, rk string) (*Entity, bool) {
	rows, ok := t.Entities[pk]
	if !ok {
		return nil, false
	}
	e, ok := rows[rk]
	return e, ok
}

func (t *table) put(e *Entity) {
	rows, ok := t.Entities[e.PartitionKey]
	if !ok {
		rows = map[string]*Entity{}
		t.Entities[e.PartitionKey] = rows
	}
	rows[e.RowKey] = e
}

func (t *table) delete(pk, rk string) bool {
	rows, ok := t.Entities[pk]
	if !ok {
		return false
	}
	if _, ok := rows[rk]; !ok {
		return false
	}
	delete(rows, rk)
	if len(rows) == 0 {
		delete(t.Entities, pk)
	}
	return true
}
