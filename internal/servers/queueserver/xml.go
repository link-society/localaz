package queueserver

import (
	"encoding/xml"

	"localaz.dev/internal/stores/queuestore"
	"localaz.dev/internal/utils/azwire"
)

// --- List Queues ---

type enumQueuesResult struct {
	XMLName         xml.Name  `xml:"EnumerationResults"`
	ServiceEndpoint string    `xml:"ServiceEndpoint,attr"`
	Prefix          string    `xml:"Prefix,omitempty"`
	Queues          xmlQueues `xml:"Queues"`
	NextMarker      string    `xml:"NextMarker"`
}

type xmlQueues struct {
	Items []xmlQueue `xml:"Queue"`
}

type xmlQueue struct {
	Name     string            `xml:"Name"`
	Metadata map[string]string `xml:"-"`
}

func marshalQueues(endpoint, prefix string, items []queuestore.QueueInfo) ([]byte, error) {
	res := enumQueuesResult{ServiceEndpoint: endpoint, Prefix: prefix}
	for _, q := range items {
		res.Queues.Items = append(res.Queues.Items, xmlQueue{Name: q.Name})
	}
	return azwire.MarshalXML(res)
}

// --- Messages ---

// messageView is the rendered form of a message for one of the three response
// shapes (enqueue, dequeue, peek). Optional fields are left empty to be omitted.
type messageView struct {
	MessageID       string
	InsertionTime   string
	ExpirationTime  string
	PopReceipt      string
	TimeNextVisible string
	DequeueCount    *int64
	MessageText     string
}

type xmlQueueMessage struct {
	MessageID       string `xml:"MessageId"`
	InsertionTime   string `xml:"InsertionTime"`
	ExpirationTime  string `xml:"ExpirationTime"`
	PopReceipt      string `xml:"PopReceipt,omitempty"`
	TimeNextVisible string `xml:"TimeNextVisible,omitempty"`
	DequeueCount    *int64 `xml:"DequeueCount,omitempty"`
	MessageText     string `xml:"MessageText,omitempty"`
}

type xmlQueueMessagesList struct {
	XMLName  xml.Name          `xml:"QueueMessagesList"`
	Messages []xmlQueueMessage `xml:"QueueMessage"`
}

func marshalMessages(views []messageView) ([]byte, error) {
	list := xmlQueueMessagesList{}
	for _, v := range views {
		list.Messages = append(list.Messages, xmlQueueMessage(v))
	}
	return azwire.MarshalXML(list)
}

func enqueuedView(m *queuestore.Message) messageView {
	return messageView{
		MessageID:       m.ID,
		InsertionTime:   azwire.FormatHTTP(m.InsertionTime),
		ExpirationTime:  expiration(m),
		PopReceipt:      m.PopReceipt,
		TimeNextVisible: azwire.FormatHTTP(m.NextVisible),
	}
}

func dequeuedView(m *queuestore.Message) messageView {
	count := int64(m.DequeueCount)
	return messageView{
		MessageID:       m.ID,
		InsertionTime:   azwire.FormatHTTP(m.InsertionTime),
		ExpirationTime:  expiration(m),
		PopReceipt:      m.PopReceipt,
		TimeNextVisible: azwire.FormatHTTP(m.NextVisible),
		DequeueCount:    &count,
		MessageText:     m.Text,
	}
}

func peekView(m *queuestore.Message) messageView {
	count := int64(m.DequeueCount)
	return messageView{
		MessageID:      m.ID,
		InsertionTime:  azwire.FormatHTTP(m.InsertionTime),
		ExpirationTime: expiration(m),
		DequeueCount:   &count,
		MessageText:    m.Text,
	}
}

func expiration(m *queuestore.Message) string {
	if m.ExpirationTime.IsZero() {
		return ""
	}
	return azwire.FormatHTTP(m.ExpirationTime)
}
