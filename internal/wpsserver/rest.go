package wpsserver

import (
	"io"
	"net/http"
	"strings"
)

// serveREST dispatches the REST publish/management API. rest is the path after
// "api/hubs/", i.e. "{hub}/...".
func (s *Server) serveREST(w http.ResponseWriter, r *http.Request, rest string) {
	rest = strings.Trim(rest, "/")
	segments := strings.Split(rest, "/")
	if len(segments) == 0 || segments[0] == "" {
		http.Error(w, "missing hub", http.StatusBadRequest)
		return
	}

	hubName := segments[0]
	h := s.manager.hub(hubName)
	tail := segments[1:]

	switch {
	// /api/hubs/{hub}/:send
	case len(tail) == 1 && tail[0] == ":send":
		s.publish(w, r, func(frame []byte) { h.broadcast(frame) }, "server", "")

	// /api/hubs/{hub}/groups/{group}/:send
	case len(tail) == 3 && tail[0] == "groups" && tail[2] == ":send":
		group := tail[1]
		s.publish(w, r, func(frame []byte) { h.sendGroup(group, frame) }, "group", group)

	// /api/hubs/{hub}/users/{userId}/:send
	case len(tail) == 3 && tail[0] == "users" && tail[2] == ":send":
		user := tail[1]
		s.publish(w, r, func(frame []byte) { h.sendUser(user, frame) }, "server", "")

	// /api/hubs/{hub}/connections/{connId}/:send
	case len(tail) == 3 && tail[0] == "connections" && tail[2] == ":send":
		connID := tail[1]
		s.publish(w, r, func(frame []byte) { h.sendConn(connID, frame) }, "server", "")

	// PUT/DELETE /api/hubs/{hub}/groups/{group}/connections/{connId}
	case len(tail) == 4 && tail[0] == "groups" && tail[2] == "connections":
		group, connID := tail[1], tail[3]
		switch r.Method {
		case http.MethodPut:
			h.join(group, connID)
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			h.leave(group, connID)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}

	// HEAD /api/hubs/{hub}/groups/{group}
	case len(tail) == 2 && tail[0] == "groups":
		s.existsResponse(w, h.groupExists(tail[1]))

	// HEAD /api/hubs/{hub}/connections/{connId}
	// DELETE /api/hubs/{hub}/connections/{connId}
	case len(tail) == 2 && tail[0] == "connections":
		switch r.Method {
		case http.MethodHead:
			s.existsResponse(w, h.connExists(tail[1]))
		case http.MethodDelete:
			h.close(tail[1])
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}

	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// publish reads the request body and delivers a framed data message via deliver.
// from is "server" or "group"; group is set only for group sends.
func (s *Server) publish(w http.ResponseWriter, r *http.Request, deliver func([]byte), from, group string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if i := strings.IndexByte(contentType, ';'); i >= 0 {
		contentType = strings.TrimSpace(contentType[:i])
	}
	dataType := dataTypeFor(contentType)

	frame := mustJSON(dataMessage{
		Type:     "message",
		From:     from,
		Group:    group,
		DataType: dataType,
		Data:     encodeData(dataType, body),
	})
	deliver(frame)

	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) existsResponse(w http.ResponseWriter, exists bool) {
	if exists {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}
