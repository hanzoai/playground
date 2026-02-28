package secrets

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// newTestKMSServer starts an httptest.Server that mimics the Hanzo KMS API.
// It returns the server, a function to set secret values, and a request counter.
func newTestKMSServer(t *testing.T) (*httptest.Server, func(key, value string), *atomic.Int64) {
	t.Helper()

	var mu sync.RWMutex
	store := map[string]string{}
	authCounter := &atomic.Int64{}

	setSecret := func(key, value string) {
		mu.Lock()
		store[key] = value
		mu.Unlock()
	}

	mux := http.NewServeMux()

	// Auth endpoint.
	mux.HandleFunc("/api/v1/auth/universal-auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			ClientID     string `json:"clientId"`
			ClientSecret string `json:"clientSecret"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		if body.ClientID != "test-client-id" || body.ClientSecret != "test-client-secret" {
			http.Error(w, `{"message":"invalid credentials"}`, http.StatusUnauthorized)
			return
		}

		authCounter.Add(1)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(kmsAuthResponse{
			AccessToken: "test-access-token",
			ExpiresIn:   7200,
		})
	})

	// Secret fetch endpoint: /api/v3/secrets/raw/{secretName}
	mux.HandleFunc("/api/v3/secrets/raw/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-access-token" {
			http.Error(w, `{"message":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		// Extract secret name from the URL path after the prefix.
		secretName := r.URL.Path[len("/api/v3/secrets/raw/"):]
		if secretName == "" {
			http.Error(w, "missing secret name", http.StatusBadRequest)
			return
		}

		mu.RLock()
		val, ok := store[secretName]
		mu.RUnlock()

		if !ok {
			http.Error(w, `{"message":"secret not found"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		resp := kmsSecretResponse{}
		resp.Secret.SecretValue = val
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv, setSecret, authCounter
}

func testKMSConfig(siteURL string) KMSConfig {
	return KMSConfig{
		SiteURL:       siteURL,
		ClientID:      "test-client-id",
		ClientSecret:  "test-client-secret",
		WorkspaceSlug: "test-workspace",
		Environment:   "test",
		SecretPath:    "/",
		CacheTTLSec:   300,
	}
}

func TestKMSProvider_Type(t *testing.T) {
	srv, _, _ := newTestKMSServer(t)
	p, err := NewKMSProvider(testKMSConfig(srv.URL))
	require.NoError(t, err)
	require.Equal(t, ProviderKMS, p.Type())
}

func TestKMSProvider_GetSecret_EmptyKey(t *testing.T) {
	srv, _, _ := newTestKMSServer(t)
	p, err := NewKMSProvider(testKMSConfig(srv.URL))
	require.NoError(t, err)

	val, err := p.GetSecret(context.Background(), "")
	require.NoError(t, err)
	require.Empty(t, val)
}

func TestKMSProvider_GetSecret_Found(t *testing.T) {
	srv, setSecret, _ := newTestKMSServer(t)
	setSecret("MY_SECRET", "super-secret-value")

	p, err := NewKMSProvider(testKMSConfig(srv.URL))
	require.NoError(t, err)

	val, err := p.GetSecret(context.Background(), "MY_SECRET")
	require.NoError(t, err)
	require.Equal(t, "super-secret-value", val)
}

func TestKMSProvider_GetSecret_NotFound(t *testing.T) {
	srv, _, _ := newTestKMSServer(t)
	p, err := NewKMSProvider(testKMSConfig(srv.URL))
	require.NoError(t, err)

	val, err := p.GetSecret(context.Background(), "NONEXISTENT_KEY")
	require.NoError(t, err)
	require.Empty(t, val)
}

func TestKMSProvider_GetSecret_KeyNormalisation(t *testing.T) {
	srv, setSecret, _ := newTestKMSServer(t)
	setSecret("MY_API_KEY", "normalised-value")

	p, err := NewKMSProvider(testKMSConfig(srv.URL))
	require.NoError(t, err)

	// Dashes and dots should normalise to underscored upper-case.
	val, err := p.GetSecret(context.Background(), "my-api.key")
	require.NoError(t, err)
	require.Equal(t, "normalised-value", val)
}

func TestKMSProvider_GetSecret_CachesValue(t *testing.T) {
	srv, setSecret, _ := newTestKMSServer(t)
	setSecret("CACHED_KEY", "first-value")

	p, err := NewKMSProvider(testKMSConfig(srv.URL))
	require.NoError(t, err)

	// First fetch populates the cache.
	val, err := p.GetSecret(context.Background(), "CACHED_KEY")
	require.NoError(t, err)
	require.Equal(t, "first-value", val)

	// Change the upstream value.
	setSecret("CACHED_KEY", "updated-value")

	// Second fetch returns the cached value.
	val, err = p.GetSecret(context.Background(), "CACHED_KEY")
	require.NoError(t, err)
	require.Equal(t, "first-value", val)
}

func TestKMSProvider_GetSecret_CacheExpiry(t *testing.T) {
	srv, setSecret, _ := newTestKMSServer(t)
	setSecret("EXPIRY_KEY", "first")

	cfg := testKMSConfig(srv.URL)
	cfg.CacheTTLSec = 1 // 1 second TTL for testing
	p, err := NewKMSProvider(cfg)
	require.NoError(t, err)

	val, err := p.GetSecret(context.Background(), "EXPIRY_KEY")
	require.NoError(t, err)
	require.Equal(t, "first", val)

	// Update upstream and wait for cache to expire.
	setSecret("EXPIRY_KEY", "second")
	time.Sleep(1100 * time.Millisecond)

	val, err = p.GetSecret(context.Background(), "EXPIRY_KEY")
	require.NoError(t, err)
	require.Equal(t, "second", val)
}

func TestKMSProvider_GetSecret_AuthCaching(t *testing.T) {
	srv, setSecret, authCounter := newTestKMSServer(t)
	setSecret("A", "1")
	setSecret("B", "2")

	p, err := NewKMSProvider(testKMSConfig(srv.URL))
	require.NoError(t, err)

	ctx := context.Background()

	// First call authenticates.
	_, err = p.GetSecret(ctx, "A")
	require.NoError(t, err)
	require.Equal(t, int64(1), authCounter.Load())

	// Second call reuses the token.
	_, err = p.GetSecret(ctx, "B")
	require.NoError(t, err)
	require.Equal(t, int64(1), authCounter.Load())
}

func TestKMSProvider_GetSecret_TokenRefresh(t *testing.T) {
	srv, setSecret, authCounter := newTestKMSServer(t)
	setSecret("REFRESH_KEY", "value")

	cfg := testKMSConfig(srv.URL)
	p, err := NewKMSProvider(cfg)
	require.NoError(t, err)

	ctx := context.Background()

	// First call authenticates.
	_, err = p.GetSecret(ctx, "REFRESH_KEY")
	require.NoError(t, err)
	require.Equal(t, int64(1), authCounter.Load())

	// Expire the token and clear the secret cache to force a re-fetch.
	p.mu.Lock()
	p.tokenExpiry = time.Now().Add(-1 * time.Minute)
	p.secretCache = make(map[string]cachedSecret)
	p.mu.Unlock()

	// Next call should re-authenticate because the token is expired.
	_, err = p.GetSecret(ctx, "REFRESH_KEY")
	require.NoError(t, err)
	require.Equal(t, int64(2), authCounter.Load())
}

func TestKMSProvider_SetSecret_Unsupported(t *testing.T) {
	srv, _, _ := newTestKMSServer(t)
	p, err := NewKMSProvider(testKMSConfig(srv.URL))
	require.NoError(t, err)

	err = p.SetSecret(context.Background(), "KEY", "VALUE")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not supported")
}

func TestKMSProvider_DeleteSecret_Unsupported(t *testing.T) {
	srv, _, _ := newTestKMSServer(t)
	p, err := NewKMSProvider(testKMSConfig(srv.URL))
	require.NoError(t, err)

	err = p.DeleteSecret(context.Background(), "KEY")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not supported")
}

func TestKMSProvider_AuthFailure(t *testing.T) {
	srv, _, _ := newTestKMSServer(t)
	cfg := testKMSConfig(srv.URL)
	cfg.ClientSecret = "wrong-secret"

	p, err := NewKMSProvider(cfg)
	require.NoError(t, err)

	_, err = p.GetSecret(context.Background(), "ANY_KEY")
	require.Error(t, err)
	require.Contains(t, err.Error(), "auth")
}

func TestKMSProvider_MissingClientID(t *testing.T) {
	_, err := NewKMSProvider(KMSConfig{
		ClientSecret: "secret",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "clientId is required")
}

func TestKMSProvider_MissingClientSecret(t *testing.T) {
	_, err := NewKMSProvider(KMSConfig{
		ClientID: "id",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "clientSecret is required")
}

func TestKMSProvider_DefaultSiteURL(t *testing.T) {
	p, err := NewKMSProvider(KMSConfig{
		ClientID:     "id",
		ClientSecret: "secret",
	})
	require.NoError(t, err)
	require.Equal(t, "https://kms.hanzo.ai", p.siteURL)
}

func TestKMSProvider_DefaultEnvironment(t *testing.T) {
	p, err := NewKMSProvider(KMSConfig{
		ClientID:     "id",
		ClientSecret: "secret",
	})
	require.NoError(t, err)
	require.Equal(t, "prod", p.environment)
}

func TestKMSProvider_DefaultSecretPath(t *testing.T) {
	p, err := NewKMSProvider(KMSConfig{
		ClientID:     "id",
		ClientSecret: "secret",
	})
	require.NoError(t, err)
	require.Equal(t, "/", p.secretPath)
}

func TestKMSProvider_DefaultCacheTTL(t *testing.T) {
	p, err := NewKMSProvider(KMSConfig{
		ClientID:     "id",
		ClientSecret: "secret",
	})
	require.NoError(t, err)
	require.Equal(t, defaultKMSCacheTTL, p.cacheTTL)
}

func TestKMSProvider_CustomCacheTTL(t *testing.T) {
	p, err := NewKMSProvider(KMSConfig{
		ClientID:     "id",
		ClientSecret: "secret",
		CacheTTLSec:  60,
	})
	require.NoError(t, err)
	require.Equal(t, 60*time.Second, p.cacheTTL)
}

func TestKMSProvider_ConcurrentGetSecret(t *testing.T) {
	srv, setSecret, _ := newTestKMSServer(t)
	for i := 0; i < 10; i++ {
		setSecret(normaliseKey("KEY_"+string(rune('A'+i))), "value")
	}

	p, err := NewKMSProvider(testKMSConfig(srv.URL))
	require.NoError(t, err)

	var wg sync.WaitGroup
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := "KEY_" + string(rune('A'+idx))
			_, getErr := p.GetSecret(ctx, key)
			require.NoError(t, getErr)
		}(i)
	}

	wg.Wait()
}

func TestKMSProvider_QueryParams(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/universal-auth/login" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(kmsAuthResponse{
				AccessToken: "tok",
				ExpiresIn:   7200,
			})
			return
		}
		capturedQuery = r.URL.RawQuery
		resp := kmsSecretResponse{}
		resp.Secret.SecretValue = "val"
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)

	cfg := KMSConfig{
		SiteURL:       srv.URL,
		ClientID:      "test-client-id",
		ClientSecret:  "test-client-secret",
		WorkspaceSlug: "my-project",
		Environment:   "staging",
		SecretPath:    "/app/secrets",
	}
	// Use the internal constructor to attach the test server's client transport.
	p, err := NewKMSProvider(cfg)
	require.NoError(t, err)

	_, err = p.GetSecret(context.Background(), "DB_URL")
	require.NoError(t, err)

	require.Contains(t, capturedQuery, "workspaceSlug=my-project")
	require.Contains(t, capturedQuery, "environment=staging")
	require.Contains(t, capturedQuery, "secretPath=%2Fapp%2Fsecrets")
}

func TestNewProvider_KMS(t *testing.T) {
	srv, _, _ := newTestKMSServer(t)

	cfg := Config{
		Provider: ProviderKMS,
		KMS: KMSConfig{
			SiteURL:       srv.URL,
			ClientID:      "test-client-id",
			ClientSecret:  "test-client-secret",
			WorkspaceSlug: "test-workspace",
			Environment:   "prod",
		},
	}

	p, err := NewProvider(cfg)
	require.NoError(t, err)
	require.NotNil(t, p)
	require.Equal(t, ProviderKMS, p.Type())
}

func TestNewProvider_KMS_MissingConfig(t *testing.T) {
	cfg := Config{
		Provider: ProviderKMS,
		KMS:      KMSConfig{},
	}

	_, err := NewProvider(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "clientId is required")
}

func TestKMSProvider_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/universal-auth/login" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(kmsAuthResponse{
				AccessToken: "tok",
				ExpiresIn:   7200,
			})
			return
		}
		http.Error(w, `{"message":"internal server error"}`, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	p, err := NewKMSProvider(KMSConfig{
		SiteURL:      srv.URL,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	})
	require.NoError(t, err)

	_, err = p.GetSecret(context.Background(), "ANY")
	require.Error(t, err)
	require.Contains(t, err.Error(), "status 500")
}

func TestKMSProvider_EmptyTokenResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(kmsAuthResponse{
			AccessToken: "",
			ExpiresIn:   7200,
		})
	}))
	t.Cleanup(srv.Close)

	p, err := NewKMSProvider(KMSConfig{
		SiteURL:      srv.URL,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	})
	require.NoError(t, err)

	_, err = p.GetSecret(context.Background(), "KEY")
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty access token")
}
