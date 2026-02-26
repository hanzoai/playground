package ui

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/control-plane/internal/storage"
)

// BudgetHandler handles per-bot budget CRUD operations.
type BudgetHandler struct {
	storage storage.StorageProvider
}

// NewBudgetHandler creates a new BudgetHandler.
func NewBudgetHandler(s storage.StorageProvider) *BudgetHandler {
	return &BudgetHandler{storage: s}
}

// ListBudgets returns all bot budgets.
// GET /api/v1/budgets
func (h *BudgetHandler) ListBudgets(c *gin.Context) {
	ctx := c.Request.Context()

	budgets, err := h.storage.ListBotBudgets(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to list budgets"})
		return
	}

	if budgets == nil {
		budgets = []*storage.BotBudget{}
	}

	c.JSON(http.StatusOK, budgets)
}

// GetBudget returns the budget for a specific bot.
// GET /api/v1/budgets/:botId
func (h *BudgetHandler) GetBudget(c *gin.Context) {
	ctx := c.Request.Context()
	botID := c.Param("botId")

	budget, err := h.storage.GetBotBudget(ctx, botID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to get budget"})
		return
	}

	if budget == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "no budget configured for this bot"})
		return
	}

	c.JSON(http.StatusOK, budget)
}

// SetBudget upserts a budget for a specific bot.
// PUT /api/v1/budgets/:botId
func (h *BudgetHandler) SetBudget(c *gin.Context) {
	ctx := c.Request.Context()
	botID := c.Param("botId")

	var budget storage.BotBudget
	if err := c.ShouldBindJSON(&budget); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	budget.BotID = botID

	if err := h.storage.SetBotBudget(ctx, &budget); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to save budget"})
		return
	}

	c.JSON(http.StatusOK, &budget)
}

// DeleteBudget removes the budget for a specific bot.
// DELETE /api/v1/budgets/:botId
func (h *BudgetHandler) DeleteBudget(c *gin.Context) {
	ctx := c.Request.Context()
	botID := c.Param("botId")

	if err := h.storage.DeleteBotBudget(ctx, botID); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to delete budget"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// CheckBudget returns the budget status (allowed/exceeded) for a bot.
// GET /api/v1/budgets/:botId/check
func (h *BudgetHandler) CheckBudget(c *gin.Context) {
	ctx := c.Request.Context()
	botID := c.Param("botId")

	status, err := h.storage.CheckBotBudget(ctx, botID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to check budget"})
		return
	}

	c.JSON(http.StatusOK, status)
}

// GetSpendHistory returns recent spend records for a bot.
// GET /api/v1/budgets/:botId/spend?days=30&limit=100
func (h *BudgetHandler) GetSpendHistory(c *gin.Context) {
	ctx := c.Request.Context()
	botID := c.Param("botId")

	days := 30
	if d := c.Query("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
			days = parsed
		}
	}

	limit := 100
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	since := time.Now().UTC().AddDate(0, 0, -days)
	records, err := h.storage.GetBotSpendHistory(ctx, botID, since, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to get spend history"})
		return
	}

	if records == nil {
		records = []*storage.BotSpendRecord{}
	}

	c.JSON(http.StatusOK, records)
}
