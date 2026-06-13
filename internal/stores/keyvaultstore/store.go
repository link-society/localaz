// Package keyvaultstore is an in-memory implementation of the Azure Key Vault
// secrets data plane, with write-through JSON persistence so a mounted data
// volume survives restarts (matching the storage services' promise).
//
// The on-disk layout is a single snapshot file:
//
//	<root>/keyvault/secrets.json
//
// Secrets are namespaced by vault host so different vault URLs pointed at the
// same emulator stay isolated. The HTTP layer (internal/servers/kvserver)
// depends only on this concrete store.
package keyvaultstore

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"localaz.dev/internal/utils/atomicfile"
)

// ErrSecretNotFound is returned when a secret (or version) does not exist.
var ErrSecretNotFound = errors.New("keyvaultstore: secret not found")

// Store holds every vault's secrets. It is safe for concurrent use.
type Store struct {
	mu         sync.Mutex
	path       string
	vaults     map[string]*vault
	now        func() time.Time
	newVersion func() string
}

// New creates a Store persisted under root, loading any prior snapshot.
func New(root string) (*Store, error) {
	dir := filepath.Join(root, "keyvault")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("keyvaultstore: create dir: %w", err)
	}
	s := &Store{
		path:       filepath.Join(dir, "secrets.json"),
		vaults:     map[string]*vault{},
		now:        func() time.Time { return time.Now().UTC() },
		newVersion: randomVersion,
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("keyvaultstore: read snapshot: %w", err)
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, &s.vaults)
}

// persistLocked writes the current state crash-safely (temp file + fsync +
// rename + parent-dir fsync via atomicfile). The caller must hold s.mu. Errors
// are best-effort: durability, not error propagation, is the goal here.
func (s *Store) persistLocked() {
	data, err := json.Marshal(s.vaults)
	if err != nil {
		return
	}
	_ = atomicfile.Write(s.path, data, 0o644)
}

func (s *Store) getVaultLocked(host string) *vault {
	v, ok := s.vaults[host]
	if !ok {
		v = &vault{Secrets: map[string]*secretEntry{}}
		s.vaults[host] = v
	}
	return v
}

func (s *Store) lookupLocked(host, name string) (*secretEntry, error) {
	v, ok := s.vaults[host]
	if !ok {
		return nil, ErrSecretNotFound
	}
	e, ok := v.Secrets[name]
	if !ok || len(e.Versions) == 0 {
		return nil, ErrSecretNotFound
	}
	return e, nil
}

// SetSecret stores a new version of name and makes it the current version.
func (s *Store) SetSecret(host, name, value, contentType string, tags map[string]string, attrs Attributes) Secret {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.getVaultLocked(host).Secrets[name]
	if !ok {
		e = &secretEntry{Versions: map[string]*Secret{}}
		s.getVaultLocked(host).Secrets[name] = e
	}

	now := s.now()
	sec := &Secret{
		Name:        name,
		Version:     s.newVersion(),
		Value:       value,
		ContentType: contentType,
		Tags:        cloneTags(tags),
		Enabled:     true,
		NotBefore:   attrs.NotBefore,
		Expires:     attrs.Expires,
		Created:     now,
		Updated:     now,
	}
	if attrs.Enabled != nil {
		sec.Enabled = *attrs.Enabled
	}
	e.Versions[sec.Version] = sec
	e.Order = append(e.Order, sec.Version)
	e.Current = sec.Version
	s.persistLocked()
	return *sec
}

// GetSecret returns the named secret at version, or the current version when
// version is empty.
func (s *Store) GetSecret(host, name, version string) (Secret, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, err := s.lookupLocked(host, name)
	if err != nil {
		return Secret{}, err
	}
	if version == "" {
		version = e.Current
	}
	sec, ok := e.Versions[version]
	if !ok {
		return Secret{}, ErrSecretNotFound
	}
	return *sec, nil
}

// UpdateSecret mutates the attributes, content type and tags of an existing
// version (or the current version when version is empty). The value is
// immutable, matching Key Vault.
func (s *Store) UpdateSecret(host, name, version string, contentType *string, tags map[string]string, attrs Attributes) (Secret, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, err := s.lookupLocked(host, name)
	if err != nil {
		return Secret{}, err
	}
	if version == "" {
		version = e.Current
	}
	sec, ok := e.Versions[version]
	if !ok {
		return Secret{}, ErrSecretNotFound
	}
	if contentType != nil {
		sec.ContentType = *contentType
	}
	if tags != nil {
		sec.Tags = cloneTags(tags)
	}
	if attrs.Enabled != nil {
		sec.Enabled = *attrs.Enabled
	}
	if attrs.NotBefore != nil {
		sec.NotBefore = attrs.NotBefore
	}
	if attrs.Expires != nil {
		sec.Expires = attrs.Expires
	}
	sec.Updated = s.now()
	s.persistLocked()
	return *sec, nil
}

// DeleteSecret removes a secret and all its versions, returning the version
// that was current at deletion time.
func (s *Store) DeleteSecret(host, name string) (Secret, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, err := s.lookupLocked(host, name)
	if err != nil {
		return Secret{}, err
	}
	latest := *e.Versions[e.Current]
	delete(s.vaults[host].Secrets, name)
	s.persistLocked()
	return latest, nil
}

// ListSecrets returns the current version of every secret in the vault, sorted
// by name.
func (s *Store) ListSecrets(host string) []Secret {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.vaults[host]
	if !ok {
		return nil
	}
	names := make([]string, 0, len(v.Secrets))
	for n := range v.Secrets {
		names = append(names, n)
	}
	sort.Strings(names)

	out := make([]Secret, 0, len(names))
	for _, n := range names {
		e := v.Secrets[n]
		out = append(out, *e.Versions[e.Current])
	}
	return out
}

// ListVersions returns every stored version of name, oldest first.
func (s *Store) ListVersions(host, name string) ([]Secret, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, err := s.lookupLocked(host, name)
	if err != nil {
		return nil, err
	}
	out := make([]Secret, 0, len(e.Order))
	for _, ver := range e.Order {
		out = append(out, *e.Versions[ver])
	}
	return out, nil
}

func cloneTags(tags map[string]string) map[string]string {
	if len(tags) == 0 {
		return nil
	}
	out := make(map[string]string, len(tags))
	for k, v := range tags {
		out[k] = v
	}
	return out
}

// randomVersion returns a 32-character lowercase hex string, matching the shape
// of an Azure Key Vault secret version identifier.
func randomVersion() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
