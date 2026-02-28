package secrets

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// ProviderKMS uses the Hanzo KMS (Infisical-compatible API) for secret management.
const ProviderKMS ProviderType = "kms"

// defaultKMSCacheTTL is the default cache duration for resolved secrets.
const defaultKMSCacheTTL = 5 * time.Minute

// KMSConfig holds Hanzo KMS-specific settings.
type KMSConfig struct {
	SiteURL       string `yaml:"site_url" mapstructure:"site_url"`
	ClientID      string `yaml:"client_id" mapstructure:"client_id"`
	ClientSecret  string `yaml:"client_secret" mapstructure:"client_secret"`
	WorkspaceSlug string `yaml:"workspace_slug" mapstructure:"workspace_slug"`
	Environment   string `yaml:"environment" mapstructure:"environment"`
	SecretPath    string `yaml:"secret_path" mapstructure:"secret_path"`
	CacheTTLSec   int    `yaml:"cache_ttl_sec" mapstructure:"cache_ttl_sec"`
}

// kmsAuthResponse is the JSON body returned by the universal-auth login endpoint.
type kmsAuthResponse struct {
	AccessToken string `json:"accessToken"`
	ExpiresIn   int    `json:"expiresIn"`
}

// kmsSecretResponse is the JSON body returned by the v3 raw secret endpoint.
type kmsSecretResponse struct {
	Secret struct {
		SecretValue string `json:"secretValue"`
	} `json:"secret"`
}

// cachedSecret holds a resolved secret value with its expiry time.
type cachedSecret struct {
	value     string
	expiresAt time.Time
}

// KMSProvider fetches secrets from Hanzo KMS (Infisical-compatible API).
// It authenticates via universal machine identity and caches the access token.
type KMSProvider struct {
	siteURL       string
	clientID      string
	clientSecret  string
	workspaceSlug string
	environment   string
	secretPath    string
	cacheTTL      time.Duration
	httpClient    *http.Client

	// mu protects token fields and the secret cache.
	mu           sync.RWMutex
	accessToken  string
	tokenExpiry  time.Time
	secretCache  map[string]cachedSecret
}

// NewKMSProvider creates a KMS-backed secret provider.
// siteURL defaults to "https://kms.hanzo.ai" if empty.
// environment defaults to "prod" if empty.
// secretPath defaults to "/" if empty.
// cacheTTL defaults to 5 minutes if cacheTTLSec is zero or negative.
func NewKMSProvider(cfg KMSConfig) (*KMSProvider, error) {
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("kms provider: clientId is required")
	}
	if cfg.ClientSecret == "" {
		return nil, fmt.Errorf("kms provider: clientSecret is required")
	}

	siteURL := cfg.SiteURL
	if siteURL == "" {
		siteURL = "https://kms.hanzo.ai"
	}

	environment := cfg.Environment
	if environment == "" {
		environment = "prod"
	}

	secretPath := cfg.SecretPath
	if secretPath == "" {
		secretPath = "/"
	}

	cacheTTL := defaultKMSCacheTTL
	if cfg.CacheTTLSec > 0 {
		cacheTTL = time.Duration(cfg.CacheTTLSec) * time.Second
	}

	return &KMSProvider{
		siteURL:       siteURL,
		clientID:      cfg.ClientID,
		clientSecret:  cfg.ClientSecret,
		workspaceSlug: cfg.WorkspaceSlug,
		environment:   environment,
		secretPath:    secretPath,
		cacheTTL:      cacheTTL,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		secretCache:   make(map[string]cachedSecret),
	}, nil
}

// Type returns ProviderKMS.
func (p *KMSProvider) Type() ProviderType {
	return ProviderKMS
}

// GetSecret retrieves a secret value by key from Hanzo KMS.
// Cached values are returned when fresh. Returns ("", nil) for missing keys.
func (p *KMSProvider) GetSecret(ctx context.Context, key string) (string, error) {
	if key == "" {
		return "", nil
	}

	normalised := normaliseKey(key)

	// Check the cache first (read lock).
	p.mu.RLock()
	if cached, ok := p.secretCache[normalised]; ok && time.Now().Before(cached.expiresAt) {
		p.mu.RUnlock()
		return cached.value, nil
	}
	p.mu.RUnlock()

	// Ensure we have a valid access token.
	if err := p.ensureAuthenticated(ctx); err != nil {
		return "", fmt.Errorf("kms auth failed: %w", err)
	}

	// Fetch the secret from the KMS API.
	value, err := p.fetchSecret(ctx, normalised)
	if err != nil {
		return "", err
	}

	// Cache the resolved value.
	p.mu.Lock()
	p.secretCache[normalised] = cachedSecret{
		value:     value,
		expiresAt: time.Now().Add(p.cacheTTL),
	}
	p.mu.Unlock()

	return value, nil
}

// SetSecret is not supported by the KMS provider (read-only).
func (p *KMSProvider) SetSecret(_ context.Context, _, _ string) error {
	return fmt.Errorf("kms provider: SetSecret is not supported (read-only provider)")
}

// DeleteSecret is not supported by the KMS provider (read-only).
func (p *KMSProvider) DeleteSecret(_ context.Context, _ string) error {
	return fmt.Errorf("kms provider: DeleteSecret is not supported (read-only provider)")
}

// ensureAuthenticated obtains or refreshes the access token. It applies a
// 30-second safety margin so that tokens are refreshed before they expire.
func (p *KMSProvider) ensureAuthenticated(ctx context.Context) error {
	p.mu.RLock()
	if p.accessToken != "" && time.Now().Add(30*time.Second).Before(p.tokenExpiry) {
		p.mu.RUnlock()
		return nil
	}
	p.mu.RUnlock()

	return p.authenticate(ctx)
}

// authenticate performs the universal-auth login and stores the access token.
func (p *KMSProvider) authenticate(ctx context.Context) error {
	loginURL := p.siteURL + "/api/v1/auth/universal-auth/login"

	body, err := json.Marshal(map[string]string{
		"clientId":     p.clientID,
		"clientSecret": p.clientSecret,
	})
	if err != nil {
		return fmt.Errorf("kms: failed to marshal auth request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("kms: failed to create auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("kms: auth request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kms: auth returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var authResp kmsAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return fmt.Errorf("kms: failed to decode auth response: %w", err)
	}

	if authResp.AccessToken == "" {
		return fmt.Errorf("kms: auth response contained empty access token")
	}

	p.mu.Lock()
	p.accessToken = authResp.AccessToken
	p.tokenExpiry = time.Now().Add(time.Duration(authResp.ExpiresIn) * time.Second)
	p.mu.Unlock()

	return nil
}

// fetchSecret calls the v3 raw secret endpoint for a single key.
// Returns ("", nil) when the secret is not found (HTTP 404).
func (p *KMSProvider) fetchSecret(ctx context.Context, key string) (string, error) {
	p.mu.RLock()
	token := p.accessToken
	p.mu.RUnlock()

	secretURL := fmt.Sprintf("%s/api/v3/secrets/raw/%s", p.siteURL, url.PathEscape(key))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, secretURL, nil)
	if err != nil {
		return "", fmt.Errorf("kms: failed to create secret request: %w", err)
	}

	q := req.URL.Query()
	if p.workspaceSlug != "" {
		q.Set("workspaceSlug", p.workspaceSlug)
	}
	q.Set("environment", p.environment)
	q.Set("secretPath", p.secretPath)
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("kms: secret request failed for %q: %w", key, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("kms: secret request for %q returned status %d: %s", key, resp.StatusCode, string(respBody))
	}

	var secretResp kmsSecretResponse
	if err := json.NewDecoder(resp.Body).Decode(&secretResp); err != nil {
		return "", fmt.Errorf("kms: failed to decode secret response for %q: %w", key, err)
	}

	return secretResp.Secret.SecretValue, nil
}
