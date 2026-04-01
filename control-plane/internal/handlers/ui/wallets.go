package ui

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/control-plane/internal/storage"
)

// WalletHandler handles per-bot wallet CRUD operations.
type WalletHandler struct {
	storage storage.StorageProvider
	billing *BillingProxyHandler
}

// NewWalletHandler creates a new WalletHandler.
func NewWalletHandler(s storage.StorageProvider) *WalletHandler {
	return &WalletHandler{
		storage: s,
		billing: NewBillingProxyHandler(),
	}
}

// commerceWithdraw calls Commerce's POST /api/v1/billing/withdraw endpoint
// using the caller's own IAM token. Commerce enforces that non-admin users
// can only withdraw from their own account, so the token must belong to userID.
// Returns a non-nil error if the balance is insufficient or the call fails.
func (h *WalletHandler) commerceWithdraw(ctx context.Context, userToken, userID string, amountCents int64, notes string) error {
	body, _ := json.Marshal(map[string]interface{}{
		"user":     userID,
		"currency": "usd",
		"amount":   amountCents,
		"notes":    notes,
		"tags":     "bot-wallet-fund",
	})

	targetURL := fmt.Sprintf("%s/api/v1/billing/withdraw", h.billing.commerceURL)

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "POST", targetURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := h.billing.client.Do(req)
	if err != nil {
		return fmt.Errorf("commerce unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("commerce withdraw failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

// commerceDeposit calls Commerce's POST /api/v1/billing/deposit endpoint
// using the service token. This credits the bot's service account so that
// cloud-api's balance gate allows LLM requests.
func (h *WalletHandler) commerceDeposit(ctx context.Context, serviceUser string, amountCents int64, notes string) error {
	body, _ := json.Marshal(map[string]interface{}{
		"user":     serviceUser,
		"currency": "usd",
		"amount":   amountCents,
		"notes":    notes,
		"tags":     "bot-wallet-fund",
	})

	targetURL := fmt.Sprintf("%s/api/v1/billing/deposit", h.billing.commerceURL)

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "POST", targetURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build deposit request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	// Use service token (admin) for deposits — user tokens can't deposit.
	if h.billing.serviceToken != "" {
		req.Header.Set("Authorization", "Bearer "+h.billing.serviceToken)
	}

	resp, err := h.billing.client.Do(req)
	if err != nil {
		return fmt.Errorf("commerce unreachable for deposit: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("commerce deposit failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

// commerceServiceWithdraw calls Commerce's POST /api/v1/billing/withdraw
// using the service token (admin). Used to debit the bot service account.
func (h *WalletHandler) commerceServiceWithdraw(ctx context.Context, userID string, amountCents int64, notes string) error {
	body, _ := json.Marshal(map[string]interface{}{
		"user":     userID,
		"currency": "usd",
		"amount":   amountCents,
		"notes":    notes,
		"tags":     "bot-wallet-refund",
	})

	targetURL := fmt.Sprintf("%s/api/v1/billing/withdraw", h.billing.commerceURL)

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "POST", targetURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if h.billing.serviceToken != "" {
		req.Header.Set("Authorization", "Bearer "+h.billing.serviceToken)
	}

	resp, err := h.billing.client.Do(req)
	if err != nil {
		return fmt.Errorf("commerce unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("commerce withdraw failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

// botServiceAccount is the Commerce user ID for the shared bot LLM service
// account. Cloud-api bills LLM usage against this account. When users fund
// bot wallets, we deposit the amount here so the balance gate allows requests.
const botServiceAccount = "hanzo/cloud-agent-v2"

// jwtUserID extracts the "owner/name" Commerce user ID from a JWT Bearer header.
// Returns ("", "") if the header is missing, malformed, or has no owner/name.
func jwtUserID(authHeader string) (userID, rawToken string) {
	rawToken = strings.TrimPrefix(authHeader, "Bearer ")
	if rawToken == "" || rawToken == authHeader {
		return "", ""
	}
	parts := strings.Split(rawToken, ".")
	if len(parts) != 3 {
		return "", rawToken
	}
	payload := parts[1]
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}
	b, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return "", rawToken
	}
	var claims struct {
		Owner string `json:"owner"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(b, &claims); err != nil {
		return "", rawToken
	}
	if claims.Owner == "" || claims.Name == "" {
		return "", rawToken
	}
	return strings.ToLower(claims.Owner + "/" + claims.Name), rawToken
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
//
// When Source == "usd", a withdrawal is first created in Commerce to deduct
// the equivalent amount from the caller's IAM user balance. The local bot
// wallet is only updated after Commerce confirms the withdrawal, so the two
// ledgers stay in sync. If the user has insufficient balance Commerce returns
// 402 and the bot wallet is NOT updated.
func (h *WalletHandler) FundWallet(c *gin.Context) {
	ctx := c.Request.Context()
	botID := c.Param("botId")

	var req fundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	// When funding from the user's USD (Commerce) balance, deduct it first.
	// Accept both "usd" and "user_usd" — the frontend sends "user_usd".
	isUsdSource := strings.EqualFold(req.Source, "usd") || strings.EqualFold(req.Source, "user_usd")
	if isUsdSource && req.AmountUsdCents > 0 {
		userID, rawToken := jwtUserID(c.GetHeader("Authorization"))
		if userID == "" {
			c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "cannot determine user identity from token; re-login and try again"})
			return
		}

		notes := req.Description
		if notes == "" {
			notes = fmt.Sprintf("Bot wallet fund: %s", botID)
		}

		if err := h.commerceWithdraw(ctx, rawToken, userID, req.AmountUsdCents, notes); err != nil {
			// Preserve the Commerce HTTP status when possible (402 = insufficient balance).
			status := http.StatusPaymentRequired
			msg := err.Error()
			if strings.Contains(msg, "commerce withdraw failed (402)") {
				status = http.StatusPaymentRequired
			} else if strings.Contains(msg, "commerce withdraw failed (4") {
				status = http.StatusBadRequest
			} else if strings.Contains(msg, "commerce unreachable") {
				status = http.StatusBadGateway
			}
			c.JSON(status, ErrorResponse{Error: "failed to deduct from USD balance: " + msg})
			return
		}

		// Deposit the withdrawn amount to the bot service account so cloud-api's
		// balance gate allows LLM requests. This makes the flow:
		// user balance (hanzo/z) → withdraw → deposit → service account (hanzo/cloud-agent-v2)
		depositNotes := fmt.Sprintf("Bot wallet fund from %s: %s", userID, botID)
		if err := h.commerceDeposit(ctx, botServiceAccount, req.AmountUsdCents, depositNotes); err != nil {
			// The user's withdraw already succeeded — log the deposit failure
			// but don't fail the request. The funds are safe in the ledger.
			fmt.Printf("[WARN] bot wallet fund: user withdraw succeeded but service account deposit failed: %v\n", err)
		}
	}

	tx, err := h.storage.FundBotWallet(ctx, botID, req.AmountAiCoin, req.AmountUsdCents, req.Source, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to fund wallet: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, tx)
}

// walletWithdrawRequest is the JSON body for withdrawing from a wallet.
type walletWithdrawRequest struct {
	AmountAiCoin   float64 `json:"amount_ai_coin"`
	AmountUsdCents int64   `json:"amount_usd_cents"`
	Description    string  `json:"description"`
}

// WithdrawWallet withdraws funds from a bot wallet and deposits back to user's Commerce balance.
// POST /api/v1/:botId/wallet/withdraw
func (h *WalletHandler) WithdrawWallet(c *gin.Context) {
	ctx := c.Request.Context()
	botID := c.Param("botId")

	var req walletWithdrawRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	// Withdraw from local bot wallet first.
	tx, err := h.storage.WithdrawFromBotWallet(ctx, botID, req.AmountAiCoin, req.AmountUsdCents, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to withdraw from wallet: " + err.Error()})
		return
	}

	// Deposit the withdrawn USD back to the user's Commerce balance.
	// Reverse of fund: service account → withdraw, user → deposit.
	// Both use the service token (admin) since we're moving between accounts.
	if req.AmountUsdCents > 0 {
		userID, _ := jwtUserID(c.GetHeader("Authorization"))
		if userID != "" {
			// Withdraw from service account (uses service token via commerceDeposit's sibling pattern).
			svcWithdrawNotes := fmt.Sprintf("Bot wallet withdraw refund: %s → %s", botID, userID)
			if wErr := h.commerceServiceWithdraw(ctx, botServiceAccount, req.AmountUsdCents, svcWithdrawNotes); wErr != nil {
				fmt.Printf("[WARN] bot wallet withdraw: service account debit failed: %v\n", wErr)
			}
			// Deposit back to user's balance.
			depositNotes := fmt.Sprintf("Bot wallet withdraw refund from %s", botID)
			if dErr := h.commerceDeposit(ctx, userID, req.AmountUsdCents, depositNotes); dErr != nil {
				fmt.Printf("[WARN] bot wallet withdraw: user deposit failed: %v\n", dErr)
			}
		}
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
	if err != nil || transactions == nil {
		// Return empty list on any error (table may not exist yet)
		c.JSON(http.StatusOK, []*storage.WalletTransaction{})
		return
	}

	c.JSON(http.StatusOK, transactions)
}

// GetUsage returns LLM usage (token costs) from the bot service account.
// This shows what cloud-api has charged for bot chat completions.
// GET /api/v1/:botId/wallet/usage?limit=50
func (h *WalletHandler) GetUsage(c *gin.Context) {
	limit := "50"
	if l := c.Query("limit"); l != "" {
		limit = l
	}

	targetURL := fmt.Sprintf("%s/api/v1/billing/transactions?user=%s&limit=%s",
		h.billing.commerceURL, botServiceAccount, limit)

	reqCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "GET", targetURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to build request"})
		return
	}
	if h.billing.serviceToken != "" {
		req.Header.Set("Authorization", "Bearer "+h.billing.serviceToken)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := h.billing.client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "commerce unavailable"})
		return
	}
	defer resp.Body.Close()

	// Forward the response directly.
	for k, v := range resp.Header {
		for _, val := range v {
			c.Writer.Header().Add(k, val)
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(c.Writer, resp.Body)
}

// ListAutoPurchaseRules returns all auto-purchase rules for a bot.
// GET /api/v1/:botId/wallet/auto-purchase
func (h *WalletHandler) ListAutoPurchaseRules(c *gin.Context) {
	ctx := c.Request.Context()
	botID := c.Param("botId")

	rules, err := h.storage.GetAutoPurchaseRules(ctx, botID)
	if err != nil || rules == nil {
		c.JSON(http.StatusOK, []*storage.AutoPurchaseRule{})
		return
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
	TotalBots     int     `json:"total_bots"`
	TotalAiCoin   float64 `json:"total_ai_coin"`
	TotalUsdCents int64   `json:"total_usd_cents"`
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
