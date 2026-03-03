package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- CreateHold tests ---

func TestCreateHold_Success(t *testing.T) {
	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/billing/holds", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		json.NewDecoder(r.Body).Decode(&capturedBody)

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Hold{
			ID:          "hold-abc123",
			UserID:      "hanzo/z",
			AmountCents: 400,
			Status:      "pending",
			CreatedAt:   time.Now(),
		})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	hold, err := client.CreateHold(context.Background(), "hanzo/Z", "tok", 400, "Cloud compute: test-node")
	require.NoError(t, err)

	assert.Equal(t, "hold-abc123", hold.ID)
	assert.Equal(t, "hanzo/z", hold.UserID)
	assert.Equal(t, 400, hold.AmountCents)
	assert.Equal(t, "pending", hold.Status)

	// Verify request body was sent correctly
	assert.Equal(t, "hanzo/z", capturedBody["user_id"])
	assert.Equal(t, float64(400), capturedBody["amount_cents"])
	assert.Equal(t, "usd", capturedBody["currency"])
	assert.Equal(t, "Cloud compute: test-node", capturedBody["description"])
}

func TestCreateHold_NormalizesUserID(t *testing.T) {
	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Hold{ID: "hold-1"})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	_, err := client.CreateHold(context.Background(), "Hanzo/A", "tok", 100, "test")
	require.NoError(t, err)
	assert.Equal(t, "hanzo/a", capturedBody["user_id"])
}

func TestCreateHold_UsesServiceToken(t *testing.T) {
	var capturedAuth string
	var capturedOrg string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		capturedOrg = r.Header.Get("X-Hanzo-Org")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Hold{ID: "hold-svc"})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "svc-token-123")
	_, err := client.CreateHold(context.Background(), "hanzo/z", "user-token", 200, "test")
	require.NoError(t, err)
	assert.Equal(t, "Bearer svc-token-123", capturedAuth)
	assert.Equal(t, "hanzo", capturedOrg)
}

func TestCreateHold_CommerceError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		w.Write([]byte(`{"error": "insufficient_funds"}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	_, err := client.CreateHold(context.Background(), "hanzo/z", "tok", 400, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 402")
}

func TestCreateHold_NetworkError(t *testing.T) {
	client := newTestClient("http://127.0.0.1:1", "")
	_, err := client.CreateHold(context.Background(), "hanzo/z", "tok", 100, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "billing: hold request failed")
}

// --- SettleHold tests ---

func TestSettleHold_Success(t *testing.T) {
	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/billing/holds/hold-abc123/settle", r.URL.Path)

		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	err := client.SettleHold(context.Background(), "hold-abc123", "tok", 350)
	require.NoError(t, err)
	assert.Equal(t, float64(350), capturedBody["actual_cents"])
}

func TestSettleHold_PartialUsage(t *testing.T) {
	// User was charged for 1 hour but used only 30 minutes worth
	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	err := client.SettleHold(context.Background(), "hold-xyz", "tok", 2)
	require.NoError(t, err)
	assert.Equal(t, float64(2), capturedBody["actual_cents"])
}

func TestSettleHold_CommerceError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "hold_not_found"}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	err := client.SettleHold(context.Background(), "nonexistent", "tok", 100)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 404")
}

func TestSettleHold_NetworkError(t *testing.T) {
	client := newTestClient("http://127.0.0.1:1", "")
	err := client.SettleHold(context.Background(), "hold-1", "tok", 100)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "billing: settle request failed")
}

// --- RecordUsage tests ---

func TestRecordUsage_Success(t *testing.T) {
	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/billing/usage", r.URL.Path)

		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	metadata := map[string]string{"node_id": "cloud-abc", "type": "compute"}
	err := client.RecordUsage(context.Background(), "hanzo/Z", "tok", 12, metadata)
	require.NoError(t, err)
	assert.Equal(t, "hanzo/z", capturedBody["user_id"])
	assert.Equal(t, float64(12), capturedBody["cents_used"])
	assert.Equal(t, "usd", capturedBody["currency"])

	meta := capturedBody["metadata"].(map[string]interface{})
	assert.Equal(t, "cloud-abc", meta["node_id"])
	assert.Equal(t, "compute", meta["type"])
}

func TestRecordUsage_NoMetadata(t *testing.T) {
	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	err := client.RecordUsage(context.Background(), "hanzo/z", "tok", 5, nil)
	require.NoError(t, err)
	assert.Nil(t, capturedBody["metadata"])
}

func TestRecordUsage_CommerceError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal_error"}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	err := client.RecordUsage(context.Background(), "hanzo/z", "tok", 10, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
}

func TestRecordUsage_NetworkError(t *testing.T) {
	client := newTestClient("http://127.0.0.1:1", "")
	err := client.RecordUsage(context.Background(), "hanzo/z", "tok", 10, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "billing: usage request failed")
}
