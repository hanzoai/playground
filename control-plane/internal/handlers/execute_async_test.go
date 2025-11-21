package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Agent-Field/agentfield/control-plane/internal/events"
	"github.com/Agent-Field/agentfield/control-plane/internal/services"
	"github.com/Agent-Field/agentfield/control-plane/pkg/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestExecuteAsyncHandler_QueueSaturation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	agent := &types.AgentNode{
		ID:        "node-1",
		BaseURL:   "http://agent.example",
		Reasoners: []types.ReasonerDefinition{{ID: "reasoner-a"}},
	}

	store := newTestExecutionStorage(agent)
	payloads := services.NewFilePayloadStore(t.TempDir())

	// Fill the async queue to saturation
	pool := getAsyncWorkerPool()
	// Submit jobs until queue is full
	for i := 0; i < cap(pool.queue)+1; i++ {
		job := asyncExecutionJob{
			controller: newExecutionController(store, payloads, nil),
			plan: preparedExecution{
				exec: &types.Execution{
					ExecutionID: "test-exec",
					RunID:       "test-run",
				},
			},
		}
		if !pool.submit(job) {
			break
		}
	}

	router := gin.New()
	router.POST("/api/v1/execute/async/:target", ExecuteAsyncHandler(store, payloads, nil))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/execute/async/node-1.reasoner-a", strings.NewReader(`{"input":{"foo":"bar"}}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusServiceUnavailable, resp.Code)

	var payload map[string]string
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
	require.Contains(t, payload["error"], "async execution queue is full")

	// Verify execution was marked as failed
	records, err := store.QueryExecutionRecords(context.Background(), types.ExecutionFilter{})
	require.NoError(t, err)
	if len(records) > 0 {
		require.Equal(t, types.ExecutionStatusFailed, records[len(records)-1].Status)
	}
}

func TestExecuteAsyncHandler_WithWebhook(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var requestCount int32
	agentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer agentServer.Close()

	agent := &types.AgentNode{
		ID:        "node-1",
		BaseURL:   agentServer.URL,
		Reasoners: []types.ReasonerDefinition{{ID: "reasoner-a"}},
	}

	store := newTestExecutionStorage(agent)
	payloads := services.NewFilePayloadStore(t.TempDir())

	router := gin.New()
	router.POST("/api/v1/execute/async/:target", ExecuteAsyncHandler(store, payloads, nil))

	reqBody := `{
		"input": {"foo": "bar"},
		"webhook": {
			"url": "https://example.com/webhook",
			"secret": "test-secret"
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/execute/async/node-1.reasoner-a", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusAccepted, resp.Code)

	var payload AsyncExecuteResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
	require.NotEmpty(t, payload.ExecutionID)
	require.True(t, payload.WebhookRegistered)

	// Wait for async execution to complete
	require.Eventually(t, func() bool {
		record, err := store.GetExecutionRecord(context.Background(), payload.ExecutionID)
		if err != nil || record == nil {
			return false
		}
		return record.Status == types.ExecutionStatusSucceeded
	}, 2*time.Second, 50*time.Millisecond)

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&requestCount) > 0
	}, time.Second, 50*time.Millisecond)
}

func TestExecuteAsyncHandler_InvalidWebhook(t *testing.T) {
	gin.SetMode(gin.TestMode)

	agent := &types.AgentNode{
		ID:        "node-1",
		BaseURL:   "http://agent.example",
		Reasoners: []types.ReasonerDefinition{{ID: "reasoner-a"}},
	}

	store := newTestExecutionStorage(agent)
	payloads := services.NewFilePayloadStore(t.TempDir())

	router := gin.New()
	router.POST("/api/v1/execute/async/:target", ExecuteAsyncHandler(store, payloads, nil))

	// Webhook with invalid URL (too long)
	longURL := strings.Repeat("a", 4097)
	reqBody := `{
		"input": {"foo": "bar"},
		"webhook": {
			"url": "` + longURL + `"
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/execute/async/node-1.reasoner-a", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusAccepted, resp.Code)

	var payload AsyncExecuteResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
	require.NotEmpty(t, payload.ExecutionID)
	require.False(t, payload.WebhookRegistered)
	require.NotNil(t, payload.WebhookError)
}

func TestHandleSync_AsyncAcknowledgment(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var requestCount int32
	agentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		// Return HTTP 202 Accepted
		w.WriteHeader(http.StatusAccepted)
	}))
	defer agentServer.Close()

	agent := &types.AgentNode{
		ID:        "node-1",
		BaseURL:   agentServer.URL,
		Reasoners: []types.ReasonerDefinition{{ID: "reasoner-a"}},
	}

	store := newTestExecutionStorage(agent)
	payloads := services.NewFilePayloadStore(t.TempDir())

	router := gin.New()
	router.POST("/api/v1/execute/:target", ExecuteHandler(store, payloads, nil))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/execute/node-1.reasoner-a", strings.NewReader(`{"input":{"foo":"bar"}}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	// Start request in goroutine since it will wait for completion
	done := make(chan bool)
	go func() {
		router.ServeHTTP(resp, req)
		done <- true
	}()

	// Simulate status update callback after a short delay
	time.Sleep(100 * time.Millisecond)
	executionID := ""
	records, _ := store.QueryExecutionRecords(context.Background(), types.ExecutionFilter{})
	if len(records) > 0 {
		executionID = records[0].ExecutionID
	}

	if executionID != "" {
		// Update execution to completed state
		_, err := store.UpdateExecutionRecord(context.Background(), executionID, func(current *types.Execution) (*types.Execution, error) {
			if current == nil {
				return nil, nil
			}
			now := time.Now().UTC()
			current.Status = types.ExecutionStatusSucceeded
			result := json.RawMessage(`{"result":"success"}`)
			current.ResultPayload = result
			completed := now
			current.CompletedAt = &completed
			duration := int64(100)
			current.DurationMS = &duration
			return current, nil
		})
		if err == nil {
			// Publish completion event
			eventBus := store.GetExecutionEventBus()
			if eventBus != nil {
				eventBus.Publish(events.ExecutionEvent{
					Type:        events.ExecutionCompleted,
					ExecutionID: executionID,
					WorkflowID:  "test-run",
					Status:      string(types.ExecutionStatusSucceeded),
					Timestamp:   time.Now(),
				})
			}
		}
	}

	// Wait for response or timeout
	select {
	case <-done:
		// Response completed
	case <-time.After(2 * time.Second):
		t.Fatal("Request timed out waiting for async completion")
	}

	// Note: In a real scenario, the sync handler would wait for the callback
	// This test verifies the async acknowledgment path exists
	require.Equal(t, int32(1), atomic.LoadInt32(&requestCount))
}

func TestCallAgent_HTTP202Response(t *testing.T) {
	gin.SetMode(gin.TestMode)

	agentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return HTTP 202 Accepted
		w.WriteHeader(http.StatusAccepted)
	}))
	defer agentServer.Close()

	agent := &types.AgentNode{
		ID:        "node-1",
		BaseURL:   agentServer.URL,
		Reasoners: []types.ReasonerDefinition{{ID: "reasoner-a"}},
	}

	store := newTestExecutionStorage(agent)
	controller := newExecutionController(store, nil, nil)

	plan := &preparedExecution{
		exec: &types.Execution{
			ExecutionID: "test-exec",
			RunID:       "test-run",
		},
		requestBody: []byte(`{"input":{"foo":"bar"}}`),
		agent:       agent,
		target: &parsedTarget{
			NodeID:     "node-1",
			TargetName: "reasoner-a",
		},
	}

	body, elapsed, asyncAccepted, err := controller.callAgent(context.Background(), plan)

	require.NoError(t, err)
	require.True(t, asyncAccepted)
	require.Nil(t, body)
	require.Greater(t, elapsed, time.Duration(0))
}

func TestCallAgent_ErrorResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	agentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer agentServer.Close()

	agent := &types.AgentNode{
		ID:        "node-1",
		BaseURL:   agentServer.URL,
		Reasoners: []types.ReasonerDefinition{{ID: "reasoner-a"}},
	}

	store := newTestExecutionStorage(agent)
	controller := newExecutionController(store, nil, nil)

	plan := &preparedExecution{
		exec: &types.Execution{
			ExecutionID: "test-exec",
			RunID:       "test-run",
		},
		requestBody: []byte(`{"input":{"foo":"bar"}}`),
		agent:       agent,
		target: &parsedTarget{
			NodeID:     "node-1",
			TargetName: "reasoner-a",
		},
	}

	body, elapsed, asyncAccepted, err := controller.callAgent(context.Background(), plan)

	require.Error(t, err)
	require.False(t, asyncAccepted)
	require.Contains(t, err.Error(), "agent error (500)")
	require.NotNil(t, body)
	require.Greater(t, elapsed, time.Duration(0))
}

func TestCallAgent_Timeout(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Server that delays response beyond timeout
	agentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer agentServer.Close()

	agent := &types.AgentNode{
		ID:        "node-1",
		BaseURL:   agentServer.URL,
		Reasoners: []types.ReasonerDefinition{{ID: "reasoner-a"}},
	}

	store := newTestExecutionStorage(agent)
	controller := newExecutionController(store, nil, nil)
	// Set shorter timeout for test
	controller.httpClient.Timeout = 100 * time.Millisecond

	plan := &preparedExecution{
		exec: &types.Execution{
			ExecutionID: "test-exec",
			RunID:       "test-run",
		},
		requestBody: []byte(`{"input":{"foo":"bar"}}`),
		agent:       agent,
		target: &parsedTarget{
			NodeID:     "node-1",
			TargetName: "reasoner-a",
		},
	}

	body, elapsed, asyncAccepted, err := controller.callAgent(context.Background(), plan)

	require.Error(t, err)
	require.False(t, asyncAccepted)
	require.Contains(t, err.Error(), "timeout")
	require.Nil(t, body)
	require.Greater(t, elapsed, time.Duration(0))
}

func TestCallAgent_ReadResponseError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	agentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Close connection immediately to cause read error
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, _ := hj.Hijack()
			conn.Close()
		}
	}))
	defer agentServer.Close()

	agent := &types.AgentNode{
		ID:        "node-1",
		BaseURL:   agentServer.URL,
		Reasoners: []types.ReasonerDefinition{{ID: "reasoner-a"}},
	}

	store := newTestExecutionStorage(agent)
	controller := newExecutionController(store, nil, nil)

	plan := &preparedExecution{
		exec: &types.Execution{
			ExecutionID: "test-exec",
			RunID:       "test-run",
		},
		requestBody: []byte(`{"input":{"foo":"bar"}}`),
		agent:       agent,
		target: &parsedTarget{
			NodeID:     "node-1",
			TargetName: "reasoner-a",
		},
	}

	body, elapsed, asyncAccepted, err := controller.callAgent(context.Background(), plan)

	require.Error(t, err)
	require.False(t, asyncAccepted)
	require.Contains(t, err.Error(), "agent call failed")
	require.Nil(t, body)
	require.Greater(t, elapsed, time.Duration(0))
}

func TestCallAgent_HeaderPropagation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var receivedHeaders http.Header
	agentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer agentServer.Close()

	agent := &types.AgentNode{
		ID:        "node-1",
		BaseURL:   agentServer.URL,
		Reasoners: []types.ReasonerDefinition{{ID: "reasoner-a"}},
	}

	store := newTestExecutionStorage(agent)
	controller := newExecutionController(store, nil, nil)

	parentID := "parent-exec-123"
	sessionID := "session-456"
	actorID := "actor-789"

	plan := &preparedExecution{
		exec: &types.Execution{
			ExecutionID:       "test-exec",
			RunID:             "test-run",
			ParentExecutionID: &parentID,
			SessionID:         &sessionID,
			ActorID:           &actorID,
		},
		requestBody: []byte(`{"input":{"foo":"bar"}}`),
		agent:       agent,
		target: &parsedTarget{
			NodeID:     "node-1",
			TargetName: "reasoner-a",
		},
	}

	_, _, _, err := controller.callAgent(context.Background(), plan)
	require.NoError(t, err)

	require.Equal(t, "test-run", receivedHeaders.Get("X-Run-ID"))
	require.Equal(t, "test-exec", receivedHeaders.Get("X-Execution-ID"))
	require.Equal(t, parentID, receivedHeaders.Get("X-Parent-Execution-ID"))
	require.Equal(t, sessionID, receivedHeaders.Get("X-Session-ID"))
	require.Equal(t, actorID, receivedHeaders.Get("X-Actor-ID"))
}
