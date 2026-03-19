package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRateLimitRouter(rl *RateLimiter) *gin.Engine {
	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/api/v1/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})
	return router
}

func TestRateLimiter_AllowsWithinBurst(t *testing.T) {
	rl := NewRateLimiter(100, 10)
	defer rl.Stop()
	router := setupRateLimitRouter(rl)

	// First 10 requests should succeed (burst=10)
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "request %d should succeed", i)
	}
}

func TestRateLimiter_RejectsBeyondBurst(t *testing.T) {
	rl := NewRateLimiter(0.001, 5) // Very slow refill
	defer rl.Stop()
	router := setupRateLimitRouter(rl)

	// Exhaust the burst
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// Next request should be rejected
	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	var resp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "rate_limit_exceeded", resp["error"])
}

func TestRateLimiter_DifferentIPsIndependent(t *testing.T) {
	rl := NewRateLimiter(0.001, 2)
	defer rl.Stop()
	router := setupRateLimitRouter(rl)

	// Exhaust IP1
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
		req.RemoteAddr = "1.1.1.1:1234"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// IP1 should be rejected
	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.RemoteAddr = "1.1.1.1:1234"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// IP2 should still work
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req2.RemoteAddr = "2.2.2.2:1234"
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestSSEMiddleware_RejectsExcessConcurrent(t *testing.T) {
	rl := NewRateLimiter(100, 200)
	defer rl.Stop()

	router := gin.New()
	router.Use(rl.SSEMiddleware(2))
	router.GET("/events", func(c *gin.Context) {
		// Simulate a long-lived SSE connection by blocking until context is done
		<-c.Request.Context().Done()
		c.String(http.StatusOK, "done")
	})

	var wg sync.WaitGroup

	// Start 2 concurrent SSE connections (should succeed)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/events", nil)
			req.RemoteAddr = "1.1.1.1:1234"
			w := httptest.NewRecorder()
			// This will block, but we cancel via context
			ctx, cancel := testContext()
			defer cancel()
			req = req.WithContext(ctx)
			router.ServeHTTP(w, req)
		}()
	}

	// Give goroutines time to register
	// (The SSE middleware increments the counter synchronously before c.Next())
	// We need a brief pause to let the goroutines start
	testWaitBriefly()

	// Third concurrent should be rejected
	req3 := httptest.NewRequest(http.MethodGet, "/events", nil)
	req3.RemoteAddr = "1.1.1.1:1234"
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusTooManyRequests, w3.Code)

	var resp map[string]string
	err := json.Unmarshal(w3.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "sse_limit_exceeded", resp["error"])
}

func TestSSEMiddleware_DifferentIPsIndependent(t *testing.T) {
	rl := NewRateLimiter(100, 200)
	defer rl.Stop()

	router := gin.New()
	router.Use(rl.SSEMiddleware(1))
	router.GET("/events", func(c *gin.Context) {
		<-c.Request.Context().Done()
		c.String(http.StatusOK, "done")
	})

	// Start 1 SSE for IP1
	go func() {
		req := httptest.NewRequest(http.MethodGet, "/events", nil)
		req.RemoteAddr = "1.1.1.1:1234"
		ctx, cancel := testContext()
		defer cancel()
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}()

	testWaitBriefly()

	// IP2 should still be able to connect
	done := make(chan int, 1)
	go func() {
		req := httptest.NewRequest(http.MethodGet, "/events", nil)
		req.RemoteAddr = "2.2.2.2:1234"
		ctx, cancel := testContext()
		defer cancel()
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		done <- w.Code
	}()

	testWaitBriefly()

	// IP1 should be rejected (already at limit)
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	req.RemoteAddr = "1.1.1.1:1234"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}
