package middleware

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/control-plane/internal/logger"
)

// IAMConfig holds IAM authentication middleware settings.
type IAMConfig struct {
	Enabled        bool
	Endpoint       string // Internal IAM endpoint (e.g., http://iam.hanzo.svc:8000)
	PublicEndpoint string // Public IAM endpoint (e.g., https://hanzo.id)
	ClientID       string
	ClientSecret   string
	Organization   string
	Application    string
	SkipPaths      []string
}

// IAMUserInfo represents the user identity extracted from an IAM token.
type IAMUserInfo struct {
	Sub          string `json:"sub"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Organization string `json:"organization"`
	IsAdmin      bool   `json:"isAdmin"`
	Type         string `json:"type"` // "user" or "application"
}

const (
	// Context keys for downstream handlers.
	ContextKeyUser = "iam_user"
	ContextKeyOrg  = "iam_org"
)

// tokenCache provides simple TTL caching for validated tokens.
type tokenCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
}

type cacheEntry struct {
	user      *IAMUserInfo
	expiresAt time.Time
}

var cache = &tokenCache{entries: make(map[string]*cacheEntry)}

const cacheTTL = 60 * time.Second

func (tc *tokenCache) get(token string) (*IAMUserInfo, bool) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	entry, ok := tc.entries[token]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.user, true
}

func (tc *tokenCache) set(token string, user *IAMUserInfo) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.entries[token] = &cacheEntry{
		user:      user,
		expiresAt: time.Now().Add(cacheTTL),
	}
}

// IAMAuth validates bearer tokens against the Hanzo IAM userinfo endpoint.
// If IAM is disabled or no bearer token is present, it falls through to the
// next middleware (allowing API key auth to handle it).
func IAMAuth(config IAMConfig) gin.HandlerFunc {
	skipPathSet := make(map[string]struct{}, len(config.SkipPaths))
	for _, p := range config.SkipPaths {
		skipPathSet[p] = struct{}{}
	}

	client := &http.Client{Timeout: 5 * time.Second}

	return func(c *gin.Context) {
		if !config.Enabled {
			c.Next()
			return
		}

		// Skip explicit paths
		if _, ok := skipPathSet[c.Request.URL.Path]; ok {
			c.Next()
			return
		}

		// Always allow health, metrics, UI
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/v1/health") || strings.HasPrefix(path, "/api/v1/health") || path == "/health" || path == "/metrics" {
			c.Next()
			return
		}
		// Allow UI static files and SPA routes (everything not under /v1/ or /api/)
		if !strings.HasPrefix(path, "/v1/") && !strings.HasPrefix(path, "/api/") {
			c.Next()
			return
		}

		// Extract bearer token
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			// No bearer token — fall through to API key auth
			c.Next()
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")

		// Check if this looks like an API key (short, no dots) vs IAM token (JWT-like)
		// API keys are typically short hex strings; IAM tokens are JWTs with dots
		if !strings.Contains(token, ".") {
			// Likely an API key, let the API key middleware handle it
			c.Next()
			return
		}

		// Check cache
		if user, ok := cache.get(token); ok {
			c.Set(ContextKeyUser, user)
			c.Set(ContextKeyOrg, user.Organization)
			c.Next()
			return
		}

		// Validate against IAM userinfo endpoint
		userinfoURL := fmt.Sprintf("%s/api/userinfo", config.Endpoint)
		req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, userinfoURL, nil)
		if err != nil {
			logger.Logger.Error().Err(err).Msg("IAM: failed to create userinfo request")
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error":   "auth_error",
				"message": "failed to validate token",
			})
			return
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)
		if err != nil {
			logger.Logger.Warn().Err(err).Str("endpoint", userinfoURL).Msg("IAM: userinfo request failed")
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error":   "iam_unavailable",
				"message": "IAM service is unreachable",
			})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			logger.Logger.Warn().
				Int("status", resp.StatusCode).
				Str("body", string(body)).
				Msg("IAM: token validation failed")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "invalid or expired IAM token",
			})
			return
		}

		var user IAMUserInfo
		if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
			logger.Logger.Error().Err(err).Msg("IAM: failed to decode userinfo response")
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error":   "auth_error",
				"message": "failed to parse user identity",
			})
			return
		}

		// Enforce organization match if configured
		if config.Organization != "" && user.Organization != "" && user.Organization != config.Organization {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "user does not belong to the required organization",
			})
			return
		}

		// Cache the result
		cache.set(token, &user)

		// Set user context for downstream handlers
		c.Set(ContextKeyUser, &user)
		c.Set(ContextKeyOrg, user.Organization)

		logger.Logger.Debug().
			Str("sub", user.Sub).
			Str("email", user.Email).
			Str("org", user.Organization).
			Msg("IAM: token validated")

		c.Next()
	}
}

// GetIAMUser extracts the IAM user from the gin context (nil if not IAM-authed).
func GetIAMUser(c *gin.Context) *IAMUserInfo {
	if user, exists := c.Get(ContextKeyUser); exists {
		if u, ok := user.(*IAMUserInfo); ok {
			return u
		}
	}
	return nil
}

// GetOrganization extracts the org from the gin context.
func GetOrganization(c *gin.Context) string {
	if org, exists := c.Get(ContextKeyOrg); exists {
		if o, ok := org.(string); ok {
			return o
		}
	}
	return ""
}
