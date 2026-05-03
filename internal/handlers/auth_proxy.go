package handlers

import (
	"bytes"
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
	// FallbackTokenEndpoint is used when the primary TokenEndpoint is unreachable.
	// Typically the public endpoint (e.g. "https://hanzo.id/oauth/token").
	FallbackTokenEndpoint string
	// FallbackUserinfoEndpoint is used when the primary UserinfoEndpoint is unreachable.
	FallbackUserinfoEndpoint string
}

var proxyClient = &http.Client{Timeout: 15 * time.Second}

// doTokenRequest sends a POST to the given endpoint with the provided body.
// Returns the response or an error.
func doTokenRequest(ctx *gin.Context, endpoint string, body []byte, contentType string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx.Request.Context(), http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")
	return proxyClient.Do(req)
}

// AuthTokenProxyHandler proxies OAuth token exchange requests to the IAM server.
// This avoids browser CORS issues when the IAM server doesn't send
// Access-Control-Allow-Origin headers.
//
// The frontend POSTs form-encoded token requests to /auth/token, and this
// handler forwards them server-to-server to the IAM token endpoint.
// If the primary (internal) endpoint is unreachable, falls back to the
// public endpoint to ensure login works even when in-cluster IAM is down.
func AuthTokenProxyHandler(cfg AuthProxyConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.TokenEndpoint == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "IAM token endpoint not configured"})
			return
		}

		// Buffer the request body so we can retry with the fallback endpoint.
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)) // 1MB limit
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
			return
		}
		contentType := c.GetHeader("Content-Type")

		// Try primary endpoint first.
		resp, err := doTokenRequest(c, cfg.TokenEndpoint, body, contentType)
		if err != nil {
			log.Printf("[auth-proxy] token endpoint %s error: %v", cfg.TokenEndpoint, err)

			// Fallback to public endpoint if configured and different.
			if cfg.FallbackTokenEndpoint != "" && cfg.FallbackTokenEndpoint != cfg.TokenEndpoint {
				log.Printf("[auth-proxy] falling back to public token endpoint: %s", cfg.FallbackTokenEndpoint)
				resp, err = doTokenRequest(c, cfg.FallbackTokenEndpoint, body, contentType)
				if err != nil {
					log.Printf("[auth-proxy] fallback token endpoint %s also failed: %v", cfg.FallbackTokenEndpoint, err)
					c.JSON(http.StatusBadGateway, gin.H{"error": "IAM server unreachable (both internal and public)"})
					return
				}
				// Fallback succeeded — fall through to stream response
			} else {
				c.JSON(http.StatusBadGateway, gin.H{"error": "IAM server unreachable"})
				return
			}
		}
		defer resp.Body.Close()

		// Stream the response back with the same status code and content type.
		c.Status(resp.StatusCode)
		c.Header("Content-Type", resp.Header.Get("Content-Type"))
		_, _ = io.Copy(c.Writer, resp.Body)
	}
}

// AuthUserinfoProxyHandler proxies OAuth userinfo requests to the IAM server.
// Falls back to the public endpoint if the internal one is unreachable.
func AuthUserinfoProxyHandler(cfg AuthProxyConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.UserinfoEndpoint == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "IAM userinfo endpoint not configured"})
			return
		}

		doUserinfo := func(endpoint string) (*http.Response, error) {
			req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, endpoint, nil)
			if err != nil {
				return nil, err
			}
			if auth := c.GetHeader("Authorization"); auth != "" {
				req.Header.Set("Authorization", auth)
			}
			req.Header.Set("Accept", "application/json")
			return proxyClient.Do(req)
		}

		// Try primary endpoint first.
		resp, err := doUserinfo(cfg.UserinfoEndpoint)
		if err != nil {
			log.Printf("[auth-proxy] userinfo endpoint %s error: %v", cfg.UserinfoEndpoint, err)

			// Fallback to public endpoint if configured and different.
			if cfg.FallbackUserinfoEndpoint != "" && cfg.FallbackUserinfoEndpoint != cfg.UserinfoEndpoint {
				log.Printf("[auth-proxy] falling back to public userinfo endpoint: %s", cfg.FallbackUserinfoEndpoint)
				resp, err = doUserinfo(cfg.FallbackUserinfoEndpoint)
				if err != nil {
					log.Printf("[auth-proxy] fallback userinfo endpoint %s also failed: %v", cfg.FallbackUserinfoEndpoint, err)
					c.JSON(http.StatusBadGateway, gin.H{"error": "IAM server unreachable (both internal and public)"})
					return
				}
			} else {
				c.JSON(http.StatusBadGateway, gin.H{"error": "IAM server unreachable"})
				return
			}
		}
		defer resp.Body.Close()

		c.Status(resp.StatusCode)
		c.Header("Content-Type", resp.Header.Get("Content-Type"))
		_, _ = io.Copy(c.Writer, resp.Body)
	}
}
