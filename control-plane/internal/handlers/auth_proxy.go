package handlers

import (
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// AuthProxyConfig holds the upstream IAM endpoints for the OAuth proxy.
type AuthProxyConfig struct {
	// TokenEndpoint is the IAM token endpoint (e.g. "http://iam.hanzo.svc:8000/oauth/token"
	// or "https://hanzo.id/oauth/token").
	TokenEndpoint string
	// UserinfoEndpoint is the IAM userinfo endpoint.
	UserinfoEndpoint string
}

var proxyClient = &http.Client{Timeout: 15 * time.Second}

// AuthTokenProxyHandler proxies OAuth token exchange requests to the IAM server.
// This avoids browser CORS issues when the IAM server doesn't send
// Access-Control-Allow-Origin headers.
//
// The frontend POSTs form-encoded token requests to /auth/token, and this
// handler forwards them server-to-server to the IAM token endpoint.
func AuthTokenProxyHandler(cfg AuthProxyConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.TokenEndpoint == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "IAM token endpoint not configured"})
			return
		}

		// Forward the body as-is to the upstream token endpoint.
		req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, cfg.TokenEndpoint, c.Request.Body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create proxy request"})
			return
		}
		req.Header.Set("Content-Type", c.GetHeader("Content-Type"))
		req.Header.Set("Accept", "application/json")

		resp, err := proxyClient.Do(req)
		if err != nil {
			log.Printf("[auth-proxy] token endpoint %s error: %v", cfg.TokenEndpoint, err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "IAM server unreachable"})
			return
		}
		defer resp.Body.Close()

		// Stream the response back with the same status code and content type.
		c.Status(resp.StatusCode)
		c.Header("Content-Type", resp.Header.Get("Content-Type"))
		_, _ = io.Copy(c.Writer, resp.Body)
	}
}

// AuthUserinfoProxyHandler proxies OAuth userinfo requests to the IAM server.
func AuthUserinfoProxyHandler(cfg AuthProxyConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.UserinfoEndpoint == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "IAM userinfo endpoint not configured"})
			return
		}

		req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, cfg.UserinfoEndpoint, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create proxy request"})
			return
		}
		// Forward the Authorization header from the browser.
		if auth := c.GetHeader("Authorization"); auth != "" {
			req.Header.Set("Authorization", auth)
		}
		req.Header.Set("Accept", "application/json")

		resp, err := proxyClient.Do(req)
		if err != nil {
			log.Printf("[auth-proxy] userinfo endpoint %s error: %v", cfg.UserinfoEndpoint, err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "IAM server unreachable"})
			return
		}
		defer resp.Body.Close()

		c.Status(resp.StatusCode)
		c.Header("Content-Type", resp.Header.Get("Content-Type"))
		_, _ = io.Copy(c.Writer, resp.Body)
	}
}
