package ai

import (
	"errors"
	"os"
	"time"
)

// Config holds AI/LLM configuration for making API calls.
type Config struct {
	// API Key for OpenAI or OpenRouter
	APIKey string

	// BaseURL can be either OpenAI or OpenRouter endpoint
	// Default: https://api.openai.com/v1
	// OpenRouter: https://openrouter.ai/api/v1
	BaseURL string

	// Default model to use (e.g., "gpt-4o", "openai/gpt-4o" for OpenRouter)
	Model string

	// Default temperature for responses (0.0 to 2.0)
	Temperature float64

	// Default max tokens for responses
	MaxTokens int

	// HTTP timeout for requests
	Timeout time.Duration

	// Optional: Site URL for OpenRouter rankings
	SiteURL string

	// Optional: Site name for OpenRouter rankings
	SiteName string
}

// envOr reads the first non-empty value from the given env var names.
func envOr(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

// DefaultConfig returns a Config with sensible defaults.
// It reads from environment variables (HANZO_ prefix preferred, with fallback):
//   - HANZO_API_KEY / OPENAI_API_KEY / OPENROUTER_API_KEY
//   - HANZO_AI_BASE_URL / AI_BASE_URL (defaults to OpenAI)
//   - HANZO_AI_MODEL / AI_MODEL (defaults to gpt-4o)
func DefaultConfig() *Config {
	// HANZO_API_KEY takes highest priority.
	apiKey := os.Getenv("HANZO_API_KEY")
	baseURL := "https://api.openai.com/v1"

	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	// Check for OpenRouter configuration -- takes precedence over plain OPENAI_API_KEY.
	if routerKey := os.Getenv("OPENROUTER_API_KEY"); routerKey != "" {
		apiKey = routerKey
		baseURL = "https://openrouter.ai/api/v1"
	}

	// Allow override via HANZO_AI_BASE_URL or AI_BASE_URL.
	if customURL := envOr("HANZO_AI_BASE_URL", "AI_BASE_URL"); customURL != "" {
		baseURL = customURL
	}

	model := envOr("HANZO_AI_MODEL", "AI_MODEL")
	if model == "" {
		model = "gpt-4o"
	}

	return &Config{
		APIKey:      apiKey,
		BaseURL:     baseURL,
		Model:       model,
		Temperature: 0.7,
		MaxTokens:   4096,
		Timeout:     30 * time.Second,
	}
}

// Validate ensures the configuration is valid.
func (c *Config) Validate() error {
	if c.APIKey == "" {
		return errors.New("API key is required")
	}
	if c.BaseURL == "" {
		return errors.New("base URL is required")
	}
	if c.Model == "" {
		return errors.New("model is required")
	}
	return nil
}

// IsOpenRouter returns true if the base URL is for OpenRouter.
func (c *Config) IsOpenRouter() bool {
	return c.BaseURL == "https://openrouter.ai/api/v1" ||
		c.BaseURL == "https://openrouter.ai/api/v1/"
}
