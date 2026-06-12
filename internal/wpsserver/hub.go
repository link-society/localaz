// Package wpsserver implements the Azure Web PubSub data plane: the REST
// publish API consumed by the official azwebpubsub SDK plus the client
// WebSocket endpoint speaking the json.webpubsub.azure.v1 subprotocol. State
// lives only in memory (live connections are not persisted), so the hub itself
// is the source of truth rather than a separate store package.
package wpsserver

import "sync"

// conn is a single live client connection on a hub.
type conn interface {
	// id returns the connection id assigned at handshake time.
	id() string
	// userID returns the user id carried in the access token, if any.
	userID() string
	// send queues a fully framed protocol message for delivery to the client.
	send(frame []byte)
}

// hub holds the connections, group memberships and user index for one hub.
type hub struct {
	mu     sync.RWMutex
	conns  map[string]conn
	groups map[string]map[string]struct{}
}

func newHub() *hub {
	return &hub{
		conns:  make(map[string]conn),
		groups: make(map[string]map[string]struct{}),
	}
}

func (h *hub) add(c conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conns[c.id()] = c
}

func (h *hub) remove(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.conns, id)
	for _, members := range h.groups {
		delete(members, id)
	}
}

func (h *hub) join(group, id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	members, ok := h.groups[group]
	if !ok {
		members = make(map[string]struct{})
		h.groups[group] = members
	}
	members[id] = struct{}{}
}

func (h *hub) leave(group, id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if members, ok := h.groups[group]; ok {
		delete(members, id)
	}
}

func (h *hub) groupExists(group string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.groups[group]) > 0
}

func (h *hub) connExists(id string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.conns[id]
	return ok
}

// broadcast delivers frame to every connection on the hub.
func (h *hub) broadcast(frame []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, c := range h.conns {
		c.send(frame)
	}
}

// sendGroup delivers frame to every connection that has joined group.
func (h *hub) sendGroup(group string, frame []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for id := range h.groups[group] {
		if c, ok := h.conns[id]; ok {
			c.send(frame)
		}
	}
}

// sendUser delivers frame to every connection authenticated as userID.
func (h *hub) sendUser(userID string, frame []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, c := range h.conns {
		if c.userID() == userID {
			c.send(frame)
		}
	}
}

// sendConn delivers frame to a single connection if it is still present.
func (h *hub) sendConn(id string, frame []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if c, ok := h.conns[id]; ok {
		c.send(frame)
	}
}

func (h *hub) close(id string) {
	h.mu.RLock()
	c, ok := h.conns[id]
	h.mu.RUnlock()
	if ok {
		if closer, isCloser := c.(interface{ closeNow() }); isCloser {
			closer.closeNow()
		}
	}
}

// manager owns the set of hubs, creating them on demand.
type manager struct {
	mu   sync.Mutex
	hubs map[string]*hub
}

func newManager() *manager {
	return &manager{hubs: make(map[string]*hub)}
}

func (m *manager) hub(name string) *hub {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, ok := m.hubs[name]
	if !ok {
		h = newHub()
		m.hubs[name] = h
	}
	return h
}
