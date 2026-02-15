package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/control-plane/internal/storage"
	"github.com/hanzoai/playground/control-plane/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestAdminRESTListReasoners(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tempDir := t.TempDir()
	cfg := storage.StorageConfig{
		Mode: "local",
		Local: storage.LocalStorageConfig{
			DatabasePath: filepath.Join(tempDir, "agents.db"),
			KVStorePath:  filepath.Join(tempDir, "agents.bolt"),
		},
	}

	localStore := storage.NewLocalStorage(storage.LocalStorageConfig{})
	if err := localStore.Initialize(ctx, cfg); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "fts5") {
			t.Skip("sqlite3 compiled without FTS5; skipping reasoner aggregation test")
		}
		require.NoError(t, err)
	}
	t.Cleanup(func() { _ = localStore.Close(ctx) })

	schema := json.RawMessage("{}")
	node := &types.AgentNode{
		ID:            "node-1",
		HealthStatus:  types.HealthStatusActive,
		Version:       "1.0.0",
		LastHeartbeat: time.Now().UTC(),
		Reasoners: []types.ReasonerDefinition{
			{ID: "reason", InputSchema: schema, OutputSchema: schema},
			{ID: "another", InputSchema: schema, OutputSchema: schema},
		},
	}
	require.NoError(t, localStore.RegisterAgent(ctx, node))

	// Set up Gin router with admin REST routes
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerAdminRESTRoutes(router, localStore)

	// Make request
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/reasoners", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Reasoners []struct {
			ReasonerID  string `json:"reasoner_id"`
			AgentNodeID string `json:"agent_node_id"`
			LastHB      string `json:"last_heartbeat"`
		} `json:"reasoners"`
		Count int `json:"count"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Equal(t, 2, resp.Count)
	require.Len(t, resp.Reasoners, 2)
	require.Equal(t, "node-1.reason", resp.Reasoners[0].ReasonerID)
	require.Equal(t, "node-1", resp.Reasoners[0].AgentNodeID)
	require.NotEmpty(t, resp.Reasoners[0].LastHB)
}
