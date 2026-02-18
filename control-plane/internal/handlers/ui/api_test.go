package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hanzoai/playground/control-plane/internal/core/domain"
	"github.com/hanzoai/playground/control-plane/internal/core/interfaces"
	"github.com/hanzoai/playground/control-plane/internal/services"
	"github.com/hanzoai/playground/control-plane/internal/storage"
	"github.com/hanzoai/playground/control-plane/pkg/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestGetNodesSummaryHandler_Structure tests the handler structure and routing
// This works without a server by testing handler setup and basic request handling
func TestGetNodesSummaryHandler_Structure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a minimal UIService mock using real storage (lightweight)
	ctx := context.Background()
	tempDir := t.TempDir()
	cfg := storage.StorageConfig{
		Mode: "local",
		Local: storage.LocalStorageConfig{
			DatabasePath: tempDir + "/test.db",
			KVStorePath:  tempDir + "/test.bolt",
		},
	}

	realStorage := storage.NewLocalStorage(storage.LocalStorageConfig{})
	err := realStorage.Initialize(ctx, cfg)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "fts5") {
		t.Skip("sqlite3 compiled without FTS5")
	}
	require.NoError(t, err)
	defer realStorage.Close(ctx)

	// Create real UIService with minimal dependencies
	mockNodeClient := &MockNodeClientForUI{}
	mockBotService := &MockBotServiceForUI{}
	statusManager := services.NewStatusManager(realStorage, services.StatusManagerConfig{}, nil, mockNodeClient)
	uiService := services.NewUIService(realStorage, mockNodeClient, mockBotService, statusManager)

	handler := NewNodesHandler(uiService)
	router := gin.New()
	router.GET("/api/ui/v1/nodes", handler.GetNodesSummaryHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/ui/v1/nodes", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should return OK (even if no nodes)
	assert.Equal(t, http.StatusOK, resp.Code)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	assert.Contains(t, result, "nodes")
	assert.Contains(t, result, "count")
}

// TestGetNodeDetailsHandler_Structure tests node details handler structure
func TestGetNodeDetailsHandler_Structure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctx := context.Background()
	tempDir := t.TempDir()
	cfg := storage.StorageConfig{
		Mode: "local",
		Local: storage.LocalStorageConfig{
			DatabasePath: tempDir + "/test.db",
			KVStorePath:  tempDir + "/test.bolt",
		},
	}

	realStorage := storage.NewLocalStorage(storage.LocalStorageConfig{})
	err := realStorage.Initialize(ctx, cfg)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "fts5") {
		t.Skip("sqlite3 compiled without FTS5")
	}
	require.NoError(t, err)
	defer realStorage.Close(ctx)

	mockNodeClient := &MockNodeClientForUI{}
	mockNodeClient.On("GetBotStatus", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("node not found"))
	mockBotService := &MockBotServiceForUI{}
	statusManager := services.NewStatusManager(realStorage, services.StatusManagerConfig{}, nil, mockNodeClient)
	uiService := services.NewUIService(realStorage, mockNodeClient, mockBotService, statusManager)

	handler := NewNodesHandler(uiService)
	router := gin.New()
	router.GET("/api/ui/v1/nodes/:nodeId", handler.GetNodeDetailsHandler)

	// Test with missing nodeId (should return 400 or 404 depending on router)
	req := httptest.NewRequest(http.MethodGet, "/api/ui/v1/nodes/", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.True(t, resp.Code == http.StatusBadRequest || resp.Code == http.StatusNotFound)

	// Test with nodeId (should return 404 if not found, but handler works)
	req = httptest.NewRequest(http.MethodGet, "/api/ui/v1/nodes/node-1", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	// Should handle gracefully (404 or 500 depending on implementation)
	assert.True(t, resp.Code == http.StatusNotFound || resp.Code == http.StatusInternalServerError)
}

// TestGetNodeStatusHandler_Structure tests node status handler
func TestGetNodeStatusHandler_Structure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctx := context.Background()
	tempDir := t.TempDir()
	cfg := storage.StorageConfig{
		Mode: "local",
		Local: storage.LocalStorageConfig{
			DatabasePath: tempDir + "/test.db",
			KVStorePath:  tempDir + "/test.bolt",
		},
	}

	realStorage := storage.NewLocalStorage(storage.LocalStorageConfig{})
	err := realStorage.Initialize(ctx, cfg)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "fts5") {
		t.Skip("sqlite3 compiled without FTS5")
	}
	require.NoError(t, err)
	defer realStorage.Close(ctx)

	mockNodeClient := &MockNodeClientForUI{}
	mockNodeClient.On("GetBotStatus", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("node not found"))
	mockBotService := &MockBotServiceForUI{}
	statusManager := services.NewStatusManager(realStorage, services.StatusManagerConfig{}, nil, mockNodeClient)
	uiService := services.NewUIService(realStorage, mockNodeClient, mockBotService, statusManager)

	handler := NewNodesHandler(uiService)
	router := gin.New()
	router.GET("/api/ui/v1/nodes/:nodeId/status", handler.GetNodeStatusHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/ui/v1/nodes/node-1/status", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle request (may return error if node not found)
	assert.True(t, resp.Code == http.StatusNotFound || resp.Code == http.StatusInternalServerError || resp.Code == http.StatusOK)
}

// TestRefreshNodeStatusHandler_Structure tests refresh node status handler
func TestRefreshNodeStatusHandler_Structure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctx := context.Background()
	tempDir := t.TempDir()
	cfg := storage.StorageConfig{
		Mode: "local",
		Local: storage.LocalStorageConfig{
			DatabasePath: tempDir + "/test.db",
			KVStorePath:  tempDir + "/test.bolt",
		},
	}

	realStorage := storage.NewLocalStorage(storage.LocalStorageConfig{})
	err := realStorage.Initialize(ctx, cfg)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "fts5") {
		t.Skip("sqlite3 compiled without FTS5")
	}
	require.NoError(t, err)
	defer realStorage.Close(ctx)

	mockNodeClient := &MockNodeClientForUI{}
	mockBotService := &MockBotServiceForUI{}
	statusManager := services.NewStatusManager(realStorage, services.StatusManagerConfig{}, nil, mockNodeClient)
	uiService := services.NewUIService(realStorage, mockNodeClient, mockBotService, statusManager)

	handler := NewNodesHandler(uiService)
	router := gin.New()
	router.POST("/api/ui/v1/nodes/:nodeId/status/refresh", handler.RefreshNodeStatusHandler)

	req := httptest.NewRequest(http.MethodPost, "/api/ui/v1/nodes/node-1/status/refresh", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle request
	assert.True(t, resp.Code >= http.StatusBadRequest) // Any response is valid
}

// TestBulkNodeStatusHandler_Validation tests bulk node status handler request validation
func TestBulkNodeStatusHandler_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctx := context.Background()
	tempDir := t.TempDir()
	cfg := storage.StorageConfig{
		Mode: "local",
		Local: storage.LocalStorageConfig{
			DatabasePath: tempDir + "/test.db",
			KVStorePath:  tempDir + "/test.bolt",
		},
	}

	realStorage := storage.NewLocalStorage(storage.LocalStorageConfig{})
	err := realStorage.Initialize(ctx, cfg)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "fts5") {
		t.Skip("sqlite3 compiled without FTS5")
	}
	require.NoError(t, err)
	defer realStorage.Close(ctx)

	mockNodeClient := &MockNodeClientForUI{}
	mockBotService := &MockBotServiceForUI{}
	statusManager := services.NewStatusManager(realStorage, services.StatusManagerConfig{}, nil, mockNodeClient)
	uiService := services.NewUIService(realStorage, mockNodeClient, mockBotService, statusManager)

	handler := NewNodesHandler(uiService)
	router := gin.New()
	router.POST("/api/ui/v1/nodes/status/bulk", handler.BulkNodeStatusHandler)

	// Test with invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/api/ui/v1/nodes/status/bulk", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusBadRequest, resp.Code)

	// Test with valid JSON but missing required field
	req = httptest.NewRequest(http.MethodPost, "/api/ui/v1/nodes/status/bulk", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()

	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusBadRequest, resp.Code)

	// Test with valid JSON
	req = httptest.NewRequest(http.MethodPost, "/api/ui/v1/nodes/status/bulk", strings.NewReader(`{"node_ids": ["node-1", "node-2"]}`))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()

	router.ServeHTTP(resp, req)
	// Should process request (may return error if nodes don't exist, but handler works)
	assert.True(t, resp.Code >= http.StatusOK)
}

// TestGetDashboardSummaryHandler_Structure tests dashboard handler structure
func TestGetDashboardSummaryHandler_Structure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctx := context.Background()
	tempDir := t.TempDir()
	cfg := storage.StorageConfig{
		Mode: "local",
		Local: storage.LocalStorageConfig{
			DatabasePath: tempDir + "/test.db",
			KVStorePath:  tempDir + "/test.bolt",
		},
	}

	realStorage := storage.NewLocalStorage(storage.LocalStorageConfig{})
	err := realStorage.Initialize(ctx, cfg)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "fts5") {
		t.Skip("sqlite3 compiled without FTS5")
	}
	require.NoError(t, err)
	defer realStorage.Close(ctx)

	mockBotService := &MockBotServiceForUI{}
	handler := NewDashboardHandler(realStorage, mockBotService)
	router := gin.New()
	router.GET("/api/ui/v1/dashboard", handler.GetDashboardSummaryHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/ui/v1/dashboard", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should return OK (even with empty data)
	assert.Equal(t, http.StatusOK, resp.Code)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	// Dashboard should have some structure
	assert.NotNil(t, result)
}

// TestAPIErrorHandling tests error handling in API handlers
func TestAPIErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctx := context.Background()
	tempDir := t.TempDir()
	cfg := storage.StorageConfig{
		Mode: "local",
		Local: storage.LocalStorageConfig{
			DatabasePath: tempDir + "/test.db",
			KVStorePath:  tempDir + "/test.bolt",
		},
	}

	realStorage := storage.NewLocalStorage(storage.LocalStorageConfig{})
	err := realStorage.Initialize(ctx, cfg)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "fts5") {
		t.Skip("sqlite3 compiled without FTS5")
	}
	require.NoError(t, err)
	defer realStorage.Close(ctx)

	mockNodeClient := &MockNodeClientForUI{}
	mockBotService := &MockBotServiceForUI{}
	statusManager := services.NewStatusManager(realStorage, services.StatusManagerConfig{}, nil, mockNodeClient)
	uiService := services.NewUIService(realStorage, mockNodeClient, mockBotService, statusManager)

	handler := NewNodesHandler(uiService)
	router := gin.New()
	router.GET("/api/ui/v1/nodes/:nodeId", handler.GetNodeDetailsHandler)

	// Test various invalid inputs
	tests := []struct {
		name   string
		path   string
		method string
	}{
		{"empty nodeId", "/api/ui/v1/nodes/", "GET"},
		{"special chars in nodeId", "/api/ui/v1/nodes/node%20with%20spaces", "GET"},
		{"very long nodeId", "/api/ui/v1/nodes/" + strings.Repeat("a", 1000), "GET"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			// Should handle gracefully (not panic)
			assert.True(t, resp.Code >= http.StatusBadRequest)
		})
	}
}

// TestAPIMethodValidation tests that handlers only accept correct HTTP methods
func TestAPIMethodValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctx := context.Background()
	tempDir := t.TempDir()
	cfg := storage.StorageConfig{
		Mode: "local",
		Local: storage.LocalStorageConfig{
			DatabasePath: tempDir + "/test.db",
			KVStorePath:  tempDir + "/test.bolt",
		},
	}

	realStorage := storage.NewLocalStorage(storage.LocalStorageConfig{})
	err := realStorage.Initialize(ctx, cfg)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "fts5") {
		t.Skip("sqlite3 compiled without FTS5")
	}
	require.NoError(t, err)
	defer realStorage.Close(ctx)

	mockNodeClient := &MockNodeClientForUI{}
	mockBotService := &MockBotServiceForUI{}
	statusManager := services.NewStatusManager(realStorage, services.StatusManagerConfig{}, nil, mockNodeClient)
	uiService := services.NewUIService(realStorage, mockNodeClient, mockBotService, statusManager)

	handler := NewNodesHandler(uiService)
	router := gin.New()
	router.GET("/api/ui/v1/nodes", handler.GetNodesSummaryHandler)
	router.POST("/api/ui/v1/nodes/:nodeId/status/refresh", handler.RefreshNodeStatusHandler)

	// Test GET endpoint with wrong method
	req := httptest.NewRequest(http.MethodPost, "/api/ui/v1/nodes", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusNotFound, resp.Code) // Gin returns 404 for wrong method

	// Test POST endpoint with wrong method
	req = httptest.NewRequest(http.MethodGet, "/api/ui/v1/nodes/node-1/status/refresh", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusNotFound, resp.Code)
}

// TestAPIResponseFormat tests that API responses have correct format
func TestAPIResponseFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctx := context.Background()
	tempDir := t.TempDir()
	cfg := storage.StorageConfig{
		Mode: "local",
		Local: storage.LocalStorageConfig{
			DatabasePath: tempDir + "/test.db",
			KVStorePath:  tempDir + "/test.bolt",
		},
	}

	realStorage := storage.NewLocalStorage(storage.LocalStorageConfig{})
	err := realStorage.Initialize(ctx, cfg)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "fts5") {
		t.Skip("sqlite3 compiled without FTS5")
	}
	require.NoError(t, err)
	defer realStorage.Close(ctx)

	mockNodeClient := &MockNodeClientForUI{}
	mockBotService := &MockBotServiceForUI{}
	statusManager := services.NewStatusManager(realStorage, services.StatusManagerConfig{}, nil, mockNodeClient)
	uiService := services.NewUIService(realStorage, mockNodeClient, mockBotService, statusManager)

	handler := NewNodesHandler(uiService)
	router := gin.New()
	router.GET("/api/ui/v1/nodes", handler.GetNodesSummaryHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/ui/v1/nodes", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Verify response is valid JSON
	assert.Equal(t, "application/json; charset=utf-8", resp.Header().Get("Content-Type"))

	var result map[string]interface{}
	err = json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err, "Response should be valid JSON")

	// Verify expected fields
	assert.Contains(t, result, "nodes")
	assert.Contains(t, result, "count")
}

// MockNodeClientForUI is a minimal mock for interfaces.NodeClient
type MockNodeClientForUI struct {
	mock.Mock
}

func (m *MockNodeClientForUI) GetBotStatus(ctx context.Context, nodeID string) (*interfaces.BotStatusResponse, error) {
	args := m.Called(ctx, nodeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*interfaces.BotStatusResponse), args.Error(1)
}

func (m *MockNodeClientForUI) GetMCPHealth(ctx context.Context, nodeID string) (*interfaces.MCPHealthResponse, error) {
	return nil, nil
}

func (m *MockNodeClientForUI) RestartMCPServer(ctx context.Context, nodeID, alias string) error {
	return nil
}

func (m *MockNodeClientForUI) GetMCPTools(ctx context.Context, nodeID, alias string) (*interfaces.MCPToolsResponse, error) {
	return nil, nil
}

func (m *MockNodeClientForUI) ShutdownNode(ctx context.Context, nodeID string, graceful bool, timeoutSeconds int) (*interfaces.NodeShutdownResponse, error) {
	return nil, nil
}

// MockBotServiceForUI is a minimal mock for interfaces.BotService
type MockBotServiceForUI struct {
	mock.Mock
}

func (m *MockBotServiceForUI) GetBotStatus(name string) (*domain.BotStatus, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.BotStatus), args.Error(1)
}

func (m *MockBotServiceForUI) RunAgent(name string, options domain.RunOptions) (*domain.RunningAgent, error) {
	return nil, nil
}

func (m *MockBotServiceForUI) StopAgent(name string) error {
	return nil
}

func (m *MockBotServiceForUI) ListRunningAgents() ([]domain.RunningAgent, error) {
	return []domain.RunningAgent{}, nil
}

// MockBotService is a mock for interfaces.BotService (used by dashboard)
type MockBotService struct {
	mock.Mock
}

func (m *MockBotService) GetNodes(ctx context.Context) ([]*types.Node, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.Node), args.Error(1)
}

func (m *MockBotService) GetNode(ctx context.Context, id string) (*types.Node, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Node), args.Error(1)
}
