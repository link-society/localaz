package wpsserver

import (
	"encoding/base64"
	"encoding/json"
)

// Web PubSub protocol message types for the json.webpubsub.azure.v1
// subprotocol. Only the subset needed for pub/sub fidelity is modelled.

// systemMessage is sent by the server on connect (and on disconnect).
type systemMessage struct {
	Type         string `json:"type"`
	Event        string `json:"event"`
	ConnectionID string `json:"connectionId,omitempty"`
	UserID       string `json:"userId,omitempty"`
}

// ackMessage acknowledges a client request that carried an ackId.
type ackMessage struct {
	Type    string  `json:"type"`
	AckID   uint64  `json:"ackId"`
	Success bool    `json:"success"`
	Error   *ackErr `json:"error,omitempty"`
}

type ackErr struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

// dataMessage carries data pushed to the client from the server or a group.
type dataMessage struct {
	Type     string          `json:"type"`
	From     string          `json:"from"`
	Group    string          `json:"group,omitempty"`
	DataType string          `json:"dataType"`
	Data     json.RawMessage `json:"data"`
}

// clientMessage is the envelope the client sends to the server. Only the
// fields needed to dispatch are decoded; data is kept raw.
type clientMessage struct {
	Type     string          `json:"type"`
	Group    string          `json:"group"`
	AckID    *uint64         `json:"ackId"`
	DataType string          `json:"dataType"`
	Data     json.RawMessage `json:"data"`
	Event    string          `json:"event"`
}

// dataTypeFor maps an HTTP content type used by the REST publish API onto the
// dataType field of a downstream WebSocket message.
func dataTypeFor(contentType string) string {
	switch {
	case contentType == "" || contentType == "application/json":
		return "json"
	case contentType == "text/plain":
		return "text"
	default:
		return "binary"
	}
}

// encodeData renders a raw HTTP publish body as the JSON value of the data
// field for the given dataType.
func encodeData(dataType string, body []byte) json.RawMessage {
	switch dataType {
	case "json":
		if json.Valid(body) {
			return json.RawMessage(body)
		}
		// Fall back to a JSON string if the body is not valid JSON.
		encoded, _ := json.Marshal(string(body))
		return encoded
	case "binary":
		encoded, _ := json.Marshal(base64.StdEncoding.EncodeToString(body))
		return encoded
	default:
		encoded, _ := json.Marshal(string(body))
		return encoded
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
