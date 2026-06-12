package wpsserver

import (
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

// Server exposes the Web PubSub data plane: the client WebSocket endpoint under
// /client/hubs/{hub} and the REST publish API under /api/hubs/{hub}.
type Server struct {
	manager  *manager
	upgrader websocket.Upgrader
}

// New constructs a Web PubSub Server with an empty hub set.
func New() *Server {
	return &Server{
		manager: newManager(),
		upgrader: websocket.Upgrader{
			Subprotocols:    []string{"json.webpubsub.azure.v1"},
			CheckOrigin:     func(*http.Request) bool { return true },
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
		},
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	switch {
	case strings.HasPrefix(path, "client/hubs/"):
		s.serveClient(w, r, strings.TrimPrefix(path, "client/hubs/"))
	case strings.HasPrefix(path, "api/hubs/"):
		s.serveREST(w, r, strings.TrimPrefix(path, "api/hubs/"))
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// serveClient upgrades a client connection and runs the protocol loop. rest is
// the path after "client/hubs/", i.e. "{hub}".
func (s *Server) serveClient(w http.ResponseWriter, r *http.Request, rest string) {
	hubName := strings.Trim(rest, "/")
	if hubName == "" {
		http.Error(w, "missing hub", http.StatusBadRequest)
		return
	}

	claims := parseToken(r.URL.Query().Get("access_token"))

	socket, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	c := &wsConn{
		connID: newConnID(),
		user:   claims.Subject,
		socket: socket,
		hub:    s.manager.hub(hubName),
	}
	c.run(claims.Groups)
}
