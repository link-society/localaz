// Package armstore holds the in-memory state of the emulated Azure Resource
// Manager control plane: a single subscription and tenant fixed by
// configuration, plus the resource groups created at runtime. Like the other
// pub/sub services, this state is transient and never persisted.
package armstore

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Store is the ARM state container. Its zero value is not usable; call New.
type Store struct {
	cfg       Config
	mu        sync.Mutex
	groups    map[string]ResourceGroup
	resources map[string]map[string]any
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
	return &Store{
		cfg:       cfg,
		groups:    make(map[string]ResourceGroup),
		resources: make(map[string]map[string]any),
	}
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

// maxResources caps the number of distinct generic provider resources the
// store will hold. The emulator keeps everything in memory, so without a bound
// a client looping PUTs with distinct IDs could exhaust memory. Replacing an
// already-stored ID is always allowed; only new IDs are refused once the cap is
// reached.
const maxResources = 10000

// PutResource stores a generic provider resource keyed by its (case-insensitive)
// ARM resource ID, replacing any existing record. It reports false (and stores
// nothing) when storing a new ID would exceed maxResources.
func (s *Store) PutResource(id string, body map[string]any) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := strings.ToLower(id)
	if _, exists := s.resources[key]; !exists && len(s.resources) >= maxResources {
		return false
	}
	s.resources[key] = body
	return true
}

// GetResource returns a stored provider resource by ID.
func (s *Store) GetResource(id string) (map[string]any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.resources[strings.ToLower(id)]
	return r, ok
}

// ListResources returns the stored resources whose ID begins with idPrefix and
// whose type matches resourceType (both compared case-insensitively), sorted by
// ID for stable output.
func (s *Store) ListResources(idPrefix, resourceType string) []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	prefix := strings.ToLower(idPrefix)
	wantType := strings.ToLower(resourceType)
	keys := make([]string, 0, len(s.resources))
	for key, r := range s.resources {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		if t, _ := r["type"].(string); strings.ToLower(t) != wantType {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		out = append(out, s.resources[key])
	}
	return out
}

// DeleteResource removes a provider resource, reporting whether it existed.
func (s *Store) DeleteResource(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := strings.ToLower(id)
	if _, ok := s.resources[key]; !ok {
		return false
	}
	delete(s.resources, key)
	return true
}
