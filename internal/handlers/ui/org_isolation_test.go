package ui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/internal/server/middleware"
	"github.com/hanzoai/playground/internal/storage"
	"github.com/hanzoai/playground/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// orgIsolationStorage initializes a real LocalStorage backed by a temp SQLite
// database. This gives us actual org-scoped query behavior instead of mocks.
func orgIsolationStorage(t *testing.T) storage.StorageProvider {
	t.Helper()
	ctx := context.Background()
	tmpDir := t.TempDir()
	cfg := storage.StorageConfig{
		Mode: "local",
		Local: storage.LocalStorageConfig{
			DatabasePath: tmpDir + "/isolation.db",
			KVStorePath:  tmpDir + "/isolation.bolt",
		},
	}

	s := storage.NewLocalStorage(storage.LocalStorageConfig{})
	err := s.Initialize(ctx, cfg)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "fts5") {
		t.Skip("sqlite3 compiled without FTS5")
	}
	require.NoError(t, err)
	t.Cleanup(func() { s.Close(ctx) })
	return s
}

// setOrgContext injects IAM org context into a gin.Context the same way the
// IAM middleware does in production. Handlers read it via middleware.RequireOrg.
func setOrgContext(c *gin.Context, org string) {
	c.Set(middleware.ContextKeyOrg, org)
	c.Set(middleware.ContextKeyUser, &middleware.IAMUserInfo{
		Sub:          "test-user",
		Email:        "test@example.com",
		Organization: org,
	})
}

// orgRouter builds a gin.Engine with a middleware that sets the org from a
// custom X-Test-Org header. This lets each test request impersonate a different
// org without standing up a real IAM server.
func orgRouter(botsHandler *BotsHandler, activityHandler *RecentActivityHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.Use(func(c *gin.Context) {
		org := c.GetHeader("X-Test-Org")
		if org != "" {
			setOrgContext(c, org)
		}
		c.Next()
	})

	r.GET("/api/v1/bots", botsHandler.GetAllBotsHandler)
	r.GET("/api/v1/executions/recent", activityHandler.GetRecentActivityHandler)
	return r
}

// seedTwoOrgs registers nodes and execution records belonging to two distinct orgs.
func seedTwoOrgs(t *testing.T, s storage.StorageProvider) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()

	// Org A node with one bot
	nodeA := &types.Node{
		ID:             "node-org-a",
		OrgID:          "org-a",
		TeamID:         "team-a",
		BaseURL:        "http://node-a:8080",
		Version:        "1.0.0",
		DeploymentType: "long_running",
		HealthStatus:   types.HealthStatusActive,
		LastHeartbeat:  now,
		RegisteredAt:   now,
		Bots: []types.BotDefinition{
			{ID: "bot-alpha", Tags: []string{"ml"}},
		},
	}
	require.NoError(t, s.RegisterNode(ctx, nodeA))

	// Org B node with one bot
	nodeB := &types.Node{
		ID:             "node-org-b",
		OrgID:          "org-b",
		TeamID:         "team-b",
		BaseURL:        "http://node-b:8080",
		Version:        "2.0.0",
		DeploymentType: "long_running",
		HealthStatus:   types.HealthStatusActive,
		LastHeartbeat:  now,
		RegisteredAt:   now,
		Bots: []types.BotDefinition{
			{ID: "bot-beta", Tags: []string{"nlp"}},
		},
	}
	require.NoError(t, s.RegisterNode(ctx, nodeB))

	// Execution record for Org A
	execA := &types.Execution{
		OrgID:       "org-a",
		ExecutionID: "exec-a-001",
		RunID:       "run-a-001",
		NodeID:      "node-org-a",
		BotID:       "bot-alpha",
		Status:      string(types.ExecutionStatusSucceeded),
		StartedAt:   now.Add(-5 * time.Minute),
	}
	require.NoError(t, s.CreateExecutionRecord(ctx, execA))

	// Execution record for Org B
	execB := &types.Execution{
		OrgID:       "org-b",
		ExecutionID: "exec-b-001",
		RunID:       "run-b-001",
		NodeID:      "node-org-b",
		BotID:       "bot-beta",
		Status:      string(types.ExecutionStatusSucceeded),
		StartedAt:   now.Add(-3 * time.Minute),
	}
	require.NoError(t, s.CreateExecutionRecord(ctx, execB))
}

// ---------------------------------------------------------------------------
// Cross-tenant isolation: Bots
// ---------------------------------------------------------------------------

func TestGetAllBots_OrgA_CannotSeeOrgB(t *testing.T) {
	s := orgIsolationStorage(t)
	seedTwoOrgs(t, s)

	botsHandler := NewBotsHandler(s)
	activityHandler := NewRecentActivityHandler(s)
	router := orgRouter(botsHandler, activityHandler)

	// Request as org-a
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bots", nil)
	req.Header.Set("X-Test-Org", "org-a")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp BotsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	// Should see only org-a's bot
	assert.Equal(t, 1, resp.Total, "org-a should see exactly 1 bot")
	if assert.Len(t, resp.Bots, 1) {
		assert.Equal(t, "node-org-a.bot-alpha", resp.Bots[0].BotID)
		assert.Equal(t, "node-org-a", resp.Bots[0].NodeID)
	}
}

func TestGetAllBots_OrgB_CannotSeeOrgA(t *testing.T) {
	s := orgIsolationStorage(t)
	seedTwoOrgs(t, s)

	botsHandler := NewBotsHandler(s)
	activityHandler := NewRecentActivityHandler(s)
	router := orgRouter(botsHandler, activityHandler)

	// Request as org-b
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bots", nil)
	req.Header.Set("X-Test-Org", "org-b")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp BotsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	// Should see only org-b's bot
	assert.Equal(t, 1, resp.Total, "org-b should see exactly 1 bot")
	if assert.Len(t, resp.Bots, 1) {
		assert.Equal(t, "node-org-b.bot-beta", resp.Bots[0].BotID)
		assert.Equal(t, "node-org-b", resp.Bots[0].NodeID)
	}
}

func TestGetAllBots_EmptyOrg_ReturnsEmptyNotError(t *testing.T) {
	s := orgIsolationStorage(t)
	seedTwoOrgs(t, s)

	botsHandler := NewBotsHandler(s)
	activityHandler := NewRecentActivityHandler(s)
	router := orgRouter(botsHandler, activityHandler)

	// Request as org-c which has no data
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bots", nil)
	req.Header.Set("X-Test-Org", "org-c")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp BotsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	assert.Equal(t, 0, resp.Total, "org with no data should see 0 bots")
	assert.Empty(t, resp.Bots, "bots list should be empty, not nil")
}

// ---------------------------------------------------------------------------
// Cross-tenant isolation: Recent Activity
// ---------------------------------------------------------------------------

func TestGetRecentActivity_OrgA_CannotSeeOrgB(t *testing.T) {
	s := orgIsolationStorage(t)
	seedTwoOrgs(t, s)

	botsHandler := NewBotsHandler(s)
	activityHandler := NewRecentActivityHandler(s)
	router := orgRouter(botsHandler, activityHandler)

	// Request as org-a
	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/recent", nil)
	req.Header.Set("X-Test-Org", "org-a")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp RecentActivityResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	// Should see only org-a's execution
	require.Len(t, resp.Executions, 1, "org-a should see exactly 1 execution")
	assert.Equal(t, "exec-a-001", resp.Executions[0].ExecutionID)
	assert.Equal(t, "node-org-a", resp.Executions[0].AgentName)
}

func TestGetRecentActivity_OrgB_CannotSeeOrgA(t *testing.T) {
	s := orgIsolationStorage(t)
	seedTwoOrgs(t, s)

	botsHandler := NewBotsHandler(s)
	activityHandler := NewRecentActivityHandler(s)
	router := orgRouter(botsHandler, activityHandler)

	// Request as org-b
	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/recent", nil)
	req.Header.Set("X-Test-Org", "org-b")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp RecentActivityResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	// Should see only org-b's execution
	require.Len(t, resp.Executions, 1, "org-b should see exactly 1 execution")
	assert.Equal(t, "exec-b-001", resp.Executions[0].ExecutionID)
	assert.Equal(t, "node-org-b", resp.Executions[0].AgentName)
}

func TestGetRecentActivity_EmptyOrg_ReturnsEmptyNotError(t *testing.T) {
	s := orgIsolationStorage(t)
	seedTwoOrgs(t, s)

	botsHandler := NewBotsHandler(s)
	activityHandler := NewRecentActivityHandler(s)
	router := orgRouter(botsHandler, activityHandler)

	// Request as org-c which has no data
	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/recent", nil)
	req.Header.Set("X-Test-Org", "org-c")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp RecentActivityResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	assert.Empty(t, resp.Executions, "org with no executions should get empty list")
}

// ---------------------------------------------------------------------------
// Missing org context: RequireOrg returns 403 — no silent fallback
// ---------------------------------------------------------------------------

func TestGetAllBots_NoOrgHeader_Returns403(t *testing.T) {
	s := orgIsolationStorage(t)
	seedTwoOrgs(t, s)

	botsHandler := NewBotsHandler(s)
	activityHandler := NewRecentActivityHandler(s)
	router := orgRouter(botsHandler, activityHandler)

	// Request WITHOUT org header — no IAM context means no org.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bots", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code, "missing org must return 403, not silent fallback")
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "org_required", resp["error"])
}

func TestGetRecentActivity_NoOrgHeader_Returns403(t *testing.T) {
	s := orgIsolationStorage(t)
	seedTwoOrgs(t, s)

	botsHandler := NewBotsHandler(s)
	activityHandler := NewRecentActivityHandler(s)
	router := orgRouter(botsHandler, activityHandler)

	// Request WITHOUT org header
	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/recent", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code, "missing org must return 403, not silent fallback")
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "org_required", resp["error"])
}

// ---------------------------------------------------------------------------
// RequireIAMOrg: strict mode returns 403 when no IAM user is present
// ---------------------------------------------------------------------------

func TestRequireIAMOrg_NoUser_Returns403(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/strict", func(c *gin.Context) {
		org, ok := middleware.RequireIAMOrg(c)
		if !ok {
			return // already aborted with 403
		}
		c.JSON(http.StatusOK, gin.H{"org": org})
	})

	req := httptest.NewRequest(http.MethodGet, "/strict", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "org_required", resp["error"])
}

func TestRequireIAMOrg_EmptyOrg_Returns403(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/strict", func(c *gin.Context) {
		// Set IAM user but with empty org
		c.Set(middleware.ContextKeyUser, &middleware.IAMUserInfo{
			Sub:          "user-1",
			Email:        "user@test.com",
			Organization: "",
		})
		org, ok := middleware.RequireIAMOrg(c)
		if !ok {
			return
		}
		c.JSON(http.StatusOK, gin.H{"org": org})
	})

	req := httptest.NewRequest(http.MethodGet, "/strict", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "org_required", resp["error"])
}

// ---------------------------------------------------------------------------
// Bot execution handler: node org check (bots.go line 118)
// ---------------------------------------------------------------------------

func TestExecuteBot_CrossOrgNode_ReturnsForbidden(t *testing.T) {
	s := orgIsolationStorage(t)
	seedTwoOrgs(t, s)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		org := c.GetHeader("X-Test-Org")
		if org != "" {
			setOrgContext(c, org)
		}
		c.Next()
	})

	botsHandler := NewBotsHandler(s)
	r.GET("/api/v1/bots/:botId", botsHandler.GetBotDetailsHandler)

	// org-a trying to access org-b's bot
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bots/node-org-b.bot-beta", nil)
	req.Header.Set("X-Test-Org", "org-a")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code, "org-a must not access org-b's bot details")
}

func TestExecuteBot_SameOrgNode_Allowed(t *testing.T) {
	s := orgIsolationStorage(t)
	seedTwoOrgs(t, s)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		org := c.GetHeader("X-Test-Org")
		if org != "" {
			setOrgContext(c, org)
		}
		c.Next()
	})

	botsHandler := NewBotsHandler(s)
	r.GET("/api/v1/bots/:botId", botsHandler.GetBotDetailsHandler)

	// org-a accessing its own bot
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bots/node-org-a.bot-alpha", nil)
	req.Header.Set("X-Test-Org", "org-a")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code, "org-a should access its own bot")
}

// ---------------------------------------------------------------------------
// Symmetry check: ensure both orgs see only their own data in a single test
// ---------------------------------------------------------------------------

func TestCrossTenantIsolation_Symmetric(t *testing.T) {
	s := orgIsolationStorage(t)
	seedTwoOrgs(t, s)

	botsHandler := NewBotsHandler(s)
	activityHandler := NewRecentActivityHandler(s)
	router := orgRouter(botsHandler, activityHandler)

	for _, tc := range []struct {
		org          string
		expectedBot  string
		expectedExec string
	}{
		{"org-a", "node-org-a.bot-alpha", "exec-a-001"},
		{"org-b", "node-org-b.bot-beta", "exec-b-001"},
	} {
		t.Run("bots/"+tc.org, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/bots", nil)
			req.Header.Set("X-Test-Org", tc.org)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)
			var resp BotsResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			require.Equal(t, 1, resp.Total)
			assert.Equal(t, tc.expectedBot, resp.Bots[0].BotID)
		})

		t.Run("activity/"+tc.org, func(t *testing.T) {
			// Invalidate cache between orgs to avoid stale hits
			activityHandler.cache = NewRecentActivityCache()

			req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/recent", nil)
			req.Header.Set("X-Test-Org", tc.org)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)
			var resp RecentActivityResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			require.Len(t, resp.Executions, 1)
			assert.Equal(t, tc.expectedExec, resp.Executions[0].ExecutionID)
		})
	}
}
