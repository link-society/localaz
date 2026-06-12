// Package queueserver implements the HTTP surface of the Azure Queue service,
// faithful enough that the official Azure SDKs (azqueue) and the Azure CLI work
// against it unmodified.
package queueserver

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"localaz.dev/internal/stores/queuestore"
	"localaz.dev/internal/utils/azerr"
	"localaz.dev/internal/utils/azwire"
)

// apiVersion is reported in the x-ms-version response header.
const apiVersion = "2021-08-06"

// Server routes Azure Queue REST requests onto a queuestore.Store.
type Server struct {
	store *queuestore.Store
}

// New constructs a Server backed by the given store.
func New(store *queuestore.Store) *Server {
	return &Server{store: store}
}

// request carries the parsed routing information for a single call.
type request struct {
	account   string
	queue     string
	messages  bool   // path contains the /messages segment
	messageID string // set for /messages/{id}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.setCommonHeaders(w)

	segments := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if len(segments) == 0 || segments[0] == "" {
		s.writeError(w, r, azerr.New(http.StatusBadRequest, azerr.CodeInvalidInput, "Missing account in request URL."))
		return
	}

	req := request{account: segments[0]}
	if len(segments) >= 2 {
		req.queue = segments[1]
	}
	if len(segments) >= 3 && segments[2] == "messages" {
		req.messages = true
	}
	if len(segments) >= 4 && segments[2] == "messages" {
		req.messageID = segments[3]
	}

	q := r.URL.Query()

	switch {
	case req.queue == "":
		s.handleAccount(w, r, req, q)
	case req.messages:
		s.handleMessages(w, r, req, q)
	default:
		s.handleQueue(w, r, req, q)
	}
}

func (s *Server) handleAccount(w http.ResponseWriter, r *http.Request, req request, q url) {
	if q.Get("comp") == "list" {
		s.listQueues(w, r, req, q)
		return
	}
	s.writeError(w, r, azerr.New(http.StatusBadRequest, azerr.CodeInvalidQueryParameter,
		"The requested account-level operation is not supported."))
}

func (s *Server) handleQueue(w http.ResponseWriter, r *http.Request, req request, q url) {
	switch r.Method {
	case http.MethodPut:
		if q.Get("comp") == "metadata" {
			s.setMetadata(w, r, req)
			return
		}
		s.createQueue(w, r, req)
	case http.MethodDelete:
		s.deleteQueue(w, r, req)
	case http.MethodGet, http.MethodHead:
		s.getMetadata(w, r, req)
	default:
		s.methodNotAllowed(w, r)
	}
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request, req request, q url) {
	if req.messageID != "" {
		switch r.Method {
		case http.MethodDelete:
			s.deleteMessage(w, r, req, q)
		case http.MethodPut:
			s.updateMessage(w, r, req, q)
		default:
			s.methodNotAllowed(w, r)
		}
		return
	}
	switch r.Method {
	case http.MethodPost:
		s.putMessage(w, r, req, q)
	case http.MethodGet:
		s.getMessages(w, r, req, q)
	case http.MethodDelete:
		s.clearMessages(w, r, req)
	default:
		s.methodNotAllowed(w, r)
	}
}

// --- helpers ---

// url is the subset of url.Values the handlers need.
type url interface {
	Get(string) string
}

func (s *Server) setCommonHeaders(w http.ResponseWriter) {
	w.Header().Set("Server", "localaz")
	w.Header().Set("x-ms-version", apiVersion)
	w.Header().Set("x-ms-request-id", newRequestID())
	w.Header().Set("Date", azwire.NowHTTP())
}

func (s *Server) serviceEndpoint(r *http.Request, account string) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/%s", scheme, r.Host, account)
}

func (s *Server) writeError(w http.ResponseWriter, r *http.Request, e *azerr.Error) {
	w.Header().Set("x-ms-error-code", string(e.Code))
	w.Header().Set("Content-Type", "application/xml")
	body, err := e.XML()
	if err != nil {
		http.Error(w, e.Message, e.Status)
		return
	}
	w.WriteHeader(e.Status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}

// mapStoreErr converts a store sentinel error into an Azure error response,
// returning true if the error was handled.
func (s *Server) mapStoreErr(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, queuestore.ErrQueueNotFound):
		s.writeError(w, r, azerr.QueueNotFound())
	case errors.Is(err, queuestore.ErrQueueExists):
		s.writeError(w, r, azerr.QueueAlreadyExists())
	case errors.Is(err, queuestore.ErrMessageNotFound):
		s.writeError(w, r, azerr.MessageNotFound())
	case errors.Is(err, queuestore.ErrPopReceipt):
		s.writeError(w, r, azerr.PopReceiptMismatch())
	default:
		s.writeError(w, r, azerr.Internal(err.Error()))
	}
	return true
}

func (s *Server) methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	s.writeError(w, r, azerr.New(http.StatusMethodNotAllowed, azerr.CodeUnsupportedHeader,
		"The HTTP method is not supported for this resource."))
}

func newRequestID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
