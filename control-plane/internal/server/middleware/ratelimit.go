package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter implements a per-IP token bucket rate limiter.
type RateLimiter struct {
	buckets         map[string]*bucket
	mu              sync.Mutex
	rate            float64 // tokens per second
	burst           int     // max tokens
	cleanupInterval time.Duration
	stopCh          chan struct{}
}

type bucket struct {
	tokens   float64
	lastTime time.Time
}

// NewRateLimiter creates a rate limiter with the given tokens/second rate and burst capacity.
// It starts a background goroutine to clean up stale buckets every 5 minutes.
func NewRateLimiter(rate float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		buckets:         make(map[string]*bucket),
		rate:            rate,
		burst:           burst,
		cleanupInterval: 5 * time.Minute,
		stopCh:          make(chan struct{}),
	}
	go rl.cleanup()
	return rl
}

// Middleware returns a gin middleware that enforces the per-IP rate limit.
// Returns 429 Too Many Requests when the bucket is exhausted.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !rl.allow(ip) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":   "rate_limit_exceeded",
				"message": "too many requests, please try again later",
			})
			return
		}
		c.Next()
	}
}

// SSEMiddleware returns a gin middleware that limits the number of concurrent
// SSE (long-lived) connections per IP. This prevents a single client from
// exhausting server resources with persistent connections.
func (rl *RateLimiter) SSEMiddleware(maxConcurrent int) gin.HandlerFunc {
	var mu sync.Mutex
	active := make(map[string]int)

	return func(c *gin.Context) {
		ip := c.ClientIP()

		mu.Lock()
		current := active[ip]
		if current >= maxConcurrent {
			mu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":   "sse_limit_exceeded",
				"message": "too many concurrent SSE connections",
			})
			return
		}
		active[ip]++
		mu.Unlock()

		// Decrement when the connection closes.
		defer func() {
			mu.Lock()
			active[ip]--
			if active[ip] <= 0 {
				delete(active, ip)
			}
			mu.Unlock()
		}()

		c.Next()
	}
}

// Stop terminates the background cleanup goroutine.
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

// allow checks whether the given IP has tokens remaining in its bucket.
func (rl *RateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[ip]
	if !ok {
		rl.buckets[ip] = &bucket{
			tokens:   float64(rl.burst) - 1,
			lastTime: now,
		}
		return true
	}

	// Refill tokens based on elapsed time.
	elapsed := now.Sub(b.lastTime).Seconds()
	b.tokens += elapsed * rl.rate
	if b.tokens > float64(rl.burst) {
		b.tokens = float64(rl.burst)
	}
	b.lastTime = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// cleanup periodically removes stale buckets (those with full tokens that
// haven't been touched in over cleanupInterval).
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for ip, b := range rl.buckets {
				if now.Sub(b.lastTime) > rl.cleanupInterval {
					delete(rl.buckets, ip)
				}
			}
			rl.mu.Unlock()
		case <-rl.stopCh:
			return
		}
	}
}
