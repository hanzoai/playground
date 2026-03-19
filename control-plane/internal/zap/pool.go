package zap

import (
	"context"
	"fmt"
	"sync"
)

// Pool manages a set of sidecar processes keyed by bot ID.
type Pool struct {
	mu       sync.RWMutex
	sidecars map[string]*Sidecar
}

// NewPool creates an empty sidecar pool.
func NewPool() *Pool {
	return &Pool{
		sidecars: make(map[string]*Sidecar),
	}
}

// Get returns the sidecar for the given bot ID, if one exists.
func (p *Pool) Get(botID string) (*Sidecar, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	s, ok := p.sidecars[botID]
	return s, ok
}

// Spawn creates a new sidecar for the given bot ID and adds it to the pool.
// Returns an error if a sidecar for this bot already exists.
func (p *Pool) Spawn(ctx context.Context, botID string, opts SidecarOpts) (*Sidecar, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.sidecars[botID]; exists {
		return nil, fmt.Errorf("zap: sidecar already exists for bot %s", botID)
	}

	opts.BotID = botID
	s, err := SpawnSidecar(ctx, opts)
	if err != nil {
		return nil, err
	}

	p.sidecars[botID] = s
	return s, nil
}

// Remove stops and removes the sidecar for the given bot ID.
func (p *Pool) Remove(botID string) error {
	p.mu.Lock()
	s, ok := p.sidecars[botID]
	if ok {
		delete(p.sidecars, botID)
	}
	p.mu.Unlock()

	if !ok {
		return fmt.Errorf("zap: no sidecar for bot %s", botID)
	}

	return s.Stop(context.Background())
}

// ForSpace returns all sidecars associated with the given space ID.
func (p *Pool) ForSpace(spaceID string) []*Sidecar {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var result []*Sidecar
	for _, s := range p.sidecars {
		if s.opts.SpaceID == spaceID {
			result = append(result, s)
		}
	}
	return result
}

// StopAll shuts down all sidecars in the pool.
func (p *Pool) StopAll(ctx context.Context) {
	p.mu.Lock()
	all := make(map[string]*Sidecar, len(p.sidecars))
	for k, v := range p.sidecars {
		all[k] = v
	}
	p.sidecars = make(map[string]*Sidecar)
	p.mu.Unlock()

	for _, s := range all {
		_ = s.Stop(ctx)
	}
}

// Len returns the number of active sidecars.
func (p *Pool) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.sidecars)
}
