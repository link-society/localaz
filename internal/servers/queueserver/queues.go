package queueserver

import (
	"net/http"
	"strconv"
	"strings"

	"localaz.dev/internal/utils/azerr"
)

// readMetadata extracts x-ms-meta-* headers into a metadata map, lower-casing
// keys to preserve Azure's case-insensitive semantics.
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

func (s *Server) createQueue(w http.ResponseWriter, r *http.Request, req request) {
	if !validQueueName(req.queue) {
		s.writeError(w, r, azerr.InvalidResourceName())
		return
	}
	created, err := s.store.CreateQueue(req.account, req.queue, readMetadata(r.Header))
	if s.mapStoreErr(w, r, err) {
		return
	}
	if created {
		w.WriteHeader(http.StatusCreated)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteQueue(w http.ResponseWriter, r *http.Request, req request) {
	if s.mapStoreErr(w, r, s.store.DeleteQueue(req.account, req.queue)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getMetadata(w http.ResponseWriter, r *http.Request, req request) {
	info, err := s.store.GetMetadata(req.account, req.queue)
	if s.mapStoreErr(w, r, err) {
		return
	}
	w.Header().Set("x-ms-approximate-messages-count", strconv.Itoa(info.ApproximateCount))
	writeMetadata(w.Header(), info.Metadata)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) setMetadata(w http.ResponseWriter, r *http.Request, req request) {
	if s.mapStoreErr(w, r, s.store.SetMetadata(req.account, req.queue, readMetadata(r.Header))) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listQueues(w http.ResponseWriter, r *http.Request, req request, q url) {
	prefix := q.Get("prefix")
	items := s.store.ListQueues(req.account, prefix)
	body, err := marshalQueues(s.serviceEndpoint(r, req.account), prefix, items)
	if err != nil {
		s.writeError(w, r, azerr.Internal(err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// validQueueName enforces Azure's queue naming rules: 3-63 chars, lower-case
// letters, digits and single internal hyphens.
func validQueueName(name string) bool {
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
