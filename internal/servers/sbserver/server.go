// Package sbserver implements the subset of the AMQP 1.0 protocol that the
// official azservicebus SDK uses, so the emulator can accept queue/topic sends
// and deliver messages to receivers over plain TCP.
package sbserver

import (
	"net"

	"localaz.dev/internal/stores/sbstore"
)

// Server accepts AMQP connections and serves them against a broker.
type Server struct {
	broker *sbstore.Broker
}

// New constructs a Server backed by broker.
func New(broker *sbstore.Broker) *Server {
	return &Server{broker: broker}
}

// Serve accepts connections on l until it is closed, handling each in its own
// goroutine. It returns the error from Accept when the listener is closed.
func (s *Server) Serve(l net.Listener) error {
	for {
		netConn, err := l.Accept()
		if err != nil {
			return err
		}
		go newConn(netConn, s.broker).serve()
	}
}
