package blobserver

import (
	"net/http"
	"strings"

	"localaz.dev/internal/azerr"
)

// readMetadata extracts x-ms-meta-* headers into a metadata map. Keys are
// returned without the prefix, preserving Azure's case-insensitive semantics by
// lower-casing them.
func readMetadata(h http.Header) map[string]string {
	const prefix = "X-Ms-Meta-"
	meta := map[string]string{}
	for k, vals := range h {
		if len(k) > len(prefix) && strings.EqualFold(k[:len(prefix)], prefix) {
			meta[strings.ToLower(k[len(prefix):])] = strings.Join(vals, ",")
		}
	}
	return meta
}

func writeMetadata(h http.Header, meta map[string]string) {
	for k, v := range meta {
		h.Set("x-ms-meta-"+k, v)
	}
}

func (s *Server) createContainer(w http.ResponseWriter, r *http.Request, req request) {
	if !validContainerName(req.container) {
		s.writeError(w, r, azerr.InvalidResourceName())
		return
	}
	info, err := s.store.CreateContainer(req.account, req.container, readMetadata(r.Header))
	if s.mapStoreErr(w, r, err) {
		return
	}
	w.Header().Set("ETag", info.ETag)
	w.Header().Set("Last-Modified", info.LastModified.UTC().Format(timeFmt))
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) deleteContainer(w http.ResponseWriter, r *http.Request, req request) {
	err := s.store.DeleteContainer(req.account, req.container)
	if s.mapStoreErr(w, r, err) {
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) getContainerProperties(w http.ResponseWriter, r *http.Request, req request) {
	info, err := s.store.GetContainer(req.account, req.container)
	if s.mapStoreErr(w, r, err) {
		return
	}
	w.Header().Set("ETag", info.ETag)
	w.Header().Set("Last-Modified", info.LastModified.UTC().Format(timeFmt))
	w.Header().Set("x-ms-lease-status", "unlocked")
	w.Header().Set("x-ms-lease-state", "available")
	w.Header().Set("x-ms-has-immutability-policy", "false")
	w.Header().Set("x-ms-has-legal-hold", "false")
	writeMetadata(w.Header(), info.Metadata)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) listContainers(w http.ResponseWriter, r *http.Request, req request, q queryValues) {
	prefix := q.Get("prefix")
	items, err := s.store.ListContainers(req.account, prefix)
	if s.mapStoreErr(w, r, err) {
		return
	}
	body, err := marshalContainers(s.serviceEndpoint(r, req.account), prefix, items)
	if err != nil {
		s.writeError(w, r, azerr.Internal(err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (s *Server) listBlobs(w http.ResponseWriter, r *http.Request, req request, q queryValues) {
	prefix := q.Get("prefix")
	delimiter := q.Get("delimiter")
	blobs, prefixes, err := s.store.ListBlobs(req.account, req.container, prefix, delimiter)
	if s.mapStoreErr(w, r, err) {
		return
	}
	body, err := marshalBlobs(s.serviceEndpoint(r, req.account), req.container, prefix, delimiter, blobs, prefixes)
	if err != nil {
		s.writeError(w, r, azerr.Internal(err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// validContainerName enforces Azure's container naming rules: 3-63 chars, lower
// case letters, digits and single internal hyphens.
func validContainerName(name string) bool {
	if len(name) < 3 || len(name) > 63 {
		return false
	}
	if name[0] == '-' || name[len(name)-1] == '-' {
		return false
	}
	var prevHyphen bool
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			prevHyphen = false
		case c == '-':
			if prevHyphen {
				return false
			}
			prevHyphen = true
		default:
			return false
		}
	}
	return true
}
