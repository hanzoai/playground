package billing

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/control-plane/internal/logger"
	"github.com/hanzoai/playground/control-plane/internal/server/middleware"
)

// GateMode controls how billing errors are handled.
type GateMode string

const (
	// GateModeOpen always allows requests (development/testing).
	GateModeOpen GateMode = "open"
	// GateModeWarn allows requests on billing errors but logs warnings (staging).
	GateModeWarn GateMode = "warn"
	// GateModeFailClosed denies requests when billing is unavailable (production default).
	GateModeFailClosed GateMode = "fail-closed"
)

// LLMGateConfig configures the LLM billing gate middleware.
type LLMGateConfig struct {
	// Mode controls error behavior: open, warn, or fail-closed.
	Mode GateMode
	// Client is the Commerce API client for balance checks.
	Client *Client
	// Cache is the balance cache (optional; created if nil).
	Cache *balanceCache
}

// LLMGateResult is the outcome of a billing pre-check for LLM requests.
type LLMGateResult struct {
	Allowed       bool    `json:"allowed"`
	Reason        string  `json:"reason,omitempty"`
	BalanceCents  float64 `json:"balance_cents,omitempty"`
	UserID        string  `json:"user_id,omitempty"`
}

// GateContextKey is the gin context key where the LLM gate result is stored.
const GateContextKey = "billing_llm_gate"

// GetLLMGateResult extracts the gate result from the gin context.
func GetLLMGateResult(c *gin.Context) *LLMGateResult {
	if v, exists := c.Get(GateContextKey); exists {
		if r, ok := v.(*LLMGateResult); ok {
			return r
		}
	}
	return nil
}

// resolveGateMode reads BILLING_GATE_MODE from environment and maps to GateMode.
func resolveGateMode() GateMode {
	switch strings.ToLower(os.Getenv("BILLING_GATE_MODE")) {
	case "open":
		return GateModeOpen
	case "warn":
		return GateModeWarn
	case "fail-closed", "fail_closed", "closed":
		return GateModeFailClosed
	default:
		return GateModeFailClosed
	}
}

// NewLLMGateConfig creates an LLMGateConfig from environment variables.
func NewLLMGateConfig() LLMGateConfig {
	return LLMGateConfig{
		Mode:   resolveGateMode(),
		Client: NewClient(),
		Cache:  newBalanceCache(defaultBalanceCacheTTL),
	}
}

// LLMBillingGate is a Gin middleware that checks the user's Commerce balance
// before allowing an LLM completion request. The check is skipped when IAM
// is not present (personal/self-hosted mode) or when GateMode is "open".
//
// On success, it stores the LLMGateResult in the gin context for downstream
// handlers (e.g. to determine the user's billing ID for usage reporting).
func LLMBillingGate(cfg LLMGateConfig) gin.HandlerFunc {
	if cfg.Client == nil {
		cfg.Client = NewClient()
	}
	if cfg.Cache == nil {
		cfg.Cache = newBalanceCache(defaultBalanceCacheTTL)
	}

	return func(c *gin.Context) {
		// Open mode — always allow.
		if cfg.Mode == GateModeOpen {
			c.Set(GateContextKey, &LLMGateResult{Allowed: true})
			c.Next()
			return
		}

		// No IAM user — personal/self-hosted mode, no billing.
		user := middleware.GetIAMUser(c)
		if user == nil {
			c.Set(GateContextKey, &LLMGateResult{Allowed: true})
			c.Next()
			return
		}

		// Build billing user ID: "org/loginName" to match Commerce convention.
		billingUserID := user.Organization + "/" + user.LoginName
		bearerToken := extractBearerToken(c)

		result, err := checkLLMBalance(c.Request.Context(), cfg.Client, cfg.Cache, billingUserID, bearerToken)
		if err != nil {
			logger.Logger.Error().Err(err).Str("user", billingUserID).Msg("LLM billing gate: Commerce unreachable")

			if cfg.Mode == GateModeWarn {
				logger.Logger.Warn().Str("user", billingUserID).Msg("LLM billing gate: allowing in warn mode despite Commerce error")
				c.Set(GateContextKey, &LLMGateResult{Allowed: true, UserID: billingUserID})
				c.Next()
				return
			}

			// fail-closed
			c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{
				"error":   "billing_unavailable",
				"message": "Billing service is temporarily unavailable. Please try again.",
			})
			return
		}

		if !result.Allowed {
			c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{
				"error":         "insufficient_funds",
				"message":       result.Reason,
				"balance_cents": result.BalanceCents,
			})
			return
		}

		c.Set(GateContextKey, result)
		c.Next()
	}
}

// checkLLMBalance performs the actual balance check against Commerce,
// using the cache to avoid redundant calls.
func checkLLMBalance(
	ctx context.Context,
	client *Client,
	cache *balanceCache,
	userID string,
	token string,
) (*LLMGateResult, error) {
	// Check cache first.
	cacheKey := userID
	if cached, ok := cache.get(cacheKey); ok {
		available := cached.Available
		if available > 0 {
			return &LLMGateResult{
				Allowed:      true,
				BalanceCents: available,
				UserID:       userID,
			}, nil
		}
		return &LLMGateResult{
			Allowed:      false,
			Reason:       fmt.Sprintf("Insufficient funds. Balance: $%.2f. Add credits at https://billing.hanzo.ai", available/100.0),
			BalanceCents: available,
			UserID:       userID,
		}, nil
	}

	// Fetch from Commerce.
	balance, err := client.GetBalance(ctx, userID, token)
	if err != nil {
		return nil, err
	}

	// Cache the result.
	cache.set(cacheKey, balance)

	if balance.Available > 0 {
		return &LLMGateResult{
			Allowed:      true,
			BalanceCents: balance.Available,
			UserID:       userID,
		}, nil
	}

	return &LLMGateResult{
		Allowed:      false,
		Reason:       fmt.Sprintf("Insufficient funds. Balance: $%.2f. Add credits at https://billing.hanzo.ai", balance.Available/100.0),
		BalanceCents: balance.Available,
		UserID:       userID,
	}, nil
}

// extractBearerToken pulls the Bearer token from the Authorization header.
func extractBearerToken(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}
