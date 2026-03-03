package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockNodeLister implements NodeLister for tests.
type mockNodeLister struct {
	mu    sync.Mutex
	nodes []NodeInfo
}

func (m *mockNodeLister) RunningNodes() []NodeInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]NodeInfo, len(m.nodes))
	copy(result, m.nodes)
	return result
}

func (m *mockNodeLister) setNodes(nodes []NodeInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodes = nodes
}

func TestMeteringService_MetersRunningNodes(t *testing.T) {
	var mu sync.Mutex
	var usageRecords []map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/billing/usage" {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			usageRecords = append(usageRecords, body)
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	lister := &mockNodeLister{
		nodes: []NodeInfo{
			{
				NodeID:        "cloud-abc",
				BillingUserID: "hanzo/z",
				BearerToken:   "tok-1",
				CentsPerHour:  4,
				ProvisionedAt: time.Now().Add(-10 * time.Minute),
			},
		},
	}

	// Use a very short interval for testing
	svc := NewMeteringService(client, lister, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go svc.Run(ctx)

	// Wait for at least one meter cycle
	time.Sleep(200 * time.Millisecond)
	cancel()

	mu.Lock()
	defer mu.Unlock()
	require.GreaterOrEqual(t, len(usageRecords), 1, "expected at least one usage record")

	record := usageRecords[0]
	assert.Equal(t, "hanzo/z", record["user_id"])
	assert.Greater(t, record["cents_used"].(float64), float64(0))
	assert.Equal(t, "usd", record["currency"])

	meta := record["metadata"].(map[string]interface{})
	assert.Equal(t, "cloud-abc", meta["node_id"])
	assert.Equal(t, "compute", meta["type"])
}

func TestMeteringService_SkipsNodesWithoutBillingData(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	lister := &mockNodeLister{
		nodes: []NodeInfo{
			{NodeID: "no-billing", BillingUserID: "", CentsPerHour: 0},
			{NodeID: "no-rate", BillingUserID: "hanzo/z", CentsPerHour: 0},
			{NodeID: "no-user", BillingUserID: "", CentsPerHour: 4},
		},
	}

	svc := NewMeteringService(client, lister, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	go svc.Run(ctx)
	time.Sleep(200 * time.Millisecond)
	cancel()

	assert.Equal(t, 0, requestCount, "no usage should be recorded for incomplete billing data")
}

func TestMeteringService_PrunesRemovedNodes(t *testing.T) {
	var mu sync.Mutex
	var usageRecords []map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/billing/usage" {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			usageRecords = append(usageRecords, body)
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	lister := &mockNodeLister{
		nodes: []NodeInfo{
			{
				NodeID:        "node-1",
				BillingUserID: "hanzo/z",
				BearerToken:   "tok",
				CentsPerHour:  4,
				ProvisionedAt: time.Now().Add(-5 * time.Minute),
			},
		},
	}

	svc := NewMeteringService(client, lister, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	go svc.Run(ctx)

	// Wait long enough for at least one metering cycle to complete.
	time.Sleep(200 * time.Millisecond)

	// Remove the node
	lister.setNodes(nil)

	// Wait for any in-flight cycle to finish, then snapshot.
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	countAfterRemoval := len(usageRecords)
	mu.Unlock()

	// Wait for several more cycles — count should not increase.
	time.Sleep(200 * time.Millisecond)
	cancel()

	mu.Lock()
	defer mu.Unlock()
	// After removing nodes, the lastMetered map should be pruned
	// and no new records should be generated (count should be same)
	assert.Equal(t, countAfterRemoval, len(usageRecords),
		"no additional usage should be recorded after node removal")
}

func TestMeteringService_HandlesCommerceErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal"}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	lister := &mockNodeLister{
		nodes: []NodeInfo{
			{
				NodeID:        "node-err",
				BillingUserID: "hanzo/z",
				BearerToken:   "tok",
				CentsPerHour:  4,
				ProvisionedAt: time.Now().Add(-5 * time.Minute),
			},
		},
	}

	svc := NewMeteringService(client, lister, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	go svc.Run(ctx)

	// Should not panic on Commerce errors, just log them
	time.Sleep(200 * time.Millisecond)
	cancel()
}

func TestMeteringService_StopsOnContextCancel(t *testing.T) {
	client := newTestClient("http://127.0.0.1:1", "")
	lister := &mockNodeLister{}

	svc := NewMeteringService(client, lister, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		svc.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// OK, service stopped
	case <-time.After(2 * time.Second):
		t.Fatal("metering service did not stop after context cancel")
	}
}

func TestMeteringService_CalculatesCorrectCents(t *testing.T) {
	// Node running for exactly 1 hour at 4 cents/hr should report 4 cents
	var capturedCents float64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		capturedCents = body["cents_used"].(float64)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, "")
	lister := &mockNodeLister{
		nodes: []NodeInfo{
			{
				NodeID:        "node-calc",
				BillingUserID: "hanzo/z",
				BearerToken:   "tok",
				CentsPerHour:  4,
				ProvisionedAt: time.Now().Add(-1 * time.Hour),
			},
		},
	}

	svc := NewMeteringService(client, lister, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	go svc.Run(ctx)
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Should be at least 4 cents for 1 hour (may be slightly more due to elapsed time)
	assert.GreaterOrEqual(t, capturedCents, float64(4))
}
