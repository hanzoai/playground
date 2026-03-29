package ui

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// BillingProxyHandler proxies billing API calls to the Commerce backend.
// This avoids CORS issues and centralizes authentication with the service token.
type BillingProxyHandler struct {
	commerceURL   string
	serviceToken  string
	client        *http.Client
}

// NewBillingProxyHandler creates a new BillingProxyHandler.
// Reads COMMERCE_API_URL and COMMERCE_SERVICE_TOKEN from environment.
func NewBillingProxyHandler() *BillingProxyHandler {
	url := os.Getenv("COMMERCE_API_URL")
	if url == "" {
		url = "http://commerce.hanzo.svc.cluster.local:8001"
	}
	url = strings.TrimRight(url, "/")

	token := os.Getenv("COMMERCE_SERVICE_TOKEN")

	return &BillingProxyHandler{
		commerceURL:  url,
		serviceToken: token,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// ProxyGet handles GET requests by forwarding to Commerce.
// GET /v1/billing/*path
func (h *BillingProxyHandler) ProxyGet(c *gin.Context) {
	subPath := c.Param("path")
	if subPath == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "path is required"})
		return
	}

	targetURL := fmt.Sprintf("%s/api/v1/billing%s", h.commerceURL, subPath)

	// Forward query parameters
	if qs := c.Request.URL.RawQuery; qs != "" {
		targetURL += "?" + qs
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), "GET", targetURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to create request"})
		return
	}

	h.setHeaders(req, c)

	resp, err := h.client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "commerce service unavailable"})
		return
	}
	defer resp.Body.Close()

	// Forward response headers
	for k, v := range resp.Header {
		for _, val := range v {
			c.Writer.Header().Add(k, val)
		}
	}

	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(c.Writer, resp.Body)
}

// ProxyPost handles POST requests by forwarding to Commerce.
// POST /v1/billing/*path
func (h *BillingProxyHandler) ProxyPost(c *gin.Context) {
	subPath := c.Param("path")
	if subPath == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "path is required"})
		return
	}

	targetURL := fmt.Sprintf("%s/api/v1/billing%s", h.commerceURL, subPath)

	req, err := http.NewRequestWithContext(c.Request.Context(), "POST", targetURL, c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to create request"})
		return
	}

	h.setHeaders(req, c)
	if ct := c.GetHeader("Content-Type"); ct != "" {
		req.Header.Set("Content-Type", ct)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "commerce service unavailable"})
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		for _, val := range v {
			c.Writer.Header().Add(k, val)
		}
	}

	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(c.Writer, resp.Body)
}

// setHeaders adds auth and forwarding headers to the Commerce request.
func (h *BillingProxyHandler) setHeaders(req *http.Request, c *gin.Context) {
	// Use service token for Commerce auth
	if h.serviceToken != "" {
		req.Header.Set("Authorization", "Bearer "+h.serviceToken)
	}

	// Forward user identity from the incoming request
	if userAuth := c.GetHeader("Authorization"); userAuth != "" {
		req.Header.Set("X-Original-Authorization", userAuth)
	}

	req.Header.Set("Accept", "application/json")
}
