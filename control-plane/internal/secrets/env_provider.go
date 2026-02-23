package secrets

import (
	"context"
	"os"
	"strings"
	"sync"
)

// EnvProvider reads and writes secrets from environment variables.
// It is the default provider and preserves backward compatibility with
// the existing PLAYGROUND_* / AGENTS_* env-var convention.
type EnvProvider struct {
	prefix string

	// mu protects the overrides map for SetSecret / DeleteSecret.
	mu        sync.RWMutex
	overrides map[string]string
	deleted   map[string]struct{}
}

// NewEnvProvider creates an environment-variable backed secret provider.
// If prefix is empty it defaults to "PLAYGROUND_".
func NewEnvProvider(prefix string) *EnvProvider {
	if prefix == "" {
		prefix = "PLAYGROUND_"
	}
	return &EnvProvider{
		prefix:    prefix,
		overrides: make(map[string]string),
		deleted:   make(map[string]struct{}),
	}
}

// GetSecret reads a secret from environment variables.
//
// Resolution order:
//  1. In-process overrides (set via SetSecret)
//  2. PLAYGROUND_<KEY>
//  3. AGENTS_<KEY>  (legacy fallback)
//  4. <KEY>         (bare name fallback for direct env vars)
//
// Returns ("", nil) when the key is not found.
func (p *EnvProvider) GetSecret(_ context.Context, key string) (string, error) {
	if key == "" {
		return "", nil
	}

	normalised := normaliseKey(key)

	p.mu.RLock()
	defer p.mu.RUnlock()

	// Check explicit deletes first.
	if _, ok := p.deleted[normalised]; ok {
		return "", nil
	}

	// In-process override wins.
	if v, ok := p.overrides[normalised]; ok {
		return v, nil
	}

	// Try prefixed env var.
	if v := os.Getenv(p.prefix + normalised); v != "" {
		return v, nil
	}

	// Legacy AGENTS_ fallback.
	if p.prefix != "AGENTS_" {
		if v := os.Getenv("AGENTS_" + normalised); v != "" {
			return v, nil
		}
	}

	// Bare key fallback (for variables like IAM_CLIENT_SECRET set without prefix).
	if v := os.Getenv(normalised); v != "" {
		return v, nil
	}

	return "", nil
}

// SetSecret stores a secret as an in-process override.
// The value is also set in the OS environment so child processes inherit it.
func (p *EnvProvider) SetSecret(_ context.Context, key, value string) error {
	if key == "" {
		return nil
	}

	normalised := normaliseKey(key)

	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.deleted, normalised)
	p.overrides[normalised] = value

	// Mirror to the OS environment for child processes.
	return os.Setenv(p.prefix+normalised, value)
}

// DeleteSecret removes a secret from the in-process overrides and clears
// the corresponding environment variable.
func (p *EnvProvider) DeleteSecret(_ context.Context, key string) error {
	if key == "" {
		return nil
	}

	normalised := normaliseKey(key)

	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.overrides, normalised)
	p.deleted[normalised] = struct{}{}

	// Best-effort clear from the OS environment.
	_ = os.Unsetenv(p.prefix + normalised)
	_ = os.Unsetenv("AGENTS_" + normalised)
	_ = os.Unsetenv(normalised)

	return nil
}

// Type returns ProviderEnv.
func (p *EnvProvider) Type() ProviderType {
	return ProviderEnv
}

// normaliseKey upper-cases and replaces dashes/dots with underscores so that
// callers can use any reasonable key format (e.g. "api-key", "api.key",
// "API_KEY") and the provider resolves them uniformly.
func normaliseKey(key string) string {
	k := strings.ToUpper(key)
	k = strings.ReplaceAll(k, "-", "_")
	k = strings.ReplaceAll(k, ".", "_")
	return k
}
