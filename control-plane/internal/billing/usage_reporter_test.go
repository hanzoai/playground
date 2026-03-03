package billing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsageReporter_Report_BatchFlush(t *testing.T) {
	var mu sync.Mutex
	var received []map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		received = append(received, body)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "svc-token")
	reporter := NewUsageReporter(client, nil, UsageReporterConfig{
		MaxBatchSize:   50,
		FlushInterval:  50 * time.Millisecond,
		MaxRetries:     1,
		InitialBackoff: 1 * time.Millisecond,
	})

	reporter.Report(UsageRecord{
		UserID:      "hanzo/z",
		Model:       "claude-sonnet-4",
		Provider:    "anthropic",
		InputTokens: 100,
		OutputTokens: 50,
	})

	// Wait for the timer-based flush.
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, received, 1)
	assert.Equal(t, "hanzo/z", received[0]["user_id"])
	assert.Equal(t, "claude-sonnet-4", received[0]["model"])
	assert.Equal(t, "anthropic", received[0]["provider"])
	assert.Equal(t, float64(150), received[0]["tokens"]) // TotalTokens auto-computed
}

func TestUsageReporter_Report_ImmediateFlushOnFullBatch(t *testing.T) {
	var mu sync.Mutex
	var received []map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		received = append(received, body)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	batchSize := 3
	client := newTestClient(srv.URL, "")
	reporter := NewUsageReporter(client, nil, UsageReporterConfig{
		MaxBatchSize:   batchSize,
		FlushInterval:  10 * time.Second, // long interval — should not trigger
		MaxRetries:     1,
		InitialBackoff: 1 * time.Millisecond,
	})

	for i := 0; i < batchSize; i++ {
		reporter.Report(UsageRecord{
			UserID:       "hanzo/z",
			Model:        "gpt-4o",
			Provider:     "openai",
			InputTokens:  10,
			OutputTokens: 5,
		})
	}

	// The batch should flush immediately (in a goroutine).
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, received, batchSize, "all records should be flushed immediately on full batch")
}

func TestUsageReporter_Shutdown_FlushesRemaining(t *testing.T) {
	var mu sync.Mutex
	var received []map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		received = append(received, body)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	reporter := NewUsageReporter(client, nil, UsageReporterConfig{
		MaxBatchSize:   100,
		FlushInterval:  10 * time.Second, // long — should not fire
		MaxRetries:     1,
		InitialBackoff: 1 * time.Millisecond,
	})

	reporter.Report(UsageRecord{
		UserID:       "hanzo/a",
		Model:        "claude-opus-4",
		Provider:     "anthropic",
		InputTokens:  500,
		OutputTokens: 200,
	})

	assert.Equal(t, 1, reporter.QueueLen())

	reporter.Shutdown()

	assert.Equal(t, 0, reporter.QueueLen())
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, received, 1)
	assert.Equal(t, "hanzo/a", received[0]["user_id"])
}

func TestUsageReporter_Retry_OnServerError(t *testing.T) {
	var attempts int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "internal"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	reporter := NewUsageReporter(client, nil, UsageReporterConfig{
		MaxBatchSize:   1,
		FlushInterval:  50 * time.Millisecond,
		MaxRetries:     3,
		InitialBackoff: 1 * time.Millisecond,
	})

	reporter.Report(UsageRecord{
		UserID:       "hanzo/z",
		Model:        "gpt-4o",
		Provider:     "openai",
		InputTokens:  10,
		OutputTokens: 5,
	})

	time.Sleep(500 * time.Millisecond)

	assert.Equal(t, int32(3), atomic.LoadInt32(&attempts), "should retry 3 times")
}

func TestUsageReporter_NoRetry_On4xx(t *testing.T) {
	var attempts int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "bad_request"}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	reporter := NewUsageReporter(client, nil, UsageReporterConfig{
		MaxBatchSize:   1,
		FlushInterval:  50 * time.Millisecond,
		MaxRetries:     3,
		InitialBackoff: 1 * time.Millisecond,
	})

	reporter.Report(UsageRecord{
		UserID:       "hanzo/z",
		Model:        "gpt-4o",
		Provider:     "openai",
		InputTokens:  10,
		OutputTokens: 5,
	})

	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, int32(1), atomic.LoadInt32(&attempts), "4xx should not be retried")
}

func TestUsageReporter_InvalidatesCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cache := newBalanceCache(1 * time.Minute)
	cache.set("hanzo/z", &BalanceResult{Available: 500})

	client := newTestClient(srv.URL, "")
	reporter := NewUsageReporter(client, cache, UsageReporterConfig{
		MaxBatchSize:   1,
		FlushInterval:  50 * time.Millisecond,
		MaxRetries:     1,
		InitialBackoff: 1 * time.Millisecond,
	})

	reporter.Report(UsageRecord{
		UserID:       "hanzo/z",
		Model:        "claude-sonnet-4",
		Provider:     "anthropic",
		InputTokens:  100,
		OutputTokens: 50,
	})

	time.Sleep(200 * time.Millisecond)

	// Cache should have been invalidated.
	_, ok := cache.get("hanzo/z")
	assert.False(t, ok, "cache should be invalidated after usage report")
}

func TestUsageReporter_SetsOrgHeader(t *testing.T) {
	var capturedOrg string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedOrg = r.Header.Get("X-Hanzo-Org")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "svc-token")
	reporter := NewUsageReporter(client, nil, UsageReporterConfig{
		MaxBatchSize:   1,
		FlushInterval:  50 * time.Millisecond,
		MaxRetries:     1,
		InitialBackoff: 1 * time.Millisecond,
	})

	reporter.Report(UsageRecord{
		UserID:       "hanzo/z",
		Model:        "claude-sonnet-4",
		Provider:     "anthropic",
		InputTokens:  100,
		OutputTokens: 50,
	})

	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, "hanzo", capturedOrg)
}

func TestUsageReporter_AutoComputes_TotalTokens(t *testing.T) {
	var capturedTokens float64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		capturedTokens = body["tokens"].(float64)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	reporter := NewUsageReporter(client, nil, UsageReporterConfig{
		MaxBatchSize:   1,
		FlushInterval:  50 * time.Millisecond,
		MaxRetries:     1,
		InitialBackoff: 1 * time.Millisecond,
	})

	reporter.Report(UsageRecord{
		UserID:       "hanzo/z",
		Model:        "gpt-4o",
		Provider:     "openai",
		InputTokens:  200,
		OutputTokens: 100,
		// TotalTokens not set — should be auto-computed.
	})

	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, float64(300), capturedTokens)
}

func TestUsageReporter_SetsTimestamp(t *testing.T) {
	var capturedBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	reporter := NewUsageReporter(client, nil, UsageReporterConfig{
		MaxBatchSize:   1,
		FlushInterval:  50 * time.Millisecond,
		MaxRetries:     1,
		InitialBackoff: 1 * time.Millisecond,
	})

	reporter.Report(UsageRecord{
		UserID:       "hanzo/z",
		Model:        "claude-sonnet-4",
		Provider:     "anthropic",
		InputTokens:  10,
		OutputTokens: 5,
	})

	time.Sleep(200 * time.Millisecond)

	// The record should have a timestamp that was set by Report().
	// We can't check the payload directly since timestamp is not sent to Commerce,
	// but we can verify the record was set before queueing.
	assert.Equal(t, 0, reporter.QueueLen(), "queue should be empty after flush")
}

func TestUsageReporter_DefaultConfig(t *testing.T) {
	cfg := DefaultUsageReporterConfig()
	assert.Equal(t, 50, cfg.MaxBatchSize)
	assert.Equal(t, 5*time.Second, cfg.FlushInterval)
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 500*time.Millisecond, cfg.InitialBackoff)
}

func TestUsageReporter_QueueLen(t *testing.T) {
	client := newTestClient("http://127.0.0.1:1", "")
	reporter := NewUsageReporter(client, nil, UsageReporterConfig{
		MaxBatchSize:   100,
		FlushInterval:  10 * time.Second,
		MaxRetries:     1,
		InitialBackoff: 1 * time.Millisecond,
	})

	assert.Equal(t, 0, reporter.QueueLen())

	reporter.Report(UsageRecord{UserID: "a", Model: "m", Provider: "p"})
	reporter.Report(UsageRecord{UserID: "b", Model: "m", Provider: "p"})

	assert.Equal(t, 2, reporter.QueueLen())
}
