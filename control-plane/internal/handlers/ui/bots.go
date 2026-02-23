package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hanzoai/playground/control-plane/internal/events"
	"github.com/hanzoai/playground/control-plane/internal/storage"
	"github.com/hanzoai/playground/control-plane/pkg/types"

	"github.com/gin-gonic/gin"
)

// BotsHandler provides handlers for UI-related bot operations.
type BotsHandler struct {
	storage storage.StorageProvider
}

// NewBotsHandler creates a new BotsHandler.
func NewBotsHandler(storageProvider storage.StorageProvider) *BotsHandler {
	return &BotsHandler{storage: storageProvider}
}

// BotWithNode represents a bot with its associated node information.
type BotWithNode struct {
	// Bot identification
	BotID  string `json:"bot_id"` // Format: "node_id.bot_id"
	Name        string `json:"name"`        // Human-readable name
	Description string `json:"description"` // Bot description

	// Node context
	NodeID      string             `json:"node_id"`
	NodeStatus  types.HealthStatus `json:"node_status"`
	NodeVersion string             `json:"node_version"`

	// Bot details
	InputSchema  interface{}        `json:"input_schema"`
	OutputSchema interface{}        `json:"output_schema"`
	MemoryConfig types.MemoryConfig `json:"memory_config"`
	Tags         []string           `json:"tags"`

	// Performance metrics (placeholder for future implementation)
	AvgResponseTime *int       `json:"avg_response_time_ms,omitempty"`
	SuccessRate     *float64   `json:"success_rate,omitempty"`
	TotalRuns       *int       `json:"total_runs,omitempty"`
	LastExecuted    *time.Time `json:"last_executed,omitempty"`

	// Timestamps
	LastUpdated time.Time `json:"last_updated"`
}

// BotsResponse represents the response for the all bots endpoint.
type BotsResponse struct {
	Bots    []BotWithNode `json:"bots"`
	Total        int                `json:"total"`
	OnlineCount  int                `json:"online_count"`
	OfflineCount int                `json:"offline_count"`
	NodesCount   int                `json:"nodes_count"`
}

// GetAllBotsHandler handles requests for all bots across all nodes.
func (h *BotsHandler) GetAllBotsHandler(c *gin.Context) {
	// Parse query parameters
	statusFilter := c.Query("status") // "online", "offline", "all" (default: "all")
	searchTerm := c.Query("search")   // Search in bot names/descriptions
	limitStr := c.Query("limit")      // Pagination limit
	offsetStr := c.Query("offset")    // Pagination offset

	// Set defaults
	if statusFilter == "" {
		statusFilter = "all"
	}

	limit := 50 // Default limit
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	offset := 0 // Default offset
	if offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	// Get all nodes based on status filter
	var filters types.BotFilters
	if statusFilter == "online" {
		activeStatus := types.HealthStatusActive
		filters.HealthStatus = &activeStatus
	}

	ctx := c.Request.Context()
	nodes, err := h.storage.ListNodes(ctx, filters)
	if err != nil {
		fmt.Printf("❌ Error listing agents for bots: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve nodes"})
		return
	}

	fmt.Printf("📊 Found %d nodes for bot aggregation\n", len(nodes))

	// Aggregate bots from all nodes
	var allBots []BotWithNode
	onlineCount := 0
	offlineCount := 0

	// Ensure we always have a valid slice
	if allBots == nil {
		allBots = make([]BotWithNode, 0)
	}

	for _, node := range nodes {
		fmt.Printf("  Processing node %s with %d bots (status: %s)\n",
			node.ID, len(node.Bots), node.HealthStatus)

		for _, bot := range node.Bots {
			// Create full bot ID
			fullBotID := fmt.Sprintf("%s.%s", node.ID, bot.ID)

			// Extract name from bot ID (use ID as name for now)
			name := bot.ID
			description := fmt.Sprintf("Bot %s from node %s", bot.ID, node.ID)

			// DIAGNOSTIC LOG: Track bot status determination
			fmt.Printf("🔍 BOT_STATUS_DEBUG: Bot %s - NodeHealth: %s, NodeLifecycle: %s, LastHeartbeat: %s\n",
				fullBotID, node.HealthStatus, node.LifecycleStatus, node.LastHeartbeat.Format(time.RFC3339))

			botWithNode := BotWithNode{
				BotID:   fullBotID,
				Name:         name,
				Description:  description,
				NodeID:       node.ID,
				NodeStatus:   node.HealthStatus,
				NodeVersion:  node.Version,
				InputSchema:  bot.InputSchema,
				OutputSchema: bot.OutputSchema,
				MemoryConfig: bot.MemoryConfig,
				Tags:         bot.Tags,
				LastUpdated:  node.LastHeartbeat,
			}

			// Apply search filter
			if searchTerm != "" {
				searchLower := strings.ToLower(searchTerm)
				if !strings.Contains(strings.ToLower(name), searchLower) &&
					!strings.Contains(strings.ToLower(description), searchLower) &&
					!strings.Contains(strings.ToLower(bot.ID), searchLower) {
					continue
				}
			}

			// Count by status
			if node.HealthStatus == types.HealthStatusActive {
				onlineCount++
			} else {
				offlineCount++
			}

			allBots = append(allBots, botWithNode)
		}
	}

	// Apply status filter after aggregation (for accurate counts)
	var filteredBots []BotWithNode
	if statusFilter == "online" {
		for _, bot := range allBots {
			if bot.NodeStatus == types.HealthStatusActive {
				filteredBots = append(filteredBots, bot)
			}
		}
	} else if statusFilter == "offline" {
		for _, bot := range allBots {
			if bot.NodeStatus != types.HealthStatusActive {
				filteredBots = append(filteredBots, bot)
			}
		}
	} else {
		filteredBots = allBots
	}

	// Apply pagination
	total := len(filteredBots)
	start := offset
	end := offset + limit

	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	paginatedBots := filteredBots[start:end]

	fmt.Printf("📋 Returning %d bots (total: %d, online: %d, offline: %d) from %d nodes\n",
		len(paginatedBots), total, onlineCount, offlineCount, len(nodes))

	response := BotsResponse{
		Bots:    paginatedBots,
		Total:        total,
		OnlineCount:  onlineCount,
		OfflineCount: offlineCount,
		NodesCount:   len(nodes),
	}

	c.JSON(http.StatusOK, response)
}

// GetBotDetailsHandler handles requests for detailed information about a specific bot.
func (h *BotsHandler) GetBotDetailsHandler(c *gin.Context) {
	botID := c.Param("botId")
	if botID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bot_id is required"})
		return
	}

	// Parse bot ID (format: "node_id.bot_id")
	parts := strings.SplitN(botID, ".", 2)
	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bot_id format, expected 'node_id.bot_id'"})
		return
	}

	nodeID := parts[0]
	localBotID := parts[1]

	// Get the node
	ctx := c.Request.Context()
	node, err := h.storage.GetNode(ctx, nodeID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	// Find the bot
	var foundBot *types.BotDefinition
	for _, bot := range node.Bots {
		if bot.ID == localBotID {
			foundBot = &bot
			break
		}
	}

	if foundBot == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "bot not found"})
		return
	}

	// Create detailed response
	botDetails := BotWithNode{
		BotID:   botID,
		Name:         foundBot.ID,
		Description:  fmt.Sprintf("Bot %s from node %s", foundBot.ID, nodeID),
		NodeID:       nodeID,
		NodeStatus:   node.HealthStatus,
		NodeVersion:  node.Version,
		InputSchema:  foundBot.InputSchema,
		OutputSchema: foundBot.OutputSchema,
		MemoryConfig: foundBot.MemoryConfig,
		Tags:         foundBot.Tags,
		LastUpdated:  node.LastHeartbeat,
	}

	fmt.Printf("📋 Retrieved details for bot %s\n", botID)

	c.JSON(http.StatusOK, botDetails)
}

// PerformanceMetrics represents performance data for a bot
type PerformanceMetrics struct {
	AvgResponseTimeMs int               `json:"avg_response_time_ms"`
	SuccessRate       float64           `json:"success_rate"`
	TotalExecutions   int               `json:"total_executions"`
	ExecutionsLast24h int               `json:"executions_last_24h"`
	RecentExecutions  []RecentExecution `json:"recent_executions"`
}

// RecentExecution represents a recent execution for metrics
type RecentExecution struct {
	ExecutionID string    `json:"execution_id"`
	Status      string    `json:"status"`
	DurationMs  int       `json:"duration_ms"`
	Timestamp   time.Time `json:"timestamp"`
}

// ExecutionHistory represents paginated execution history
type ExecutionHistory struct {
	Executions []ExecutionRecord `json:"executions"`
	Total      int               `json:"total"`
	Page       int               `json:"page"`
	Limit      int               `json:"limit"`
	HasMore    bool              `json:"has_more"`
}

// ExecutionRecord represents a single execution record
type ExecutionRecord struct {
	ExecutionID string                 `json:"execution_id"`
	Status      string                 `json:"status"`
	Input       map[string]interface{} `json:"input"`
	Output      map[string]interface{} `json:"output,omitempty"`
	Error       string                 `json:"error,omitempty"`
	DurationMs  int                    `json:"duration_ms"`
	Timestamp   time.Time              `json:"timestamp"`
}

// ExecutionTemplate represents a saved execution template
type ExecutionTemplate struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Input       map[string]interface{} `json:"input"`
	CreatedAt   time.Time              `json:"created_at"`
}

// GetPerformanceMetricsHandler handles requests for bot performance metrics
func (h *BotsHandler) GetPerformanceMetricsHandler(c *gin.Context) {
	botID := c.Param("botId")
	if botID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bot_id is required"})
		return
	}

	// Parse bot ID (format: "node_id.bot_id")
	parts := strings.SplitN(botID, ".", 2)
	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bot_id format, expected 'node_id.bot_id'"})
		return
	}

	// Get real performance metrics from storage
	ctx := c.Request.Context()
	metrics, err := h.storage.GetBotPerformanceMetrics(ctx, botID)
	if err != nil {
		fmt.Printf("❌ Error getting performance metrics for bot %s: %v\n", botID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve performance metrics"})
		return
	}

	fmt.Printf("📊 Retrieved performance metrics for bot %s: %d executions, %.2f%% success rate\n",
		botID, metrics.TotalExecutions, metrics.SuccessRate*100)

	c.JSON(http.StatusOK, metrics)
}

// GetExecutionHistoryHandler handles requests for bot execution history
func (h *BotsHandler) GetExecutionHistoryHandler(c *gin.Context) {
	botID := c.Param("botId")
	if botID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bot_id is required"})
		return
	}

	// Parse pagination parameters
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "20")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	// Parse bot ID (format: "node_id.bot_id")
	parts := strings.SplitN(botID, ".", 2)
	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bot_id format, expected 'node_id.bot_id'"})
		return
	}

	// Get real execution history from storage
	ctx := c.Request.Context()
	history, err := h.storage.GetBotExecutionHistory(ctx, botID, page, limit)
	if err != nil {
		fmt.Printf("❌ Error getting execution history for bot %s: %v\n", botID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve execution history"})
		return
	}

	fmt.Printf("📋 Retrieved execution history for bot %s: %d executions (page %d, limit %d)\n",
		botID, len(history.Executions), page, limit)

	c.JSON(http.StatusOK, history)
}

// GetExecutionTemplatesHandler handles requests for bot execution templates
func (h *BotsHandler) GetExecutionTemplatesHandler(c *gin.Context) {
	botID := c.Param("botId")
	if botID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bot_id is required"})
		return
	}

	// Parse bot ID (format: "node_id.bot_id")
	parts := strings.SplitN(botID, ".", 2)
	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bot_id format, expected 'node_id.bot_id'"})
		return
	}

	// Return empty list — templates are stored per-bot on the gateway side
	templates := []ExecutionTemplate{}
	c.JSON(http.StatusOK, templates)
}

// SaveExecutionTemplateHandler handles saving new execution templates
func (h *BotsHandler) SaveExecutionTemplateHandler(c *gin.Context) {
	botID := c.Param("botId")
	if botID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bot_id is required"})
		return
	}

	// Parse bot ID (format: "node_id.bot_id")
	parts := strings.SplitN(botID, ".", 2)
	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bot_id format, expected 'node_id.bot_id'"})
		return
	}

	var template struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Input       map[string]interface{} `json:"input"`
	}

	if err := c.ShouldBindJSON(&template); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Generate template record — persisted when gateway-side template storage is wired
	savedTemplate := ExecutionTemplate{
		ID:          fmt.Sprintf("template_%d", time.Now().Unix()),
		Name:        template.Name,
		Description: template.Description,
		Input:       template.Input,
		CreatedAt:   time.Now(),
	}

	c.JSON(http.StatusCreated, savedTemplate)
}

// StreamBotEventsHandler handles bot event streaming
// GET /api/ui/v1/bots/events
func (h *BotsHandler) StreamBotEventsHandler(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Headers", "Cache-Control")

	// Generate unique subscriber ID
	subscriberID := fmt.Sprintf("bot_sse_%d_%s", time.Now().UnixNano(), c.ClientIP())

	// Subscribe to bot events
	eventChan := events.GlobalBotEventBus.Subscribe(subscriberID)
	defer events.GlobalBotEventBus.Unsubscribe(subscriberID)

	// Send initial connection confirmation
	initialEvent := map[string]interface{}{
		"type":      "connected",
		"message":   "Bot events stream connected",
		"timestamp": time.Now().Format(time.RFC3339),
	}

	if eventJSON, err := json.Marshal(initialEvent); err == nil {
		if !writeSSE(c, eventJSON) {
			return
		}
	}

	// Set up context for handling client disconnection
	ctx := c.Request.Context()

	// Send periodic heartbeat to keep connection alive
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	fmt.Printf("🔄 Bot SSE client connected: %s\n", subscriberID)

	for {
		select {
		case <-ctx.Done():
			// Client disconnected
			fmt.Printf("🔌 Bot SSE client disconnected: %s\n", subscriberID)
			return
		case <-heartbeatTicker.C:
			// Send heartbeat
			heartbeat := map[string]interface{}{
				"type":      "heartbeat",
				"timestamp": time.Now().Format(time.RFC3339),
			}
			if heartbeatJSON, err := json.Marshal(heartbeat); err == nil {
				if !writeSSE(c, heartbeatJSON) {
					return
				}
			}
		case event, ok := <-eventChan:
			if !ok {
				// Channel closed
				fmt.Printf("📡 Bot SSE channel closed for: %s\n", subscriberID)
				return
			}

			// Convert event to JSON and send
			if eventJSON, err := event.ToJSON(); err == nil {
				if !writeSSE(c, []byte(eventJSON)) {
					return
				}
				fmt.Printf("📤 Sent bot event %s to client %s\n", event.Type, subscriberID)
			}
		}
	}
}
