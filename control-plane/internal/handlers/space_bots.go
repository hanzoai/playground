package handlers

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hanzoai/playground/control-plane/internal/spaces"
)

// SpaceBotHandler manages bots within a Space by proxying to hanzo/node V2 API.
type SpaceBotHandler struct {
	store  spaces.Store
	client *http.Client
}

// NewSpaceBotHandler creates a new SpaceBotHandler.
func NewSpaceBotHandler(store spaces.Store) *SpaceBotHandler {
	return &SpaceBotHandler{
		store:  store,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// CreateBotRequest is the payload for POST /api/v1/spaces/:id/bots.
type CreateBotRequest struct {
	NodeID string `json:"node_id" binding:"required"` // which node to deploy on
	Name   string `json:"name" binding:"required"`
	Model  string `json:"model"`
	View   string `json:"view"` // terminal, desktop-linux, desktop-mac, desktop-win, chat
}

// CreateBot creates a bot in the space by calling hanzo/node's POST /v2/add_agent.
func (h *SpaceBotHandler) CreateBot(c *gin.Context) {
	spaceID := c.Param("id")
	var req CreateBotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	view := req.View
	if view == "" {
		view = "terminal"
	}

	// Look up the node to get its endpoint
	node, err := h.store.GetNode(c.Request.Context(), spaceID, req.NodeID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("node not found: %v", err)})
		return
	}

	botID := uuid.New().String()[:12]

	// Proxy the add_agent call to the hanzo/node V2 API
	agentID := botID
	if node.Endpoint != "" {
		proxyResp, proxyErr := h.proxyToNode(c, node.Endpoint, "POST", "/v2/add_agent", c.Request.Body)
		if proxyErr == nil && proxyResp.StatusCode < 300 {
			// Agent created on node successfully
			defer proxyResp.Body.Close()
		}
		// Even if proxy fails, we still record the bot in our DB for tracking
	}

	bot := &spaces.SpaceBot{
		SpaceID: spaceID,
		BotID:   botID,
		NodeID:  req.NodeID,
		AgentID: agentID,
		Name:    req.Name,
		Model:   req.Model,
		View:    view,
		Status:  "running",
	}

	if err := h.store.CreateBot(c.Request.Context(), bot); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, bot)
}

// ListBots handles GET /api/v1/spaces/:id/bots.
func (h *SpaceBotHandler) ListBots(c *gin.Context) {
	spaceID := c.Param("id")
	bots, err := h.store.ListBots(c.Request.Context(), spaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if bots == nil {
		bots = make([]*spaces.SpaceBot, 0)
	}
	c.JSON(http.StatusOK, gin.H{"bots": bots})
}

// RemoveBot handles DELETE /api/v1/spaces/:id/bots/:bid.
func (h *SpaceBotHandler) RemoveBot(c *gin.Context) {
	spaceID := c.Param("id")
	botID := c.Param("bid")

	// Look up bot to find its node
	bot, err := h.store.GetBot(c.Request.Context(), spaceID, botID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Proxy remove_agent to the hanzo/node
	node, nodeErr := h.store.GetNode(c.Request.Context(), spaceID, bot.NodeID)
	if nodeErr == nil && node.Endpoint != "" {
		_, _ = h.proxyToNode(c, node.Endpoint, "POST", "/v2/remove_agent", c.Request.Body)
	}

	if err := h.store.RemoveBot(c.Request.Context(), spaceID, botID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"removed": true})
}

// ChatMessage handles POST /api/v1/spaces/:id/bots/:bid/chat.
// Proxies to hanzo/node's POST /v2/job_message.
func (h *SpaceBotHandler) ChatMessage(c *gin.Context) {
	spaceID := c.Param("id")
	botID := c.Param("bid")

	bot, err := h.store.GetBot(c.Request.Context(), spaceID, botID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	node, err := h.store.GetNode(c.Request.Context(), spaceID, bot.NodeID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("bot's node not found: %v", err)})
		return
	}

	resp, err := h.proxyToNode(c, node.Endpoint, "POST", "/v2/job_message", c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("node proxy error: %v", err)})
		return
	}
	defer resp.Body.Close()

	// Stream the response back
	c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
}

// ChatHistory handles GET /api/v1/spaces/:id/bots/:bid/chat.
// Proxies to hanzo/node's POST /v2/last_messages.
func (h *SpaceBotHandler) ChatHistory(c *gin.Context) {
	spaceID := c.Param("id")
	botID := c.Param("bid")

	bot, err := h.store.GetBot(c.Request.Context(), spaceID, botID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	node, err := h.store.GetNode(c.Request.Context(), spaceID, bot.NodeID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("bot's node not found: %v", err)})
		return
	}

	resp, err := h.proxyToNode(c, node.Endpoint, "POST", "/v2/last_messages", c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("node proxy error: %v", err)})
		return
	}
	defer resp.Body.Close()

	c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
}

// proxyToNode forwards an HTTP request to a hanzo/node endpoint.
func (h *SpaceBotHandler) proxyToNode(c *gin.Context, nodeEndpoint, method, path string, body io.Reader) (*http.Response, error) {
	url := nodeEndpoint + path
	req, err := http.NewRequestWithContext(c.Request.Context(), method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// Forward authorization header if present
	if auth := c.GetHeader("Authorization"); auth != "" {
		req.Header.Set("Authorization", auth)
	}

	return h.client.Do(req)
}
