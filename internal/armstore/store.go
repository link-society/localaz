// Package armstore holds the in-memory state of the emulated Azure Resource
// Manager control plane: a single subscription and tenant fixed by
// configuration, plus the resource groups created at runtime. Like the other
// pub/sub services, this state is transient and never persisted.
package armstore

import (
	"fmt"
	"sort"
	"sync"
)

// Store is the ARM state container. Its zero value is not usable; call New.
type Store struct {
	cfg    Config
	mu     sync.Mutex
	groups map[string]ResourceGroup
}

// New constructs a Store with the given configuration and defaults applied.
func New(cfg Config) *Store {
	if cfg.CloudName == "" {
		cfg.CloudName = "localaz"
	}
	if cfg.SubscriptionID == "" {
		cfg.SubscriptionID = "00000000-0000-0000-0000-000000000000"
	}
	if cfg.SubscriptionName == "" {
		cfg.SubscriptionName = "localaz"
	}
	if cfg.TenantID == "" {
		cfg.TenantID = "adfs"
	}
	if cfg.Location == "" {
		cfg.Location = "localaz"
	}
	return &Store{cfg: cfg, groups: make(map[string]ResourceGroup)}
}

// Config returns the store's configuration.
func (s *Store) Config() Config { return s.cfg }

// PutResourceGroup creates or replaces a resource group, returning the stored
// record.
func (s *Store) PutResourceGroup(name, location string, tags map[string]string) ResourceGroup {
	s.mu.Lock()
	defer s.mu.Unlock()
	if location == "" {
		location = s.cfg.Location
	}
	rg := ResourceGroup{
		ID:         fmt.Sprintf("/subscriptions/%s/resourceGroups/%s", s.cfg.SubscriptionID, name),
		Name:       name,
		Type:       "Microsoft.Resources/resourceGroups",
		Location:   location,
		Tags:       tags,
		Properties: groupProperties{ProvisioningState: "Succeeded"},
	}
	s.groups[name] = rg
	return rg
}

// GetResourceGroup returns a resource group by name.
func (s *Store) GetResourceGroup(name string) (ResourceGroup, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rg, ok := s.groups[name]
	return rg, ok
}

// ListResourceGroups returns all resource groups sorted by name.
func (s *Store) ListResourceGroups() []ResourceGroup {
	s.mu.Lock()
	defer s.mu.Unlock()
	names := make([]string, 0, len(s.groups))
	for name := range s.groups {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]ResourceGroup, 0, len(names))
	for _, name := range names {
		out = append(out, s.groups[name])
	}
	return out
}

// DeleteResourceGroup removes a resource group, reporting whether it existed.
func (s *Store) DeleteResourceGroup(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.groups[name]; !ok {
		return false
	}
	delete(s.groups, name)
	return true
}
