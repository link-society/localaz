package tablestore

import "sort"

// CreateTable creates a table, returning ErrTableExists if it already exists.
func (s *Store) CreateTable(account, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ts := s.accounts[account]
	if ts == nil {
		ts = map[string]*table{}
		s.accounts[account] = ts
	}
	if _, ok := ts[name]; ok {
		return ErrTableExists
	}
	ts[name] = newTable(name)
	s.persistLocked()
	return nil
}

// DeleteTable removes a table and all its entities.
func (s *Store) DeleteTable(account, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ts := s.accounts[account]
	if ts == nil {
		return ErrTableNotFound
	}
	if _, ok := ts[name]; !ok {
		return ErrTableNotFound
	}
	delete(ts, name)
	s.persistLocked()
	return nil
}

// ListTables returns the account's table names, sorted.
func (s *Store) ListTables(account string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []string
	for name := range s.accounts[account] {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
