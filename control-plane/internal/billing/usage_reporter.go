package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hanzoai/playground/control-plane/internal/logger"
)

// UsageRecord represents a single LLM usage event to report to Commerce.
type UsageRecord struct {
	UserID           string `json:"user_id"`
	Model            string `json:"model"`
	Provider         string `json:"provider"`
	InputTokens      int    `json:"input_tokens"`
	OutputTokens     int    `json:"output_tokens"`
	CacheReadTokens  int    `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int    `json:"cache_write_tokens,omitempty"`
	TotalTokens      int    `json:"total_tokens"`
	DurationMs       int    `json:"duration_ms,omitempty"`
	AmountCents      int    `json:"amount_cents,omitempty"`
	NodeID           string `json:"node_id,omitempty"`
	Timestamp        int64  `json:"timestamp"`
}

// UsageReporterConfig configures the async usage reporter.
type UsageReporterConfig struct {
	MaxBatchSize  int
	FlushInterval time.Duration
	MaxRetries    int
	InitialBackoff time.Duration
}

// DefaultUsageReporterConfig returns production defaults.
func DefaultUsageReporterConfig() UsageReporterConfig {
	return UsageReporterConfig{
		MaxBatchSize:   50,
		FlushInterval:  5 * time.Second,
		MaxRetries:     3,
		InitialBackoff: 500 * time.Millisecond,
	}
}

// UsageReporter asynchronously reports LLM usage to Commerce API.
// Records are batched and flushed periodically or when the batch is full.
type UsageReporter struct {
	client     *Client
	config     UsageReporterConfig
	cache      *balanceCache // invalidated after usage to force fresh balance

	mu    sync.Mutex
	queue []UsageRecord
	timer *time.Timer
}

// NewUsageReporter creates a new usage reporter. Pass a balanceCache
// so the reporter can invalidate cached balances after recording usage.
func NewUsageReporter(client *Client, cache *balanceCache, cfg UsageReporterConfig) *UsageReporter {
	if cfg.MaxBatchSize <= 0 {
		cfg.MaxBatchSize = 50
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Second
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.InitialBackoff <= 0 {
		cfg.InitialBackoff = 500 * time.Millisecond
	}
	return &UsageReporter{
		client: client,
		config: cfg,
		cache:  cache,
		queue:  make([]UsageRecord, 0, cfg.MaxBatchSize),
	}
}

// Report enqueues a usage record for async delivery. Non-blocking.
func (r *UsageReporter) Report(record UsageRecord) {
	if record.Timestamp == 0 {
		record.Timestamp = time.Now().UnixMilli()
	}
	if record.TotalTokens == 0 {
		record.TotalTokens = record.InputTokens + record.OutputTokens
	}

	r.mu.Lock()
	r.queue = append(r.queue, record)
	queueLen := len(r.queue)

	// Flush immediately when batch is full.
	if queueLen >= r.config.MaxBatchSize {
		r.stopTimerLocked()
		batch := r.drainLocked()
		r.mu.Unlock()
		go r.flushBatch(batch)
		return
	}

	// Schedule a flush if not already pending.
	if r.timer == nil {
		r.timer = time.AfterFunc(r.config.FlushInterval, func() {
			r.mu.Lock()
			r.timer = nil
			batch := r.drainLocked()
			r.mu.Unlock()
			if len(batch) > 0 {
				r.flushBatch(batch)
			}
		})
	}
	r.mu.Unlock()
}

// Shutdown flushes all remaining records. Call on graceful shutdown.
func (r *UsageReporter) Shutdown() {
	r.mu.Lock()
	r.stopTimerLocked()
	batch := r.drainLocked()
	r.mu.Unlock()

	if len(batch) > 0 {
		r.flushBatch(batch)
	}
}

// QueueLen returns the current queue length (for testing/monitoring).
func (r *UsageReporter) QueueLen() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.queue)
}

// drainLocked takes all records from the queue. Caller must hold r.mu.
func (r *UsageReporter) drainLocked() []UsageRecord {
	if len(r.queue) == 0 {
		return nil
	}
	batch := r.queue
	r.queue = make([]UsageRecord, 0, r.config.MaxBatchSize)
	return batch
}

// stopTimerLocked stops the pending flush timer. Caller must hold r.mu.
func (r *UsageReporter) stopTimerLocked() {
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
}

// flushBatch sends a batch of records to Commerce API.
func (r *UsageReporter) flushBatch(batch []UsageRecord) {
	baseURL := r.client.baseURL
	serviceToken := r.client.serviceToken

	for _, record := range batch {
		if err := r.sendRecord(baseURL, serviceToken, record); err != nil {
			logger.Logger.Warn().
				Err(err).
				Str("user", record.UserID).
				Str("model", record.Model).
				Int("tokens", record.TotalTokens).
				Msg("usage-reporter: failed to report usage")
		} else {
			// Invalidate the balance cache for this user so the next
			// billing gate check fetches a fresh balance.
			if r.cache != nil {
				r.cache.invalidate(record.UserID)
			}
		}
	}
}

// sendRecord posts a single usage record with retry logic.
func (r *UsageReporter) sendRecord(baseURL, serviceToken string, record UsageRecord) error {
	payload := map[string]interface{}{
		"user_id":           strings.ToLower(record.UserID),
		"currency":          "usd",
		"amount":            record.AmountCents,
		"model":             record.Model,
		"provider":          record.Provider,
		"tokens":            record.TotalTokens,
		"promptTokens":      record.InputTokens,
		"completionTokens":  record.OutputTokens,
	}
	if record.NodeID != "" {
		payload["nodeId"] = record.NodeID
	}
	if record.CacheReadTokens > 0 {
		payload["cacheReadTokens"] = record.CacheReadTokens
	}
	if record.CacheWriteTokens > 0 {
		payload["cacheWriteTokens"] = record.CacheWriteTokens
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("usage-reporter: marshal: %w", err)
	}

	url := baseURL + "/api/v1/billing/usage"
	var lastErr error

	for attempt := 0; attempt < r.config.MaxRetries; attempt++ {
		req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(body))
		if reqErr != nil {
			return fmt.Errorf("usage-reporter: create request: %w", reqErr)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		if serviceToken != "" {
			req.Header.Set("Authorization", "Bearer "+serviceToken)
		}
		// Set org header derived from user_id "org/name".
		if parts := strings.SplitN(record.UserID, "/", 2); len(parts) == 2 {
			req.Header.Set("X-Hanzo-Org", parts[0])
		}

		resp, doErr := r.client.httpClient.Do(req)
		if doErr != nil {
			lastErr = doErr
			logger.Logger.Warn().Err(doErr).
				Int("attempt", attempt+1).
				Int("max", r.config.MaxRetries).
				Msg("usage-reporter: request error")
			r.backoff(attempt)
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			resp.Body.Close()
			return nil
		}

		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		resp.Body.Close()

		// 4xx errors are not retryable.
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return fmt.Errorf("usage-reporter: non-retryable %d: %s", resp.StatusCode, string(respBody))
		}

		// 5xx — retry.
		lastErr = fmt.Errorf("usage-reporter: Commerce API %d: %s", resp.StatusCode, string(respBody))
		logger.Logger.Warn().
			Int("status", resp.StatusCode).
			Int("attempt", attempt+1).
			Int("max", r.config.MaxRetries).
			Msg("usage-reporter: retryable error")
		r.backoff(attempt)
	}

	return fmt.Errorf("usage-reporter: all retries exhausted: %w", lastErr)
}

// backoff sleeps with exponential backoff before the next retry.
func (r *UsageReporter) backoff(attempt int) {
	delay := r.config.InitialBackoff * time.Duration(math.Pow(2, float64(attempt)))
	time.Sleep(delay)
}

