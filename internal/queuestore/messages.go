package queuestore

import "time"

// defaultTTL is Azure's default message time-to-live (7 days) when the caller
// does not specify one.
const defaultTTL = 7 * 24 * time.Hour

// Enqueue appends a message. visibilityTimeout delays the message's first
// visibility; ttl bounds its lifetime (use 0 for the 7-day default, negative
// for infinite).
func (s *Store) Enqueue(account, name, text string, visibilityTimeout, ttl time.Duration) (*Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	q, err := s.lookupLocked(account, name)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	m := &Message{
		ID:            newID(),
		Text:          text,
		InsertionTime: now,
		NextVisible:   now.Add(visibilityTimeout),
	}
	switch {
	case ttl < 0:
		// Infinite: leave ExpirationTime zero.
	case ttl == 0:
		m.ExpirationTime = now.Add(defaultTTL)
	default:
		m.ExpirationTime = now.Add(ttl)
	}
	q.Messages = append(q.Messages, m)
	s.persistLocked()
	return cloneMessage(m), nil
}

// Dequeue returns up to num currently visible messages, hiding each for
// visibilityTimeout, assigning a fresh pop receipt and incrementing its dequeue
// count.
func (s *Store) Dequeue(account, name string, num int, visibilityTimeout time.Duration) ([]*Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	q, err := s.lookupLocked(account, name)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	var out []*Message
	for _, m := range q.Messages {
		if len(out) >= num {
			break
		}
		if !m.visible(now) {
			continue
		}
		m.DequeueCount++
		m.PopReceipt = newPopReceipt()
		m.NextVisible = now.Add(visibilityTimeout)
		out = append(out, cloneMessage(m))
	}
	if len(out) > 0 {
		s.persistLocked()
	}
	return out, nil
}

// Peek returns up to num currently visible messages without changing their
// visibility or pop receipt.
func (s *Store) Peek(account, name string, num int) ([]*Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	q, err := s.lookupLocked(account, name)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	var out []*Message
	for _, m := range q.Messages {
		if len(out) >= num {
			break
		}
		if !m.visible(now) {
			continue
		}
		out = append(out, cloneMessage(m))
	}
	return out, nil
}

// DeleteMessage removes a message, requiring a matching pop receipt.
func (s *Store) DeleteMessage(account, name, id, popReceipt string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	q, err := s.lookupLocked(account, name)
	if err != nil {
		return err
	}
	for i, m := range q.Messages {
		if m.ID != id {
			continue
		}
		if m.PopReceipt != popReceipt {
			return ErrPopReceipt
		}
		q.Messages = append(q.Messages[:i], q.Messages[i+1:]...)
		s.persistLocked()
		return nil
	}
	return ErrMessageNotFound
}

// UpdateMessage updates a message's text and visibility, requiring a matching
// pop receipt, and returns the new pop receipt and next-visible time.
func (s *Store) UpdateMessage(account, name, id, popReceipt, text string, visibilityTimeout time.Duration) (string, time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	q, err := s.lookupLocked(account, name)
	if err != nil {
		return "", time.Time{}, err
	}
	for _, m := range q.Messages {
		if m.ID != id {
			continue
		}
		if m.PopReceipt != popReceipt {
			return "", time.Time{}, ErrPopReceipt
		}
		if text != "" {
			m.Text = text
		}
		now := time.Now().UTC()
		m.PopReceipt = newPopReceipt()
		m.NextVisible = now.Add(visibilityTimeout)
		s.persistLocked()
		return m.PopReceipt, m.NextVisible, nil
	}
	return "", time.Time{}, ErrMessageNotFound
}

// ClearMessages removes every message from a queue.
func (s *Store) ClearMessages(account, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	q, err := s.lookupLocked(account, name)
	if err != nil {
		return err
	}
	q.Messages = nil
	s.persistLocked()
	return nil
}

func cloneMessage(m *Message) *Message {
	c := *m
	return &c
}
