package billing

import (
	"sync"
	"time"
)

// balanceCache provides a simple TTL cache for Commerce balance lookups.
// This avoids hammering Commerce on every LLM request for the same user.
type balanceCache struct {
	mu      sync.RWMutex
	entries map[string]*balanceCacheEntry
	ttl     time.Duration
}

type balanceCacheEntry struct {
	result    *BalanceResult
	expiresAt time.Time
}

const defaultBalanceCacheTTL = 60 * time.Second

func newBalanceCache(ttl time.Duration) *balanceCache {
	if ttl <= 0 {
		ttl = defaultBalanceCacheTTL
	}
	return &balanceCache{
		entries: make(map[string]*balanceCacheEntry),
		ttl:     ttl,
	}
}

func (bc *balanceCache) get(key string) (*BalanceResult, bool) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	entry, ok := bc.entries[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.result, true
}

func (bc *balanceCache) set(key string, result *BalanceResult) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.entries[key] = &balanceCacheEntry{
		result:    result,
		expiresAt: time.Now().Add(bc.ttl),
	}
}

// invalidate removes a specific key from the cache. Called after usage
// recording so the next billing check fetches a fresh balance.
func (bc *balanceCache) invalidate(key string) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	delete(bc.entries, key)
}

// prune removes all expired entries. Safe to call periodically.
func (bc *balanceCache) prune() {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	now := time.Now()
	for k, entry := range bc.entries {
		if now.After(entry.expiresAt) {
			delete(bc.entries, k)
		}
	}
}

// len returns the number of entries (for testing).
func (bc *balanceCache) len() int {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return len(bc.entries)
}
