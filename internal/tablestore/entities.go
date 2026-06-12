package tablestore

import (
	"encoding/json"
	"time"
)

// serverProps are response-only annotations that must never be stored as user
// data; they are stripped from incoming entity bodies.
var serverProps = map[string]struct{}{
	"Timestamp":      {},
	"odata.metadata": {},
	"odata.etag":     {},
	"odata.type":     {},
	"odata.id":       {},
	"odata.editLink": {},
}

// InsertEntity adds a new entity, returning ErrEntityExists if one already has
// the same keys.
func (s *Store) InsertEntity(account, name, pk, rk string, props map[string]json.RawMessage) (*Entity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, err := s.lookupLocked(account, name)
	if err != nil {
		return nil, err
	}
	if _, ok := t.get(pk, rk); ok {
		return nil, ErrEntityExists
	}
	e := s.buildEntity(pk, rk, props)
	t.put(e)
	s.persistLocked()
	return cloneEntity(e), nil
}

// GetEntity returns a single entity by key.
func (s *Store) GetEntity(account, name, pk, rk string) (*Entity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, err := s.lookupLocked(account, name)
	if err != nil {
		return nil, err
	}
	e, ok := t.get(pk, rk)
	if !ok {
		return nil, ErrEntityNotFound
	}
	return cloneEntity(e), nil
}

// ListEntities returns every entity in a table, ordered by partition then row
// key. Filtering, projection and limiting are applied by the HTTP layer.
func (s *Store) ListEntities(account, name string) ([]*Entity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, err := s.lookupLocked(account, name)
	if err != nil {
		return nil, err
	}
	var out []*Entity
	for _, rows := range t.Entities {
		for _, e := range rows {
			out = append(out, cloneEntity(e))
		}
	}
	sortEntities(out)
	return out, nil
}

// ReplaceEntity performs an insert-or-replace (PUT). ifMatch governs concurrency
// (see matchLocked): empty means upsert, "*" requires existence, an etag
// requires an exact match.
func (s *Store) ReplaceEntity(account, name, pk, rk string, props map[string]json.RawMessage, ifMatch string) (*Entity, error) {
	return s.update(account, name, pk, rk, props, ifMatch, false)
}

// MergeEntity performs an insert-or-merge (MERGE).
func (s *Store) MergeEntity(account, name, pk, rk string, props map[string]json.RawMessage, ifMatch string) (*Entity, error) {
	return s.update(account, name, pk, rk, props, ifMatch, true)
}

func (s *Store) update(account, name, pk, rk string, props map[string]json.RawMessage, ifMatch string, merge bool) (*Entity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, err := s.lookupLocked(account, name)
	if err != nil {
		return nil, err
	}
	existing, ok := t.get(pk, rk)
	if !ok {
		if ifMatch != "" && ifMatch != "*" {
			return nil, ErrEntityNotFound
		}
		if ifMatch == "*" {
			return nil, ErrEntityNotFound
		}
		e := s.buildEntity(pk, rk, props)
		t.put(e)
		s.persistLocked()
		return cloneEntity(e), nil
	}
	if err := matchETag(existing, ifMatch); err != nil {
		return nil, err
	}

	var e *Entity
	if merge {
		merged := map[string]json.RawMessage{}
		for k, v := range existing.Props {
			merged[k] = v
		}
		for k, v := range stripServerProps(props) {
			merged[k] = v
		}
		e = s.buildEntity(pk, rk, merged)
	} else {
		e = s.buildEntity(pk, rk, props)
	}
	t.put(e)
	s.persistLocked()
	return cloneEntity(e), nil
}

// DeleteEntity removes an entity, honouring an If-Match condition.
func (s *Store) DeleteEntity(account, name, pk, rk, ifMatch string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, err := s.lookupLocked(account, name)
	if err != nil {
		return err
	}
	existing, ok := t.get(pk, rk)
	if !ok {
		return ErrEntityNotFound
	}
	if err := matchETag(existing, ifMatch); err != nil {
		return err
	}
	t.delete(pk, rk)
	s.persistLocked()
	return nil
}

// buildEntity constructs a stored entity with server-managed timestamp and
// etag, ensuring the key properties are present. Caller holds s.mu.
func (s *Store) buildEntity(pk, rk string, props map[string]json.RawMessage) *Entity {
	clean := stripServerProps(props)
	clean["PartitionKey"] = mustJSON(pk)
	clean["RowKey"] = mustJSON(rk)
	now := time.Now().UTC()
	return &Entity{
		PartitionKey: pk,
		RowKey:       rk,
		Props:        clean,
		Timestamp:    now,
		ETag:         etagFor(now),
	}
}

// matchETag reports whether ifMatch is satisfied by the entity. Empty or "*"
// always match an existing entity; otherwise the etag must be identical.
func matchETag(e *Entity, ifMatch string) error {
	if ifMatch == "" || ifMatch == "*" {
		return nil
	}
	if ifMatch != e.ETag {
		return ErrETagMismatch
	}
	return nil
}

func stripServerProps(props map[string]json.RawMessage) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(props))
	for k, v := range props {
		if _, skip := serverProps[k]; skip {
			continue
		}
		out[k] = v
	}
	return out
}

func mustJSON(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}

func cloneEntity(e *Entity) *Entity {
	c := *e
	c.Props = make(map[string]json.RawMessage, len(e.Props))
	for k, v := range e.Props {
		c.Props[k] = v
	}
	return &c
}
