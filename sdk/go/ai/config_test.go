package ai

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// allEnvKeys lists every environment variable that DefaultConfig reads.
var allEnvKeys = []string{
	"HANZO_API_KEY", "OPENAI_API_KEY", "OPENROUTER_API_KEY",
	"HANZO_AI_BASE_URL", "AI_BASE_URL",
	"HANZO_AI_MODEL", "AI_MODEL",
}

// saveAndClearEnv snapshots the current values and unsets them all.
// The returned function restores the original values.
func saveAndClearEnv(keys []string) func() {
	saved := make(map[string]string, len(keys))
	for _, k := range keys {
		if v, ok := os.LookupEnv(k); ok {
			saved[k] = v
		}
		os.Unsetenv(k)
	}
	return func() {
		for _, k := range keys {
			if v, ok := saved[k]; ok {
				os.Setenv(k, v)
			} else {
				os.Unsetenv(k)
			}
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	restore := saveAndClearEnv(allEnvKeys)
	defer restore()

	tests := []struct {
		name        string
		setupEnv    func()
		checkConfig func(t *testing.T, cfg *Config)
	}{
		{
			name: "default Hanzo config with OPENAI_API_KEY",
			setupEnv: func() {
				os.Setenv("OPENAI_API_KEY", "test-openai-key")
			},
			checkConfig: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "test-openai-key", cfg.APIKey)
				assert.Equal(t, "https://api.hanzo.ai/v1", cfg.BaseURL)
				assert.Equal(t, "gpt-4o", cfg.Model)
				assert.Equal(t, 0.7, cfg.Temperature)
				assert.Equal(t, 4096, cfg.MaxTokens)
				assert.Equal(t, 30*time.Second, cfg.Timeout)
			},
		},
		{
			name: "HANZO_API_KEY takes highest priority",
			setupEnv: func() {
				os.Setenv("HANZO_API_KEY", "hanzo-key")
				os.Setenv("OPENAI_API_KEY", "openai-key")
			},
			checkConfig: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "hanzo-key", cfg.APIKey)
				assert.Equal(t, "https://api.hanzo.ai/v1", cfg.BaseURL)
			},
		},
		{
			name: "OPENAI_API_KEY takes precedence over OpenRouter legacy fallback",
			setupEnv: func() {
				os.Setenv("OPENAI_API_KEY", "openai-key")
				os.Setenv("OPENROUTER_API_KEY", "openrouter-key")
			},
			checkConfig: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "openai-key", cfg.APIKey)
				assert.Equal(t, "https://api.hanzo.ai/v1", cfg.BaseURL)
			},
		},
		{
			name: "OpenRouter legacy fallback when no other key set",
			setupEnv: func() {
				os.Setenv("OPENROUTER_API_KEY", "openrouter-key")
			},
			checkConfig: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "openrouter-key", cfg.APIKey)
				assert.Equal(t, "https://openrouter.ai/api/v1", cfg.BaseURL)
			},
		},
		{
			name: "HANZO_AI_BASE_URL overrides OpenRouter base URL",
			setupEnv: func() {
				os.Setenv("OPENROUTER_API_KEY", "openrouter-key")
				os.Setenv("HANZO_AI_BASE_URL", "https://custom.hanzo.ai/v1")
			},
			checkConfig: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "openrouter-key", cfg.APIKey)
				assert.Equal(t, "https://custom.hanzo.ai/v1", cfg.BaseURL)
			},
		},
		{
			name: "custom base URL override via AI_BASE_URL",
			setupEnv: func() {
				os.Setenv("OPENAI_API_KEY", "test-key")
				os.Setenv("AI_BASE_URL", "https://custom.example.com/v1")
			},
			checkConfig: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "https://custom.example.com/v1", cfg.BaseURL)
			},
		},
		{
			name: "HANZO_AI_MODEL takes priority over AI_MODEL",
			setupEnv: func() {
				os.Setenv("OPENAI_API_KEY", "test-key")
				os.Setenv("HANZO_AI_MODEL", "hanzo-model")
				os.Setenv("AI_MODEL", "fallback-model")
			},
			checkConfig: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "hanzo-model", cfg.Model)
			},
		},
		{
			name: "custom model override via AI_MODEL",
			setupEnv: func() {
				os.Setenv("OPENAI_API_KEY", "test-key")
				os.Setenv("AI_MODEL", "gpt-3.5-turbo")
			},
			checkConfig: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "gpt-3.5-turbo", cfg.Model)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all env before each subtest.
			for _, k := range allEnvKeys {
				os.Unsetenv(k)
			}
			tt.setupEnv()

			cfg := DefaultConfig()
			require.NotNil(t, cfg)
			tt.checkConfig(t, cfg)
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				APIKey:  "test-key",
				BaseURL: "https://api.example.com/v1",
				Model:   "gpt-4o",
			},
			wantErr: false,
		},
		{
			name: "missing API key",
			config: &Config{
				APIKey:  "",
				BaseURL: "https://api.example.com/v1",
				Model:   "gpt-4o",
			},
			wantErr: true,
		},
		{
			name: "missing base URL",
			config: &Config{
				APIKey:  "test-key",
				BaseURL: "",
				Model:   "gpt-4o",
			},
			wantErr: true,
		},
		{
			name: "missing model",
			config: &Config{
				APIKey:  "test-key",
				BaseURL: "https://api.example.com/v1",
				Model:   "",
			},
			wantErr: true,
		},
		{
			name: "all fields missing",
			config: &Config{
				APIKey:  "",
				BaseURL: "",
				Model:   "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsOpenRouter(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		expected bool
	}{
		{
			name:     "OpenRouter URL without trailing slash",
			baseURL:  "https://openrouter.ai/api/v1",
			expected: true,
		},
		{
			name:     "OpenRouter URL with trailing slash",
			baseURL:  "https://openrouter.ai/api/v1/",
			expected: true,
		},
		{
			name:     "OpenAI URL",
			baseURL:  "https://api.openai.com/v1",
			expected: false,
		},
		{
			name:     "custom URL",
			baseURL:  "https://custom.example.com/v1",
			expected: false,
		},
		{
			name:     "empty URL",
			baseURL:  "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{BaseURL: tt.baseURL}
			assert.Equal(t, tt.expected, cfg.IsOpenRouter())
		})
	}
}
