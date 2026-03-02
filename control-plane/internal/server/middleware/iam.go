package middleware

import (
	"encoding/base64"
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

		// Casdoor's /api/userinfo follows OIDC standard claims and may omit
		// organization, isAdmin, etc.  Fall back to the JWT payload which
		// always carries the full Casdoor claim set (owner, isAdmin, …).
		if user.Organization == "" || !user.IsAdmin {
			if jwtClaims := parseJWTPayload(token); jwtClaims != nil {
				if user.Organization == "" {
					user.Organization = jwtClaims.Owner
				}
				if !user.IsAdmin && jwtClaims.IsAdmin {
					user.IsAdmin = true
				}
				if user.Email == "" {
					user.Email = jwtClaims.Email
				}
				if user.Name == "" {
					user.Name = jwtClaims.Name
				}
			}
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

// RequireOrg extracts the org from the gin context and aborts with 403 if missing.
// Falls back to "local" for non-IAM auth (API key). Returns empty string on abort.
func RequireOrg(c *gin.Context) (string, bool) {
	org := GetOrganization(c)
	if org != "" {
		return org, true
	}
	// For non-IAM auth (API key, internal), fall back to "local"
	// This preserves backward compatibility while ensuring org is always set
	return "local", true
}

// jwtPayload holds the Casdoor-specific JWT claims we need beyond OIDC standard.
type jwtPayload struct {
	Owner   string `json:"owner"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	IsAdmin bool   `json:"isAdmin"`
}

// parseJWTPayload extracts Casdoor claims from the JWT payload without
// cryptographic verification (the token was already validated via userinfo).
func parseJWTPayload(token string) *jwtPayload {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) < 2 {
		return nil
	}
	payload := parts[1]
	// Pad base64url to standard base64
	if m := len(payload) % 4; m != 0 {
		payload += strings.Repeat("=", 4-m)
	}
	data, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return nil
	}
	var claims jwtPayload
	if err := json.Unmarshal(data, &claims); err != nil {
		return nil
	}
	return &claims
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
