package queuestore

import (
	"sort"
	"strings"
	"time"
)

// CreateQueue creates a queue. Azure semantics: creating an existing queue with
// identical metadata is a no-op (no error); with different metadata it
// conflicts. Returns created=true only when a new queue was added.
func (s *Store) CreateQueue(account, name string, metadata map[string]string) (created bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	qs := s.accounts[account]
	if qs == nil {
		qs = map[string]*queue{}
		s.accounts[account] = qs
	}
	if existing, ok := qs[name]; ok {
		if !sameMetadata(existing.Metadata, metadata) {
			return false, ErrQueueExists
		}
		return false, nil
	}
	qs[name] = &queue{Name: name, Metadata: cloneMetadata(metadata)}
	s.persistLocked()
	return true, nil
}

// DeleteQueue removes a queue and all its messages.
func (s *Store) DeleteQueue(account, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	qs := s.accounts[account]
	if qs == nil {
		return ErrQueueNotFound
	}
	if _, ok := qs[name]; !ok {
		return ErrQueueNotFound
	}
	delete(qs, name)
	s.persistLocked()
	return nil
}

// ListQueues returns the account's queues whose names start with prefix,
// sorted by name.
func (s *Store) ListQueues(account, prefix string) []QueueInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	var out []QueueInfo
	for name, q := range s.accounts[account] {
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}
		q.evictExpired(now)
		out = append(out, QueueInfo{
			Name:             name,
			Metadata:         cloneMetadata(q.Metadata),
			ApproximateCount: len(q.Messages),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// GetMetadata returns a queue's metadata and approximate message count.
func (s *Store) GetMetadata(account, name string) (QueueInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	q, err := s.lookupLocked(account, name)
	if err != nil {
		return QueueInfo{}, err
	}
	return QueueInfo{
		Name:             name,
		Metadata:         cloneMetadata(q.Metadata),
		ApproximateCount: len(q.Messages),
	}, nil
}

// SetMetadata replaces a queue's metadata.
func (s *Store) SetMetadata(account, name string, metadata map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	q, err := s.lookupLocked(account, name)
	if err != nil {
		return err
	}
	q.Metadata = cloneMetadata(metadata)
	s.persistLocked()
	return nil
}

func sameMetadata(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func cloneMetadata(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
