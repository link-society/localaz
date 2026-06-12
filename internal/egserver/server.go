// Package egserver implements the HTTP surface of the Azure Event Grid
// namespace (pull-delivery) data plane, faithful enough that the official
// aznamespaces SDK works against it unmodified.
package egserver

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"localaz.dev/internal/egstore"
)

// Server routes Event Grid namespace requests onto an egstore.Store.
type Server struct {
	store *egstore.Store
}

// New constructs a Server backed by the given store.
func New(store *egstore.Store) *Server {
	return &Server{store: store}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "Only POST is supported.")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/")
	colon := strings.LastIndex(path, ":")
	if colon < 0 {
		writeError(w, http.StatusNotFound, "NotFound", "Unknown operation.")
		return
	}
	action := path[colon+1:]
	segments := strings.Split(path[:colon], "/")

	// Expected shapes:
	//   topics/{topic}:publish
	//   topics/{topic}/eventsubscriptions/{sub}:{action}
	if len(segments) < 2 || segments[0] != "topics" {
		writeError(w, http.StatusNotFound, "NotFound", "Unknown resource.")
		return
	}
	topic := segments[1]

	if action == "publish" {
		s.handlePublish(w, r, topic)
		return
	}

	if len(segments) < 4 || segments[2] != "eventsubscriptions" {
		writeError(w, http.StatusNotFound, "NotFound", "Unknown resource.")
		return
	}
	sub := segments[3]

	switch action {
	case "receive":
		s.handleReceive(w, r, topic, sub)
	case "acknowledge":
		s.handleLockAction(w, r, topic, sub, s.store.Acknowledge)
	case "release":
		s.handleLockAction(w, r, topic, sub, s.store.Release)
	case "reject":
		s.handleLockAction(w, r, topic, sub, s.store.Reject)
	case "renewLock":
		s.handleLockAction(w, r, topic, sub, s.store.RenewLocks)
	default:
		writeError(w, http.StatusNotFound, "NotFound", "Unknown operation.")
	}
}

func (s *Server) handlePublish(w http.ResponseWriter, r *http.Request, topic string) {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)

	var events []json.RawMessage
	if strings.Contains(r.Header.Get("Content-Type"), "batch") {
		if err := dec.Decode(&events); err != nil {
			writeError(w, http.StatusBadRequest, "InvalidRequest", "Malformed CloudEvent batch.")
			return
		}
	} else {
		var one json.RawMessage
		if err := dec.Decode(&one); err != nil {
			writeError(w, http.StatusBadRequest, "InvalidRequest", "Malformed CloudEvent.")
			return
		}
		events = []json.RawMessage{one}
	}

	s.store.Publish(topic, events)
	writeJSON(w, http.StatusOK, struct{}{})
}

func (s *Server) handleReceive(w http.ResponseWriter, r *http.Request, topic, sub string) {
	maxEvents := 1
	if v := r.URL.Query().Get("maxEvents"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxEvents = n
		}
	}

	received := s.store.Receive(topic, sub, maxEvents)
	details := make([]receiveDetail, 0, len(received))
	for _, rec := range received {
		details = append(details, receiveDetail{
			BrokerProperties: brokerProperties{
				LockToken:     rec.LockToken,
				DeliveryCount: rec.DeliveryCount,
			},
			Event: rec.Event,
		})
	}
	writeJSON(w, http.StatusOK, receiveResult{Value: details})
}

func (s *Server) handleLockAction(w http.ResponseWriter, r *http.Request, topic, sub string, op func(string, string, []string) egstore.LockResult) {
	defer r.Body.Close()
	var body lockTokensRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "InvalidRequest", "Malformed lock token request.")
		return
	}

	res := op(topic, sub, body.LockTokens)
	out := lockResult{
		SucceededLockTokens: res.Succeeded,
		FailedLockTokens:    make([]failedLockToken, 0, len(res.Failed)),
	}
	if out.SucceededLockTokens == nil {
		out.SucceededLockTokens = []string{}
	}
	for _, tok := range res.Failed {
		out.FailedLockTokens = append(out.FailedLockTokens, failedLockToken{
			LockToken: tok,
			Error: lockError{
				Code:    "TokenLost",
				Message: "The lock token is invalid or has already been resolved.",
			},
		})
	}
	writeJSON(w, http.StatusOK, out)
}
