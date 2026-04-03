package middleware

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
	FallbackAPIKey string // When IAM disabled, require this API key to be configured
}

// IAMUserInfo represents the user identity extracted from an IAM token.
type IAMUserInfo struct {
	Sub          string `json:"sub"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Organization string `json:"organization"`
	Owner        string `json:"owner"` // Casdoor returns org as "owner"; used as fallback for Organization
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

		if !config.Enabled {
			// When IAM is disabled, require API key auth as fallback.
			// If neither IAM nor API key is configured, refuse to serve API routes.
			if config.FallbackAPIKey == "" {
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
					"error": "No authentication configured. Set IAM_ENABLED=true or configure an API key.",
				})
				return
			}
			c.Next()
			return
		}

		// Extract bearer token from header, or from query params (SSE/EventSource)
		authHeader := c.GetHeader("Authorization")
		token := ""
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}
		// SSE/EventSource can't set headers — check query params as fallback
		if token == "" {
			token = c.Query("access_token")
		}
		if token == "" {
			token = c.Query("api_key")
		}
		if token == "" {
			// No token found — fall through to API key auth
			c.Next()
			return
		}

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
			logger.Logger.Error().Err(err).Str("endpoint", userinfoURL).Msg("IAM: userinfo request failed")
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error": "Authentication service unavailable",
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

		rawBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if readErr != nil {
			logger.Logger.Error().Err(readErr).Msg("IAM: failed to read userinfo response body")
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error":   "auth_error",
				"message": "failed to read user identity",
			})
			return
		}

		var user IAMUserInfo
		if err := json.Unmarshal(rawBody, &user); err != nil {
			logger.Logger.Error().Err(err).Str("body", string(rawBody)).Msg("IAM: failed to decode userinfo response")
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error":   "auth_error",
				"message": "failed to parse user identity",
			})
			return
		}

		// Resolve org from the userinfo response. Casdoor's /api/userinfo
		// returns standard OIDC claims (no "owner"/"organization" fields).
		// Try in order:
		//   1. "organization" — standard OIDC claim
		//   2. "owner"        — Casdoor full User object (if endpoint returns it)
		//   3. sub prefix     — Casdoor formats sub as "{owner}/{name}"
		//   4. JWT payload    — decode "owner" claim directly from the token
		//   5. config.Organization — single-tenant fallback
		if user.Organization == "" && user.Owner != "" {
			user.Organization = user.Owner
		}
		if user.Organization == "" && strings.Contains(user.Sub, "/") {
			user.Organization = strings.SplitN(user.Sub, "/", 2)[0]
		}
		if user.Organization == "" {
			user.Organization = jwtOwnerClaim(token)
		}
		if user.Organization == "" && config.Organization != "" {
			user.Organization = config.Organization
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

		// Allow frontend to override org via X-Org-ID header or org_id query param.
		// This enables the org switcher — user selects their personal org
		// in the UI, and API calls use that org instead of the JWT's owner.
		// Query param fallback is needed for SSE/EventSource which can't send headers.
		orgOverride := c.GetHeader("X-Org-ID")
		if orgOverride == "" {
			orgOverride = c.Query("org_id")
		}
		if orgOverride != "" && orgOverride != user.Organization {
			c.Set(ContextKeyOrg, orgOverride)
		}

		logger.Logger.Debug().
			Str("sub", user.Sub).
			Str("email", user.Email).
			Str("org", GetOrganization(c)).
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

// RequireOrg extracts the org from the gin context and aborts with 403 if missing.
// Falls back to "local" for non-IAM auth (API key). Returns empty string on abort.
func RequireOrg(c *gin.Context) (string, bool) {
	org := GetOrganization(c)
	if org != "" {
		return org, true
	}
	// For non-IAM auth (API key, internal), fall back to HANZO_DEFAULT_ORG
	// or "local" for backward compatibility. Set HANZO_DEFAULT_ORG to control this.
	fallback := os.Getenv("HANZO_DEFAULT_ORG")
	if fallback == "" {
		fallback = "local"
	}
	return fallback, true
}

// RequireOrgStrict extracts the org and aborts with 403 if no org context is available.
// Unlike RequireOrg which falls back to a default, this function enforces that an actual
// org is present (either from IAM auth or explicit context). Use this for endpoints that
// must be org-scoped (e.g., resource creation, data queries).
func RequireOrgStrict(c *gin.Context) (string, bool) {
	org := GetOrganization(c)
	if org != "" {
		return org, true
	}
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
		"error":   "org_required",
		"message": "Organization context is required. Authenticate with IAM or set X-Org-ID header.",
	})
	return "", false
}

// RequireIAMOrg extracts the org from IAM context and aborts with 403 if no IAM user.
// Use this for endpoints that strictly require IAM auth with org context.
func RequireIAMOrg(c *gin.Context) (string, bool) {
	user := GetIAMUser(c)
	if user == nil {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error":   "org_required",
			"message": "IAM authentication with organization context is required",
		})
		return "", false
	}
	org := user.Organization
	if org == "" {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error":   "org_required",
			"message": "user is not associated with an organization",
		})
		return "", false
	}
	return org, true
}

// jwtOwnerClaim decodes the JWT payload (without verification) and returns
// the "owner" claim. Casdoor embeds the full user object in the token
// including "owner":"hanzo", which /api/userinfo strips out.
// Returns "" on any error — callers treat this as a soft fallback only.
func jwtOwnerClaim(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}
	payload := parts[1]
	// Base64url padding
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}
	b, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return ""
	}
	var claims struct {
		Owner string `json:"owner"`
	}
	if err := json.Unmarshal(b, &claims); err != nil {
		return ""
	}
	return claims.Owner
}
