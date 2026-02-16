package bot

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverJSON(t *testing.T) {
	body := `{
		"discovered_at": "2025-01-01T00:00:00Z",
		"total_agents": 1,
		"total_bots": 1,
		"total_skills": 0,
		"pagination": {"limit": 50, "offset": 0, "has_more": false},
		"capabilities": [{
			"agent_id": "agent-1",
			"base_url": "http://agent",
			"version": "1.0.0",
			"health_status": "active",
			"deployment_type": "long_running",
			"last_heartbeat": "2025-01-01T00:00:00Z",
			"bots": [{"id": "deep_research", "invocation_target": "agent-1:deep_research"}],
			"skills": []
		}]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		assert.Equal(t, "agent-1", query.Get("agent"))
		assert.Equal(t, "true", query.Get("include_input_schema"))
		assert.Equal(t, "json", query.Get("format"))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
	defer server.Close()

	a, err := New(Config{
		NodeID:        "node-1",
		Version:       "1.0.0",
		PlaygroundURL: server.URL,
	})
	require.NoError(t, err)

	result, err := a.Discover(context.Background(), WithBot("agent-1"), WithDiscoveryInputSchema(true), WithLimit(50))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.JSON)

	assert.Equal(t, "json", result.Format)
	assert.Equal(t, 1, result.JSON.TotalAgents)
	assert.Equal(t, "agent-1", result.JSON.Capabilities[0].AgentID)
	assert.Equal(t, "agent-1:deep_research", result.JSON.Capabilities[0].Bots[0].InvocationTarget)
	assert.Equal(t, body, result.Raw)
}

func TestDiscoverCompactAndXML(t *testing.T) {
	compact := `{"discovered_at":"2025-01-01T00:00:00Z","bots":[{"id":"r1","agent_id":"a1","target":"a1:r1"}],"skills":[]}`
	xml := "<discovery><summary total_agents=\"0\" /></discovery>"

	compactServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "compact", r.URL.Query().Get("format"))
		fmt.Fprint(w, compact)
	}))
	defer compactServer.Close()

	xmlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "xml", r.URL.Query().Get("format"))
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, xml)
	}))
	defer xmlServer.Close()

	aCompact, err := New(Config{
		NodeID:        "node-1",
		Version:       "1.0.0",
		PlaygroundURL: compactServer.URL,
	})
	require.NoError(t, err)

	compactResult, err := aCompact.Discover(context.Background(), WithFormat("compact"))
	require.NoError(t, err)
	require.NotNil(t, compactResult.Compact)
	assert.Equal(t, "compact", compactResult.Format)
	assert.Equal(t, "a1", compactResult.Compact.Bots[0].AgentID)

	aXML, err := New(Config{
		NodeID:        "node-1",
		Version:       "1.0.0",
		PlaygroundURL: xmlServer.URL,
	})
	require.NoError(t, err)

	xmlResult, err := aXML.Discover(context.Background(), WithFormat("xml"), WithTags([]string{"ml"}))
	require.NoError(t, err)
	assert.Equal(t, "xml", xmlResult.Format)
	assert.Equal(t, xml, xmlResult.XML)
	assert.Nil(t, xmlResult.JSON)
	assert.Nil(t, xmlResult.Compact)
}

func TestDiscoverRejectsInvalidFormat(t *testing.T) {
	a, err := New(Config{
		NodeID:        "node-1",
		Version:       "1.0.0",
		PlaygroundURL: "http://localhost:8080",
	})
	require.NoError(t, err)

	_, err = a.Discover(context.Background(), WithFormat("yaml"))
	require.Error(t, err)
}

func TestDedupeHelper(t *testing.T) {
	values := []string{"a", "b", "a", "", "c"}
	assert.Equal(t, []string{"a", "b", "c"}, dedupe(values))
}
