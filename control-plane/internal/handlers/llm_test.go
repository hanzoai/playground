package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestResolveProvider(t *testing.T) {
	tests := []struct {
		model    string
		expected string
	}{
		{"claude-sonnet-4", "anthropic"},
		{"claude-opus-4-6", "anthropic"},
		{"gpt-4o", "openai"},
		{"o1-preview", "openai"},
		{"o3-mini", "openai"},
		{"gemini-pro", "google"},
		{"llama-3.1-70b", "meta"},
		{"mixtral-8x7b", "meta"},
		{"mistral-large", "meta"},
		{"zen4-pro", "hanzo"},
		{"some-unknown-model", "unknown:some-unknown-model"},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			assert.Equal(t, tt.expected, resolveProvider(tt.model))
		})
	}
}

func TestLLMChatCompletionsHandler_MissingModel(t *testing.T) {
	router := gin.New()
	router.POST("/v1/chat/completions", LLMChatCompletionsHandler(nil))

	w := httptest.NewRecorder()
	body := `{"messages": [{"role": "user", "content": "hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	errObj := resp["error"].(map[string]interface{})
	assert.Equal(t, "model is required", errObj["message"])
}

func TestLLMChatCompletionsHandler_InvalidJSON(t *testing.T) {
	router := gin.New()
	router.POST("/v1/chat/completions", LLMChatCompletionsHandler(nil))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLLMChatCompletionsHandler_UpstreamSuccess(t *testing.T) {
	// Mock upstream LLM service.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    "chatcmpl-123",
			"model": "claude-sonnet-4",
			"usage": map[string]int{
				"prompt_tokens":     100,
				"completion_tokens": 50,
				"total_tokens":      150,
			},
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]string{
						"role":    "assistant",
						"content": "Hello!",
					},
				},
			},
		})
	}))
	defer upstream.Close()

	// Override the upstream URL.
	t.Setenv("LLM_API_URL", upstream.URL)

	router := gin.New()
	router.POST("/v1/chat/completions", LLMChatCompletionsHandler(nil))

	w := httptest.NewRecorder()
	body := `{"model": "claude-sonnet-4", "messages": [{"role": "user", "content": "hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "chatcmpl-123", resp["id"])
	assert.Equal(t, "claude-sonnet-4", resp["model"])

	usage := resp["usage"].(map[string]interface{})
	assert.Equal(t, float64(100), usage["prompt_tokens"])
	assert.Equal(t, float64(50), usage["completion_tokens"])
}

func TestLLMChatCompletionsHandler_UpstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"message": "rate limited",
				"type":    "rate_limit_error",
			},
		})
	}))
	defer upstream.Close()

	t.Setenv("LLM_API_URL", upstream.URL)

	router := gin.New()
	router.POST("/v1/chat/completions", LLMChatCompletionsHandler(nil))

	w := httptest.NewRecorder()
	body := `{"model": "claude-sonnet-4", "messages": [{"role": "user", "content": "hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// Should forward the upstream error status.
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestLLMChatCompletionsHandler_UpstreamDown(t *testing.T) {
	t.Setenv("LLM_API_URL", "http://127.0.0.1:1")

	router := gin.New()
	router.POST("/v1/chat/completions", LLMChatCompletionsHandler(nil))

	w := httptest.NewRecorder()
	body := `{"model": "claude-sonnet-4", "messages": [{"role": "user", "content": "hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestLLMChatCompletionsHandler_ForwardsAPIKey(t *testing.T) {
	var capturedAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    "chatcmpl-456",
			"model": "gpt-4o",
		})
	}))
	defer upstream.Close()

	t.Setenv("LLM_API_URL", upstream.URL)
	t.Setenv("LLM_API_KEY", "sk-test-key-123")

	router := gin.New()
	router.POST("/v1/chat/completions", LLMChatCompletionsHandler(nil))

	w := httptest.NewRecorder()
	body := `{"model": "gpt-4o", "messages": [{"role": "user", "content": "test"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Bearer sk-test-key-123", capturedAuth)
}
