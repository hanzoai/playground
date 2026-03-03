package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// --- checkLLMBalance unit tests ---

func TestCheckLLMBalance_SufficientFunds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(BalanceResult{
			User:      r.URL.Query().Get("user"),
			Available: 1000,
			Currency:  "usd",
		})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	cache := newBalanceCache(1 * time.Minute)

	result, err := checkLLMBalance(context.Background(), client, cache, "hanzo/z", "tok")
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, float64(1000), result.BalanceCents)
	assert.Equal(t, "hanzo/z", result.UserID)
}

func TestCheckLLMBalance_ZeroBalance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(BalanceResult{Available: 0})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	cache := newBalanceCache(1 * time.Minute)

	result, err := checkLLMBalance(context.Background(), client, cache, "hanzo/z", "tok")
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Contains(t, result.Reason, "Insufficient funds")
	assert.Contains(t, result.Reason, "billing.hanzo.ai")
}

func TestCheckLLMBalance_NegativeBalance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(BalanceResult{Available: -50})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	cache := newBalanceCache(1 * time.Minute)

	result, err := checkLLMBalance(context.Background(), client, cache, "hanzo/z", "tok")
	require.NoError(t, err)
	assert.False(t, result.Allowed)
}

func TestCheckLLMBalance_CommerceDown(t *testing.T) {
	client := newTestClient("http://127.0.0.1:1", "")
	cache := newBalanceCache(1 * time.Minute)

	_, err := checkLLMBalance(context.Background(), client, cache, "hanzo/z", "tok")
	assert.Error(t, err)
}

func TestCheckLLMBalance_UsesCacheOnSecondCall(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(BalanceResult{Available: 500})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	cache := newBalanceCache(1 * time.Minute)

	// First call — hits Commerce.
	result, err := checkLLMBalance(context.Background(), client, cache, "hanzo/z", "tok")
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, 1, callCount)

	// Second call — should use cache (context not needed since cache hit).
	result, err = checkLLMBalance(context.Background(), client, cache, "hanzo/z", "tok")
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, 1, callCount, "second call should not hit Commerce")
}

func TestCheckLLMBalance_CacheExpiryRefetches(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(BalanceResult{Available: 500})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	cache := newBalanceCache(10 * time.Millisecond) // very short TTL

	_, err := checkLLMBalance(context.Background(), client, cache, "hanzo/z", "tok")
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	time.Sleep(20 * time.Millisecond) // let cache expire

	_, err = checkLLMBalance(context.Background(), client, cache, "hanzo/z", "tok")
	require.NoError(t, err)
	assert.Equal(t, 2, callCount, "should re-fetch after cache expiry")
}

// --- GateMode resolution tests ---

func TestResolveGateMode_Defaults(t *testing.T) {
	tests := []struct {
		envVal   string
		expected GateMode
	}{
		{"open", GateModeOpen},
		{"warn", GateModeWarn},
		{"fail-closed", GateModeFailClosed},
		{"fail_closed", GateModeFailClosed},
		{"closed", GateModeFailClosed},
		{"", GateModeFailClosed},
		{"unknown", GateModeFailClosed},
	}
	for _, tt := range tests {
		t.Run(tt.envVal, func(t *testing.T) {
			t.Setenv("BILLING_GATE_MODE", tt.envVal)
			assert.Equal(t, tt.expected, resolveGateMode())
		})
	}
}

// --- GetLLMGateResult tests ---

func TestGetLLMGateResult_NotSet(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	assert.Nil(t, GetLLMGateResult(c))
}

func TestGetLLMGateResult_Set(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	expected := &LLMGateResult{Allowed: true, UserID: "hanzo/z"}
	c.Set(GateContextKey, expected)

	got := GetLLMGateResult(c)
	assert.Equal(t, expected, got)
}

// --- extractBearerToken tests ---

func TestExtractBearerToken(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Request = httptest.NewRequest(http.MethodPost, "/test", nil)
	c.Request.Header.Set("Authorization", "Bearer my-jwt-token")

	assert.Equal(t, "my-jwt-token", extractBearerToken(c))
}

func TestExtractBearerToken_NoHeader(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/test", nil)

	assert.Equal(t, "", extractBearerToken(c))
}

func TestExtractBearerToken_NonBearerScheme(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/test", nil)
	c.Request.Header.Set("Authorization", "Basic abc123")

	assert.Equal(t, "", extractBearerToken(c))
}
