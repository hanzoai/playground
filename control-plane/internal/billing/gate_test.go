package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestServer creates a mock Commerce balance API returning the given
// available cents. It also records the last request for header assertions.
func newTestServer(availableCents float64) (*httptest.Server, *http.Request) {
	var lastReq *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastReq = r
		json.NewEncoder(w).Encode(BalanceResult{
			User:      r.URL.Query().Get("user"),
			Currency:  "usd",
			Balance:   availableCents,
			Available: availableCents,
		})
	}))
	return srv, lastReq
}

func newTestClient(baseURL, serviceToken string) *Client {
	return &Client{
		baseURL:      baseURL,
		serviceToken: serviceToken,
		httpClient:   http.DefaultClient,
	}
}

// --- CentsPerHour tests ---

func TestCentsPerHour_KnownSlugs(t *testing.T) {
	tests := []struct {
		slug     string
		expected int
	}{
		{"s-1vcpu-2gb", 2},
		{"s-2vcpu-4gb", 4},
		{"s-4vcpu-8gb", 7},
		{"g-2vcpu-8gb", 7},
		{"s-8vcpu-16gb", 14},
	}
	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			assert.Equal(t, tt.expected, CentsPerHour(tt.slug))
		})
	}
}

func TestCentsPerHour_Presets(t *testing.T) {
	tests := []struct {
		preset   string
		expected int
	}{
		{"starter", 2},
		{"pro", 4},
		{"power", 7},
		{"gpu", 7},
	}
	for _, tt := range tests {
		t.Run(tt.preset, func(t *testing.T) {
			assert.Equal(t, tt.expected, CentsPerHour(tt.preset))
		})
	}
}

func TestCentsPerHour_UnknownSlug(t *testing.T) {
	assert.Equal(t, 4, CentsPerHour("unknown-slug"))
}

// --- CheckProvisionAllowance tests ---

func TestCheckProvisionAllowance_SufficientFunds(t *testing.T) {
	srv, _ := newTestServer(500) // $5.00
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	allow, err := CheckProvisionAllowance(context.Background(), client, "hanzo/z", "tok", 4, 1)
	require.NoError(t, err)

	assert.True(t, allow.Allowed)
	assert.Equal(t, 500, allow.BalanceCents)
	assert.Equal(t, 4, allow.RequiredCents)
	assert.Equal(t, 125, allow.HoursAfford)
	assert.Empty(t, allow.Reason)
}

func TestCheckProvisionAllowance_InsufficientFunds(t *testing.T) {
	srv, _ := newTestServer(3) // $0.03 — can't afford 1hr at $0.04/hr
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	allow, err := CheckProvisionAllowance(context.Background(), client, "hanzo/z", "tok", 4, 1)
	require.NoError(t, err)

	assert.False(t, allow.Allowed)
	assert.Equal(t, 3, allow.BalanceCents)
	assert.Equal(t, 4, allow.RequiredCents)
	assert.Equal(t, 0, allow.HoursAfford)
	assert.Contains(t, allow.Reason, "Insufficient funds")
	assert.Contains(t, allow.Reason, "billing.hanzo.ai")
}

func TestCheckProvisionAllowance_ZeroBalance(t *testing.T) {
	srv, _ := newTestServer(0)
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	allow, err := CheckProvisionAllowance(context.Background(), client, "hanzo/z", "tok", 4, 1)
	require.NoError(t, err)

	assert.False(t, allow.Allowed)
	assert.Equal(t, 0, allow.BalanceCents)
}

func TestCheckProvisionAllowance_ExactlyOneHour(t *testing.T) {
	srv, _ := newTestServer(4) // exactly 1 hour at pro tier
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	allow, err := CheckProvisionAllowance(context.Background(), client, "hanzo/z", "tok", 4, 1)
	require.NoError(t, err)

	assert.True(t, allow.Allowed)
	assert.Equal(t, 1, allow.HoursAfford)
}

func TestCheckProvisionAllowance_CommerceDown(t *testing.T) {
	// Unreachable server
	client := newTestClient("http://127.0.0.1:1", "")
	_, err := CheckProvisionAllowance(context.Background(), client, "hanzo/z", "tok", 4, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "billing gate")
}

// --- CentsPerHourVM tests ---

func TestCentsPerHourVM_MacInstances(t *testing.T) {
	tests := []struct {
		instanceType string
		expected     int
	}{
		{"mac2.metal", 65},
		{"mac2-m2.metal", 65},
		{"mac-m4.metal", 123},
		{"mac-m4pro.metal", 123},
		{"mac2-m1ultra.metal", 500},
	}
	for _, tt := range tests {
		t.Run(tt.instanceType, func(t *testing.T) {
			assert.Equal(t, tt.expected, CentsPerHourVM(tt.instanceType))
		})
	}
}

func TestCentsPerHourVM_NonMacInstances(t *testing.T) {
	assert.Equal(t, 4, CentsPerHourVM("t3.medium"))
	assert.Equal(t, 8, CentsPerHourVM("t3.large"))
	assert.Equal(t, 4, CentsPerHourVM("unknown-instance"))
}

// --- MinimumHours tests ---

func TestMinimumHours_MacIs24(t *testing.T) {
	macTypes := []string{
		"mac2.metal", "mac2-m1ultra.metal", "mac2-m2.metal",
		"mac2-m2pro.metal", "mac-m4.metal", "mac-m4pro.metal",
	}
	for _, mt := range macTypes {
		t.Run(mt, func(t *testing.T) {
			assert.Equal(t, 24, MinimumHours(mt), "Mac instances require 24h minimum (Apple licensing)")
		})
	}
}

func TestMinimumHours_NonMacIs1(t *testing.T) {
	assert.Equal(t, 1, MinimumHours("t3.medium"))
	assert.Equal(t, 1, MinimumHours("s-2vcpu-4gb"))
	assert.Equal(t, 1, MinimumHours("unknown"))
}

// --- Mac 24h billing tests ---

func TestCheckProvisionAllowance_Mac24hMinimum_Denied(t *testing.T) {
	// $15.00 balance, Mac needs $15.60 (24h × $0.65/hr)
	srv, _ := newTestServer(1500)
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	allow, err := CheckProvisionAllowance(context.Background(), client, "hanzo/z", "tok", 65, 24)
	require.NoError(t, err)

	assert.False(t, allow.Allowed)
	assert.Equal(t, 1560, allow.RequiredCents) // 65 × 24
	assert.Contains(t, allow.Reason, "24 hours")
	assert.Contains(t, allow.Reason, "Apple licensing")
}

func TestCheckProvisionAllowance_Mac24hSufficient(t *testing.T) {
	// $16.00 balance, Mac needs $15.60 (24h × $0.65/hr) — sufficient
	srv, _ := newTestServer(1600)
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	allow, err := CheckProvisionAllowance(context.Background(), client, "hanzo/z", "tok", 65, 24)
	require.NoError(t, err)

	assert.True(t, allow.Allowed)
	assert.Equal(t, 1560, allow.RequiredCents)
	assert.Equal(t, 24, allow.HoursAfford) // 1600/65 = 24
}

// --- Client case normalization tests ---

func TestGetBalance_NormalizesUserID(t *testing.T) {
	var capturedUser string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUser = r.URL.Query().Get("user")
		json.NewEncoder(w).Encode(BalanceResult{
			User:      capturedUser,
			Currency:  "usd",
			Available: 100,
		})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	_, err := client.GetBalance(context.Background(), "hanzo/Z", "tok")
	require.NoError(t, err)
	assert.Equal(t, "hanzo/z", capturedUser)
}

func TestGetBalance_MixedCaseNormalized(t *testing.T) {
	var capturedUser string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUser = r.URL.Query().Get("user")
		json.NewEncoder(w).Encode(BalanceResult{Available: 100})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "")

	tests := []struct {
		input    string
		expected string
	}{
		{"hanzo/A", "hanzo/a"},
		{"Hanzo/Z", "hanzo/z"},
		{"HANZO/USER", "hanzo/user"},
		{"hanzo/z", "hanzo/z"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := client.GetBalance(context.Background(), tt.input, "tok")
			require.NoError(t, err)
			assert.Equal(t, tt.expected, capturedUser)
		})
	}
}

func TestGetBalance_ServiceTokenSetsOrgHeader(t *testing.T) {
	var capturedOrgHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedOrgHeader = r.Header.Get("X-Hanzo-Org")
		json.NewEncoder(w).Encode(BalanceResult{Available: 100})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "svc-token-123")
	_, err := client.GetBalance(context.Background(), "hanzo/z", "user-tok")
	require.NoError(t, err)
	assert.Equal(t, "hanzo", capturedOrgHeader)
}

func TestGetBalance_ServiceTokenPreferred(t *testing.T) {
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		json.NewEncoder(w).Encode(BalanceResult{Available: 100})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "svc-token")
	_, err := client.GetBalance(context.Background(), "hanzo/z", "user-token")
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(capturedAuth, "svc-token"))
}

func TestGetBalance_FallsBackToUserToken(t *testing.T) {
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		json.NewEncoder(w).Encode(BalanceResult{Available: 100})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	_, err := client.GetBalance(context.Background(), "hanzo/z", "user-token")
	require.NoError(t, err)
	assert.Equal(t, "Bearer user-token", capturedAuth)
}
