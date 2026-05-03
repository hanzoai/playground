package gitops

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
)

// Manager manages git repos for all spaces.
type Manager struct {
	basePath string           // e.g., ~/.hanzo/playground/data/spaces/
	repos    map[string]*Repo // spaceID -> repo
	mu       sync.RWMutex
}

// NewManager creates a Manager that stores space repos under basePath.
func NewManager(basePath string) *Manager {
	return &Manager{
		basePath: basePath,
		repos:    make(map[string]*Repo),
	}
}

// spacePath returns the filesystem path for a space's repo.
func (m *Manager) spacePath(spaceID string) string {
	return filepath.Join(m.basePath, spaceID)
}

// GetOrInit returns the repo for a space, initializing it if needed.
func (m *Manager) GetOrInit(spaceID string) (*Repo, error) {
	m.mu.RLock()
	if r, ok := m.repos[spaceID]; ok {
		m.mu.RUnlock()
		return r, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock.
	if r, ok := m.repos[spaceID]; ok {
		return r, nil
	}

	r, err := InitOrOpen(m.spacePath(spaceID), spaceID)
	if err != nil {
		return nil, fmt.Errorf("get or init repo for space %s: %w", spaceID, err)
	}

	m.repos[spaceID] = r
	return r, nil
}

// CloneForSpace clones a remote repo into a space's workspace.
func (m *Manager) CloneForSpace(ctx context.Context, spaceID, url string, opts CloneOpts) (*Repo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := m.spacePath(spaceID)
	r, err := Clone(ctx, url, path, spaceID, opts)
	if err != nil {
		return nil, fmt.Errorf("clone for space %s: %w", spaceID, err)
	}

	m.repos[spaceID] = r
	return r, nil
}

// Remove removes a space's repo from management (does not delete files).
func (m *Manager) Remove(spaceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.repos, spaceID)
}

// ListSpaces returns all managed space IDs.
func (m *Manager) ListSpaces() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.repos))
	for id := range m.repos {
		ids = append(ids, id)
	}
	return ids
}
