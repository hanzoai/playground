package ui

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/control-plane/internal/storage"
)

// WalletHandler handles per-bot wallet CRUD operations.
type WalletHandler struct {
	storage storage.StorageProvider
}

// NewWalletHandler creates a new WalletHandler.
func NewWalletHandler(s storage.StorageProvider) *WalletHandler {
	return &WalletHandler{storage: s}
}

// GetWallet returns the wallet for a specific bot.
// GET /api/v1/:botId/wallet
func (h *WalletHandler) GetWallet(c *gin.Context) {
	ctx := c.Request.Context()
	botID := c.Param("botId")

	wallet, err := h.storage.GetBotWallet(ctx, botID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to get wallet"})
		return
	}

	if wallet == nil {
		// Return a default empty wallet so the frontend can render gracefully.
		wallet = &storage.BotWallet{
			BotID:   botID,
			Enabled: false,
		}
	}

	c.JSON(http.StatusOK, wallet)
}

// fundRequest is the JSON body for funding a wallet.
type fundRequest struct {
	AmountAiCoin   float64 `json:"amount_ai_coin"`
	AmountUsdCents int64   `json:"amount_usd_cents"`
	Source         string  `json:"source"`
	Description    string  `json:"description"`
}

// FundWallet adds funds to a bot wallet.
// POST /api/v1/:botId/wallet/fund
func (h *WalletHandler) FundWallet(c *gin.Context) {
	ctx := c.Request.Context()
	botID := c.Param("botId")

	var req fundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	tx, err := h.storage.FundBotWallet(ctx, botID, req.AmountAiCoin, req.AmountUsdCents, req.Source, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to fund wallet: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, tx)
}

// withdrawRequest is the JSON body for withdrawing from a wallet.
type withdrawRequest struct {
	AmountAiCoin   float64 `json:"amount_ai_coin"`
	AmountUsdCents int64   `json:"amount_usd_cents"`
	Description    string  `json:"description"`
}

// WithdrawWallet withdraws funds from a bot wallet.
// POST /api/v1/:botId/wallet/withdraw
func (h *WalletHandler) WithdrawWallet(c *gin.Context) {
	ctx := c.Request.Context()
	botID := c.Param("botId")

	var req withdrawRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	tx, err := h.storage.WithdrawFromBotWallet(ctx, botID, req.AmountAiCoin, req.AmountUsdCents, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to withdraw from wallet: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, tx)
}

// GetTransactions returns recent transactions for a bot wallet.
// GET /api/v1/:botId/wallet/transactions?limit=50
func (h *WalletHandler) GetTransactions(c *gin.Context) {
	ctx := c.Request.Context()
	botID := c.Param("botId")

	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	transactions, err := h.storage.GetWalletTransactions(ctx, botID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to get transactions"})
		return
	}

	if transactions == nil {
		transactions = []*storage.WalletTransaction{}
	}

	c.JSON(http.StatusOK, transactions)
}

// ListAutoPurchaseRules returns all auto-purchase rules for a bot.
// GET /api/v1/:botId/wallet/auto-purchase
func (h *WalletHandler) ListAutoPurchaseRules(c *gin.Context) {
	ctx := c.Request.Context()
	botID := c.Param("botId")

	rules, err := h.storage.GetAutoPurchaseRules(ctx, botID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to list auto-purchase rules"})
		return
	}

	if rules == nil {
		rules = []*storage.AutoPurchaseRule{}
	}

	c.JSON(http.StatusOK, rules)
}

// SaveAutoPurchaseRule creates or updates an auto-purchase rule.
// POST /api/v1/:botId/wallet/auto-purchase (create)
// PUT  /api/v1/:botId/wallet/auto-purchase/:ruleId (update)
func (h *WalletHandler) SaveAutoPurchaseRule(c *gin.Context) {
	ctx := c.Request.Context()
	botID := c.Param("botId")

	var rule storage.AutoPurchaseRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	rule.BotID = botID

	// If ruleId is provided in the URL, use it (update case).
	if ruleID := c.Param("ruleId"); ruleID != "" {
		rule.ID = ruleID
	}

	if err := h.storage.SaveAutoPurchaseRule(ctx, &rule); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to save auto-purchase rule"})
		return
	}

	c.JSON(http.StatusOK, &rule)
}

// DeleteAutoPurchaseRule removes an auto-purchase rule.
// DELETE /api/v1/:botId/wallet/auto-purchase/:ruleId
func (h *WalletHandler) DeleteAutoPurchaseRule(c *gin.Context) {
	ctx := c.Request.Context()
	botID := c.Param("botId")
	ruleID := c.Param("ruleId")

	if err := h.storage.DeleteAutoPurchaseRule(ctx, botID, ruleID); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to delete auto-purchase rule"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// ExecuteAutoPurchaseRule manually triggers an auto-purchase rule.
// POST /api/v1/:botId/wallet/auto-purchase/:ruleId/execute
func (h *WalletHandler) ExecuteAutoPurchaseRule(c *gin.Context) {
	_ = c.Request.Context()
	botID := c.Param("botId")
	ruleID := c.Param("ruleId")

	// Stub: in production this would trigger the actual purchase flow.
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "auto-purchase rule execution initiated",
		"bot_id":  botID,
		"rule_id": ruleID,
	})
}

// walletsSummaryResponse is the response for the wallets summary endpoint.
type walletsSummaryResponse struct {
	TotalBots      int     `json:"total_bots"`
	TotalAiCoin    float64 `json:"total_ai_coin"`
	TotalUsdCents  int64   `json:"total_usd_cents"`
}

// GetWalletsSummary returns aggregate wallet statistics.
// GET /api/v1/wallets/summary
func (h *WalletHandler) GetWalletsSummary(c *gin.Context) {
	ctx := c.Request.Context()

	totalBots, totalAiCoin, totalUsdCents, err := h.storage.GetWalletsSummary(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to get wallets summary"})
		return
	}

	c.JSON(http.StatusOK, walletsSummaryResponse{
		TotalBots:     totalBots,
		TotalAiCoin:   totalAiCoin,
		TotalUsdCents: totalUsdCents,
	})
}
