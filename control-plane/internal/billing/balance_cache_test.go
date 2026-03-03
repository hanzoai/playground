package billing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBalanceCache_SetAndGet(t *testing.T) {
	cache := newBalanceCache(1 * time.Minute)

	result := &BalanceResult{User: "hanzo/z", Available: 500, Currency: "usd"}
	cache.set("hanzo/z", result)

	got, ok := cache.get("hanzo/z")
	require.True(t, ok)
	assert.Equal(t, float64(500), got.Available)
	assert.Equal(t, "hanzo/z", got.User)
}

func TestBalanceCache_Miss(t *testing.T) {
	cache := newBalanceCache(1 * time.Minute)

	_, ok := cache.get("nobody")
	assert.False(t, ok)
}

func TestBalanceCache_Expiry(t *testing.T) {
	cache := newBalanceCache(10 * time.Millisecond)

	cache.set("hanzo/z", &BalanceResult{Available: 100})
	time.Sleep(20 * time.Millisecond)

	_, ok := cache.get("hanzo/z")
	assert.False(t, ok, "expired entry should not be returned")
}

func TestBalanceCache_Invalidate(t *testing.T) {
	cache := newBalanceCache(1 * time.Minute)

	cache.set("hanzo/z", &BalanceResult{Available: 100})
	cache.invalidate("hanzo/z")

	_, ok := cache.get("hanzo/z")
	assert.False(t, ok, "invalidated entry should not be returned")
}

func TestBalanceCache_Prune(t *testing.T) {
	cache := newBalanceCache(10 * time.Millisecond)

	cache.set("user1", &BalanceResult{Available: 100})
	cache.set("user2", &BalanceResult{Available: 200})

	time.Sleep(20 * time.Millisecond)

	// Add a fresh entry that should survive prune.
	cache.set("user3", &BalanceResult{Available: 300})
	cache.prune()

	assert.Equal(t, 1, cache.len(), "only non-expired entries should survive prune")
	got, ok := cache.get("user3")
	require.True(t, ok)
	assert.Equal(t, float64(300), got.Available)
}

func TestBalanceCache_OverwriteExistingKey(t *testing.T) {
	cache := newBalanceCache(1 * time.Minute)

	cache.set("hanzo/z", &BalanceResult{Available: 100})
	cache.set("hanzo/z", &BalanceResult{Available: 200})

	got, ok := cache.get("hanzo/z")
	require.True(t, ok)
	assert.Equal(t, float64(200), got.Available, "overwrite should replace old value")
}

func TestBalanceCache_DefaultTTL(t *testing.T) {
	cache := newBalanceCache(0) // should default to 60s
	assert.Equal(t, defaultBalanceCacheTTL, cache.ttl)
}

func TestBalanceCache_ConcurrentAccess(t *testing.T) {
	cache := newBalanceCache(1 * time.Minute)

	done := make(chan struct{})
	for i := 0; i < 100; i++ {
		go func(n int) {
			key := "user"
			cache.set(key, &BalanceResult{Available: float64(n)})
			cache.get(key)
			cache.invalidate(key)
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 100; i++ {
		<-done
	}
}
