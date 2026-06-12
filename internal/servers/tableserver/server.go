// Package tableserver implements the HTTP surface of the Azure Table service,
// faithful enough that the official Azure SDKs (aztables) and the Azure CLI
// work against it unmodified. Errors are reported as OData JSON, the format the
// Table service uses (unlike the XML used by Blob and Queue).
package tableserver

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"localaz.dev/internal/stores/tablestore"
	"localaz.dev/internal/utils/azerr"
	"localaz.dev/internal/utils/azwire"
)

// apiVersion is reported in the x-ms-version response header.
const apiVersion = "2019-02-02"

// Server routes Azure Table REST requests onto a tablestore.Store.
type Server struct {
	store *tablestore.Store
}

// New constructs a Server backed by the given store.
func New(store *tablestore.Store) *Server {
	return &Server{store: store}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.setCommonHeaders(w)

	segments := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if len(segments) == 0 || segments[0] == "" {
		s.writeError(w, r, azerr.New(http.StatusBadRequest, azerr.CodeInvalidInput, "Missing account in request URL."))
		return
	}
	account := segments[0]
	if len(segments) < 2 || segments[1] == "" {
		s.writeError(w, r, azerr.New(http.StatusBadRequest, azerr.CodeInvalidInput, "Missing resource in request URL."))
		return
	}

	resource := segments[1]
	switch {
	case resource == "Tables":
		s.handleTableCollection(w, r, account)
	case strings.HasPrefix(resource, "Tables("):
		name := unquoteKey(insideParens(resource))
		if name == "" {
			s.handleTableCollection(w, r, account)
			return
		}
		s.handleTableItem(w, r, account, name)
	default:
		name, predicate := splitPredicate(resource)
		if predicate == "" {
			s.handleEntityCollection(w, r, account, name)
			return
		}
		pk, rk := parseEntityKeys(predicate)
		s.handleEntityItem(w, r, account, name, pk, rk)
	}
}

func (s *Server) handleTableCollection(w http.ResponseWriter, r *http.Request, account string) {
	switch r.Method {
	case http.MethodPost:
		s.createTable(w, r, account)
	case http.MethodGet:
		s.listTables(w, r, account)
	default:
		s.methodNotAllowed(w, r)
	}
}

func (s *Server) handleTableItem(w http.ResponseWriter, r *http.Request, account, name string) {
	if r.Method == http.MethodDelete {
		s.deleteTable(w, r, account, name)
		return
	}
	s.methodNotAllowed(w, r)
}

func (s *Server) handleEntityCollection(w http.ResponseWriter, r *http.Request, account, name string) {
	switch r.Method {
	case http.MethodPost:
		s.insertEntity(w, r, account, name)
	case http.MethodGet:
		s.listEntities(w, r, account, name)
	default:
		s.methodNotAllowed(w, r)
	}
}

func (s *Server) handleEntityItem(w http.ResponseWriter, r *http.Request, account, name, pk, rk string) {
	switch r.Method {
	case http.MethodGet:
		s.getEntity(w, r, account, name, pk, rk)
	case http.MethodPut:
		s.updateEntity(w, r, account, name, pk, rk, false)
	case http.MethodPatch, "MERGE":
		s.updateEntity(w, r, account, name, pk, rk, true)
	case http.MethodDelete:
		s.deleteEntity(w, r, account, name, pk, rk)
	default:
		s.methodNotAllowed(w, r)
	}
}

// --- helpers ---

func (s *Server) setCommonHeaders(w http.ResponseWriter) {
	w.Header().Set("Server", "localaz")
	w.Header().Set("x-ms-version", apiVersion)
	w.Header().Set("x-ms-request-id", newRequestID())
	w.Header().Set("Date", azwire.NowHTTP())
	w.Header().Set("Cache-Control", "no-cache")
}

func (s *Server) metadataBase(r *http.Request, account string) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/%s/$metadata", scheme, r.Host, account)
}

func (s *Server) writeError(w http.ResponseWriter, r *http.Request, e *azerr.Error) {
	w.Header().Set("x-ms-error-code", string(e.Code))
	w.Header().Set("Content-Type", "application/json;odata=minimalmetadata;charset=utf-8")
	w.WriteHeader(e.Status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(e.JSON())
	}
}

// mapStoreErr converts a store sentinel error into an Azure error response,
// returning true if the error was handled.
func (s *Server) mapStoreErr(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, tablestore.ErrTableNotFound):
		s.writeError(w, r, azerr.TableNotFound())
	case errors.Is(err, tablestore.ErrTableExists):
		s.writeError(w, r, azerr.TableAlreadyExists())
	case errors.Is(err, tablestore.ErrEntityNotFound):
		s.writeError(w, r, azerr.EntityNotFound())
	case errors.Is(err, tablestore.ErrEntityExists):
		s.writeError(w, r, azerr.EntityAlreadyExists())
	case errors.Is(err, tablestore.ErrETagMismatch):
		s.writeError(w, r, azerr.UpdateConditionNotMet())
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
