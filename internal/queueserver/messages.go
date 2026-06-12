package queueserver

import (
	"encoding/xml"
	"io"
	"net/http"
	"strconv"
	"time"

	"localaz.dev/internal/azerr"
)

// defaultVisibilityTimeout is Azure's default for Get Messages.
const defaultVisibilityTimeout = 30 * time.Second

// putMessageBody is the request payload for Put Message and Update Message.
type putMessageBody struct {
	XMLName     xml.Name `xml:"QueueMessage"`
	MessageText string   `xml:"MessageText"`
}

func (s *Server) putMessage(w http.ResponseWriter, r *http.Request, req request, q url) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		s.writeError(w, r, azerr.Internal(err.Error()))
		return
	}
	var msg putMessageBody
	if len(body) > 0 {
		if err := xml.Unmarshal(body, &msg); err != nil {
			s.writeError(w, r, azerr.New(http.StatusBadRequest, azerr.CodeInvalidInput, "Malformed message body."))
			return
		}
	}

	visibility := parseSeconds(q.Get("visibilitytimeout"), 0)
	ttl := parseTTL(q.Get("messagettl"))

	m, err := s.store.Enqueue(req.account, req.queue, msg.MessageText, visibility, ttl)
	if s.mapStoreErr(w, r, err) {
		return
	}
	out, err := marshalMessages([]messageView{enqueuedView(m)})
	if err != nil {
		s.writeError(w, r, azerr.Internal(err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(out)
}

func (s *Server) getMessages(w http.ResponseWriter, r *http.Request, req request, q url) {
	num := parseInt(q.Get("numofmessages"), 1)
	if num < 1 {
		num = 1
	}

	var (
		views []messageView
		err   error
	)
	if q.Get("peekonly") == "true" {
		msgs, e := s.store.Peek(req.account, req.queue, num)
		err = e
		for _, m := range msgs {
			views = append(views, peekView(m))
		}
	} else {
		visibility := parseSeconds(q.Get("visibilitytimeout"), defaultVisibilityTimeout)
		msgs, e := s.store.Dequeue(req.account, req.queue, num, visibility)
		err = e
		for _, m := range msgs {
			views = append(views, dequeuedView(m))
		}
	}
	if s.mapStoreErr(w, r, err) {
		return
	}
	out, err := marshalMessages(views)
	if err != nil {
		s.writeError(w, r, azerr.Internal(err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out)
}

func (s *Server) deleteMessage(w http.ResponseWriter, r *http.Request, req request, q url) {
	if s.mapStoreErr(w, r, s.store.DeleteMessage(req.account, req.queue, req.messageID, q.Get("popreceipt"))) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) updateMessage(w http.ResponseWriter, r *http.Request, req request, q url) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		s.writeError(w, r, azerr.Internal(err.Error()))
		return
	}
	var msg putMessageBody
	if len(body) > 0 {
		if err := xml.Unmarshal(body, &msg); err != nil {
			s.writeError(w, r, azerr.New(http.StatusBadRequest, azerr.CodeInvalidInput, "Malformed message body."))
			return
		}
	}

	visibility := parseSeconds(q.Get("visibilitytimeout"), 0)
	popReceipt, nextVisible, err := s.store.UpdateMessage(
		req.account, req.queue, req.messageID, q.Get("popreceipt"), msg.MessageText, visibility)
	if s.mapStoreErr(w, r, err) {
		return
	}
	w.Header().Set("x-ms-popreceipt", popReceipt)
	w.Header().Set("x-ms-time-next-visible", nextVisible.UTC().Format(timeFmt))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) clearMessages(w http.ResponseWriter, r *http.Request, req request) {
	if s.mapStoreErr(w, r, s.store.ClearMessages(req.account, req.queue)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseInt(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return fallback
}

func parseSeconds(s string, fallback time.Duration) time.Duration {
	if s == "" {
		return fallback
	}
	if v, err := strconv.Atoi(s); err == nil {
		return time.Duration(v) * time.Second
	}
	return fallback
}

// parseTTL maps the messagettl query value to a duration: empty/0 means the
// 7-day default (signalled as 0), -1 means infinite.
func parseTTL(s string) time.Duration {
	if s == "" {
		return 0
	}
	v, err := strconv.Atoi(s)
	if err != nil || v == 0 {
		return 0
	}
	if v < 0 {
		return -1
	}
	return time.Duration(v) * time.Second
}
