package queuestore

import (
	"errors"
	"time"
)

// Sentinel errors returned by the store. The HTTP layer maps these onto the
// appropriate Azure error responses.
var (
	ErrQueueNotFound   = errors.New("queuestore: queue not found")
	ErrQueueExists     = errors.New("queuestore: queue already exists")
	ErrMessageNotFound = errors.New("queuestore: message not found")
	ErrPopReceipt      = errors.New("queuestore: pop receipt mismatch")
)

// QueueInfo is the summary view of a queue used by listings and metadata reads.
type QueueInfo struct {
	Name             string
	Metadata         map[string]string
	ApproximateCount int
}

// Message is a single queue message and its server-managed visibility state.
type Message struct {
	ID             string    `json:"id"`
	Text           string    `json:"text"`
	InsertionTime  time.Time `json:"insertion_time"`
	ExpirationTime time.Time `json:"expiration_time"`
	NextVisible    time.Time `json:"next_visible"`
	PopReceipt     string    `json:"pop_receipt"`
	DequeueCount   int       `json:"dequeue_count"`
}

// visible reports whether the message can be dequeued or peeked at the instant
// now.
func (m *Message) visible(now time.Time) bool {
	return !now.Before(m.NextVisible)
}

// expired reports whether the message has passed its time-to-live.
func (m *Message) expired(now time.Time) bool {
	return !m.ExpirationTime.IsZero() && now.After(m.ExpirationTime)
}

// queue is the in-memory representation of a single queue.
type queue struct {
	Name     string            `json:"name"`
	Metadata map[string]string `json:"metadata"`
	Messages []*Message        `json:"messages"`
}
