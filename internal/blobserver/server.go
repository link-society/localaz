// Package blobserver implements the HTTP surface of the Azure Blob service,
// faithful enough that the official Azure SDKs and the Azure CLI work against
// it unmodified.
package blobserver

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"strings"

	"localaz.dev/internal/azerr"
	"localaz.dev/internal/blobstore"
)

// apiVersion is reported in the x-ms-version response header.
const apiVersion = "2021-08-06"

// Server routes Azure Blob REST requests onto a blobstore.Store.
type Server struct {
	store blobstore.Store
}

// New constructs a Server backed by the given store.
func New(store blobstore.Store) *Server {
	return &Server{store: store}
}

// request carries the parsed routing information for a single call.
type request struct {
	account   string
	container string
	blob      string
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
		req.container = segments[1]
	}
	if len(segments) >= 3 {
		req.blob = strings.Join(segments[2:], "/")
	}

	q := r.URL.Query()

	switch {
	case req.container == "":
		s.handleAccount(w, r, req, q)
	case req.blob == "":
		s.handleContainer(w, r, req, q)
	default:
		s.handleBlob(w, r, req, q)
	}
}

func (s *Server) handleAccount(w http.ResponseWriter, r *http.Request, req request, q queryValues) {
	switch {
	case q.Get("comp") == "list":
		s.listContainers(w, r, req, q)
	case q.Get("restype") == "service" && q.Get("comp") == "properties":
		s.getServiceProperties(w, r)
	default:
		s.writeError(w, r, azerr.New(http.StatusBadRequest, azerr.CodeInvalidQueryParameter,
			"The requested account-level operation is not supported."))
	}
}

func (s *Server) handleContainer(w http.ResponseWriter, r *http.Request, req request, q queryValues) {
	if q.Get("restype") != "container" {
		s.writeError(w, r, azerr.New(http.StatusBadRequest, azerr.CodeInvalidQueryParameter,
			"restype=container is required for container operations."))
		return
	}
	switch r.Method {
	case http.MethodPut:
		s.createContainer(w, r, req)
	case http.MethodDelete:
		s.deleteContainer(w, r, req)
	case http.MethodGet, http.MethodHead:
		if q.Get("comp") == "list" {
			s.listBlobs(w, r, req, q)
			return
		}
		s.getContainerProperties(w, r, req)
	default:
		s.methodNotAllowed(w, r)
	}
}

func (s *Server) handleBlob(w http.ResponseWriter, r *http.Request, req request, q queryValues) {
	switch r.Method {
	case http.MethodPut:
		switch q.Get("comp") {
		case "block":
			s.putBlock(w, r, req, q)
		case "blocklist":
			s.putBlockList(w, r, req)
		case "":
			s.putBlob(w, r, req)
		default:
			s.notImplemented(w, r)
		}
	case http.MethodGet:
		s.getBlob(w, r, req, false)
	case http.MethodHead:
		s.getBlob(w, r, req, true)
	case http.MethodDelete:
		s.deleteBlob(w, r, req)
	default:
		s.methodNotAllowed(w, r)
	}
}

// --- helpers ---

type queryValues interface {
	Get(string) string
}

func (s *Server) setCommonHeaders(w http.ResponseWriter) {
	w.Header().Set("Server", "localaz")
	w.Header().Set("x-ms-version", apiVersion)
	w.Header().Set("x-ms-request-id", newRequestID())
	w.Header().Set("Date", nowHTTP())
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

// mapStoreErr converts a storage sentinel error into an Azure error response,
// returning true if the error was handled.
func (s *Server) mapStoreErr(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case err == nil:
		return false
	case isErr(err, blobstore.ErrContainerNotFound):
		s.writeError(w, r, azerr.ContainerNotFound())
	case isErr(err, blobstore.ErrContainerExists):
		s.writeError(w, r, azerr.ContainerAlreadyExists())
	case isErr(err, blobstore.ErrBlobNotFound):
		s.writeError(w, r, azerr.BlobNotFound())
	case isErr(err, blobstore.ErrInvalidBlockList):
		s.writeError(w, r, azerr.InvalidBlockList())
	default:
		s.writeError(w, r, azerr.Internal(err.Error()))
	}
	return true
}

func (s *Server) methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	s.writeError(w, r, azerr.New(http.StatusMethodNotAllowed, azerr.CodeUnsupportedHeader,
		"The HTTP method is not supported for this resource."))
}

func (s *Server) notImplemented(w http.ResponseWriter, r *http.Request) {
	s.writeError(w, r, azerr.New(http.StatusNotImplemented, azerr.CodeUnsupportedHeader,
		"The requested operation is not yet implemented by the emulator."))
}

func (s *Server) getServiceProperties(w http.ResponseWriter, r *http.Request) {
	const body = xmlHeaderConst + "<StorageServiceProperties><DefaultServiceVersion>" +
		apiVersion + "</DefaultServiceVersion></StorageServiceProperties>"
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}

const xmlHeaderConst = `<?xml version="1.0" encoding="utf-8"?>`

func newRequestID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
