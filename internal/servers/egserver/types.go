package egserver

import "encoding/json"

// receiveResult is the body returned by the :receive operation.
type receiveResult struct {
	Value []receiveDetail `json:"value"`
}

// receiveDetail couples broker metadata with the original CloudEvent payload.
type receiveDetail struct {
	BrokerProperties brokerProperties `json:"brokerProperties"`
	Event            json.RawMessage  `json:"event"`
}

// brokerProperties carries the lock token and delivery count for a received
// event.
type brokerProperties struct {
	LockToken     string `json:"lockToken"`
	DeliveryCount int    `json:"deliveryCount"`
}

// lockTokensRequest is the body of the acknowledge / release / reject / renew
// operations.
type lockTokensRequest struct {
	LockTokens []string `json:"lockTokens"`
}

// lockResult is the body returned by the acknowledge / release / reject / renew
// operations.
type lockResult struct {
	SucceededLockTokens []string          `json:"succeededLockTokens"`
	FailedLockTokens    []failedLockToken `json:"failedLockTokens"`
}

// failedLockToken describes a token that could not be resolved.
type failedLockToken struct {
	LockToken string    `json:"lockToken"`
	Error     lockError `json:"error"`
}

// lockError is the error detail attached to a failed lock token.
type lockError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
