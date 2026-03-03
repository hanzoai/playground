package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/control-plane/internal/billing"
	"github.com/hanzoai/playground/control-plane/internal/logger"
)

// llmHTTPClient is the shared HTTP client for LLM proxy calls.
var llmHTTPClient = &http.Client{Timeout: 120 * time.Second}

// llmUpstreamURL returns the upstream LLM API endpoint.
func llmUpstreamURL() string {
	if v := os.Getenv("LLM_API_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	if v := os.Getenv("HANZO_PLAYGROUND_CLOUD_API_ENDPOINT"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "https://api.hanzo.ai/v1"
}

// llmAPIKey returns the API key for authenticating with the upstream LLM service.
func llmAPIKey() string {
	if v := os.Getenv("LLM_API_KEY"); v != "" {
		return v
	}
	return os.Getenv("HANZO_PLAYGROUND_CLOUD_API_KEY")
}

// chatCompletionsRequest is a minimal representation of the OpenAI-compatible
// chat completions request body. We only parse the fields needed for billing.
type chatCompletionsRequest struct {
	Model    string `json:"model"`
	Stream   bool   `json:"stream,omitempty"`
	MaxTokens int   `json:"max_tokens,omitempty"`
}

// chatCompletionsResponse is a minimal representation of the OpenAI-compatible
// chat completions response body. We parse usage for billing.
type chatCompletionsResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Usage   *chatUsage `json:"usage,omitempty"`
	Error   *chatError `json:"error,omitempty"`
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type chatError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// LLMChatCompletionsHandler proxies chat/completions requests to the upstream
// LLM service (api.hanzo.ai) and reports token usage to Commerce billing.
//
// The billing gate middleware runs before this handler and ensures the user
// has sufficient balance. After the upstream response, token usage is reported
// asynchronously via the UsageReporter.
func LLMChatCompletionsHandler(reporter *billing.UsageReporter) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Read the request body so we can inspect the model.
		bodyBytes, err := io.ReadAll(io.LimitReader(c.Request.Body, 10<<20)) // 10 MB limit
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": "failed to read request body",
					"type":    "invalid_request_error",
				},
			})
			return
		}

		var reqBody chatCompletionsRequest
		if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": "invalid JSON in request body",
					"type":    "invalid_request_error",
				},
			})
			return
		}

		if reqBody.Model == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": "model is required",
					"type":    "invalid_request_error",
				},
			})
			return
		}

		// Build the upstream request.
		upstreamURL := llmUpstreamURL() + "/chat/completions"
		proxyReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, upstreamURL, bytes.NewReader(bodyBytes))
		if err != nil {
			logger.Logger.Error().Err(err).Msg("LLM proxy: failed to create upstream request")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"message": "internal proxy error",
					"type":    "server_error",
				},
			})
			return
		}

		proxyReq.Header.Set("Content-Type", "application/json")
		proxyReq.Header.Set("Accept", "application/json")

		// Use the configured API key for upstream auth.
		apiKey := llmAPIKey()
		if apiKey != "" {
			proxyReq.Header.Set("Authorization", "Bearer "+apiKey)
		}

		// Forward the request to the upstream LLM service.
		resp, err := llmHTTPClient.Do(proxyReq)
		if err != nil {
			logger.Logger.Error().Err(err).Str("url", upstreamURL).Msg("LLM proxy: upstream request failed")
			c.JSON(http.StatusBadGateway, gin.H{
				"error": gin.H{
					"message": "LLM service is temporarily unavailable",
					"type":    "server_error",
				},
			})
			return
		}
		defer resp.Body.Close()

		// Read the upstream response.
		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20)) // 50 MB limit
		if err != nil {
			logger.Logger.Error().Err(err).Msg("LLM proxy: failed to read upstream response")
			c.JSON(http.StatusBadGateway, gin.H{
				"error": gin.H{
					"message": "failed to read LLM response",
					"type":    "server_error",
				},
			})
			return
		}

		// Forward the response headers.
		for key, values := range resp.Header {
			for _, v := range values {
				c.Header(key, v)
			}
		}

		// Write the response status and body.
		c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)

		// Report usage asynchronously if the request succeeded and we have token counts.
		// Extract the gate result before the goroutine since gin recycles context objects.
		if resp.StatusCode >= 200 && resp.StatusCode < 300 && reporter != nil {
			gateResult := billing.GetLLMGateResult(c)
			duration := time.Since(start)
			go reportLLMUsage(reporter, gateResult, reqBody.Model, respBody, duration)
		}
	}
}

// reportLLMUsage parses the upstream response for token usage and enqueues
// a billing record. Runs in a goroutine — errors are logged, not propagated.
func reportLLMUsage(reporter *billing.UsageReporter, gateResult *billing.LLMGateResult, model string, respBody []byte, duration time.Duration) {
	var respData chatCompletionsResponse
	if err := json.Unmarshal(respBody, &respData); err != nil {
		logger.Logger.Warn().Err(err).Msg("LLM billing: failed to parse response for usage")
		return
	}

	if respData.Usage == nil {
		logger.Logger.Debug().Str("model", model).Msg("LLM billing: no usage data in response")
		return
	}

	userID := ""
	if gateResult != nil {
		userID = gateResult.UserID
	}
	if userID == "" {
		logger.Logger.Warn().Msg("LLM billing: no user ID for usage report")
		return
	}

	provider := resolveProvider(model)

	reporter.Report(billing.UsageRecord{
		UserID:       userID,
		Model:        model,
		Provider:     provider,
		InputTokens:  respData.Usage.PromptTokens,
		OutputTokens: respData.Usage.CompletionTokens,
		TotalTokens:  respData.Usage.TotalTokens,
		DurationMs:   int(duration.Milliseconds()),
		Timestamp:    time.Now().UnixMilli(),
	})

	logger.Logger.Debug().
		Str("user", userID).
		Str("model", model).
		Int("prompt_tokens", respData.Usage.PromptTokens).
		Int("completion_tokens", respData.Usage.CompletionTokens).
		Msg("LLM billing: usage reported")
}

// messagesRequest is a minimal representation of the Anthropic Messages API
// request body. We only parse the fields needed for billing.
type messagesRequest struct {
	Model     string `json:"model"`
	Stream    bool   `json:"stream,omitempty"`
	MaxTokens int    `json:"max_tokens,omitempty"`
}

// messagesResponse is a minimal representation of the Anthropic Messages API
// response body. We parse usage for billing.
type messagesResponse struct {
	ID    string         `json:"id"`
	Model string         `json:"model"`
	Usage *messagesUsage `json:"usage,omitempty"`
	Error *chatError     `json:"error,omitempty"`
}

type messagesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// LLMMessagesHandler proxies Anthropic-compatible /messages requests to the
// upstream LLM service and reports token usage to Commerce billing.
func LLMMessagesHandler(reporter *billing.UsageReporter) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		bodyBytes, err := io.ReadAll(io.LimitReader(c.Request.Body, 10<<20))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"type":    "error",
				"message": "failed to read request body",
			})
			return
		}

		var reqBody messagesRequest
		if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"type":    "error",
				"message": "invalid JSON in request body",
			})
			return
		}

		if reqBody.Model == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"type":    "error",
				"message": "model is required",
			})
			return
		}

		// Build the upstream request — route to /messages on the upstream.
		upstreamURL := llmUpstreamURL() + "/messages"
		proxyReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, upstreamURL, bytes.NewReader(bodyBytes))
		if err != nil {
			logger.Logger.Error().Err(err).Msg("LLM proxy: failed to create upstream request")
			c.JSON(http.StatusInternalServerError, gin.H{
				"type":    "error",
				"message": "internal proxy error",
			})
			return
		}

		proxyReq.Header.Set("Content-Type", "application/json")
		proxyReq.Header.Set("Accept", "application/json")

		apiKey := llmAPIKey()
		if apiKey != "" {
			// Anthropic uses x-api-key header, but also support Bearer for proxy compat.
			proxyReq.Header.Set("x-api-key", apiKey)
			proxyReq.Header.Set("Authorization", "Bearer "+apiKey)
		}

		// Forward anthropic-version header if present.
		if v := c.GetHeader("anthropic-version"); v != "" {
			proxyReq.Header.Set("anthropic-version", v)
		}

		resp, err := llmHTTPClient.Do(proxyReq)
		if err != nil {
			logger.Logger.Error().Err(err).Str("url", upstreamURL).Msg("LLM proxy: upstream request failed")
			c.JSON(http.StatusBadGateway, gin.H{
				"type":    "error",
				"message": "LLM service is temporarily unavailable",
			})
			return
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20))
		if err != nil {
			logger.Logger.Error().Err(err).Msg("LLM proxy: failed to read upstream response")
			c.JSON(http.StatusBadGateway, gin.H{
				"type":    "error",
				"message": "failed to read LLM response",
			})
			return
		}

		for key, values := range resp.Header {
			for _, v := range values {
				c.Header(key, v)
			}
		}

		c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)

		// Report usage asynchronously.
		if resp.StatusCode >= 200 && resp.StatusCode < 300 && reporter != nil {
			gateResult := billing.GetLLMGateResult(c)
			duration := time.Since(start)
			go reportMessagesUsage(reporter, gateResult, reqBody.Model, respBody, duration)
		}
	}
}

// reportMessagesUsage parses the Anthropic Messages response for token usage
// and enqueues a billing record.
func reportMessagesUsage(reporter *billing.UsageReporter, gateResult *billing.LLMGateResult, model string, respBody []byte, duration time.Duration) {
	var respData messagesResponse
	if err := json.Unmarshal(respBody, &respData); err != nil {
		logger.Logger.Warn().Err(err).Msg("LLM billing: failed to parse messages response for usage")
		return
	}

	if respData.Usage == nil {
		logger.Logger.Debug().Str("model", model).Msg("LLM billing: no usage data in messages response")
		return
	}

	userID := ""
	if gateResult != nil {
		userID = gateResult.UserID
	}
	if userID == "" {
		logger.Logger.Warn().Msg("LLM billing: no user ID for usage report")
		return
	}

	provider := resolveProvider(model)
	totalTokens := respData.Usage.InputTokens + respData.Usage.OutputTokens

	reporter.Report(billing.UsageRecord{
		UserID:       userID,
		Model:        model,
		Provider:     provider,
		InputTokens:  respData.Usage.InputTokens,
		OutputTokens: respData.Usage.OutputTokens,
		TotalTokens:  totalTokens,
		DurationMs:   int(duration.Milliseconds()),
		Timestamp:    time.Now().UnixMilli(),
	})

	logger.Logger.Debug().
		Str("user", userID).
		Str("model", model).
		Int("input_tokens", respData.Usage.InputTokens).
		Int("output_tokens", respData.Usage.OutputTokens).
		Msg("LLM billing: messages usage reported")
}

// resolveProvider maps model names to their provider for billing attribution.
func resolveProvider(model string) string {
	lower := strings.ToLower(model)
	switch {
	case strings.Contains(lower, "claude"):
		return "anthropic"
	case strings.Contains(lower, "gpt") || strings.Contains(lower, "o1") || strings.Contains(lower, "o3"):
		return "openai"
	case strings.Contains(lower, "gemini"):
		return "google"
	case strings.Contains(lower, "llama") || strings.Contains(lower, "mixtral") || strings.Contains(lower, "mistral"):
		return "meta"
	case strings.Contains(lower, "zen"):
		return "hanzo"
	default:
		return fmt.Sprintf("unknown:%s", model)
	}
}
