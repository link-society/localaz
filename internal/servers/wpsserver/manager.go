package wpsserver

import "sync"

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
