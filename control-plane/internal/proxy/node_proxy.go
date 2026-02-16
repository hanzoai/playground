// Package proxy provides HTTP reverse proxy to hanzo/node V2 API endpoints.
// The UI never talks to nodes directly — all requests go through the control plane.
package proxy

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/control-plane/internal/spaces"
)

// NodeProxy proxies requests from the Space API to individual hanzo/node V2 endpoints.
type NodeProxy struct {
	store  spaces.Store
	client *http.Client
}

// NewNodeProxy creates a new proxy handler.
func NewNodeProxy(store spaces.Store) *NodeProxy {
	return &NodeProxy{
		store: store,
		client: &http.Client{
			Timeout: 120 * time.Second,
			// No automatic redirects — forward everything
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// ProxyToNodeV2 handles ANY /api/v1/spaces/:id/nodes/:nid/v2/*path.
// It looks up the node's endpoint from the space store and forwards the request.
func (p *NodeProxy) ProxyToNodeV2(c *gin.Context) {
	spaceID := c.Param("id")
	nodeID := c.Param("nid")
	path := c.Param("path") // everything after /v2/

	node, err := p.store.GetNode(c.Request.Context(), spaceID, nodeID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "node not found",
			"details": err.Error(),
		})
		return
	}

	if node.Endpoint == "" {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": "node has no endpoint configured",
		})
		return
	}

	// Build target URL
	targetURL := fmt.Sprintf("%s/v2/%s", strings.TrimRight(node.Endpoint, "/"), path)

	// Add query string if present
	if c.Request.URL.RawQuery != "" {
		targetURL += "?" + c.Request.URL.RawQuery
	}

	// Create proxy request
	proxyReq, err := http.NewRequestWithContext(
		c.Request.Context(),
		c.Request.Method,
		targetURL,
		c.Request.Body,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("proxy request error: %v", err)})
		return
	}

	// Forward relevant headers
	for _, header := range []string{"Content-Type", "Authorization", "Accept", "X-Request-ID"} {
		if val := c.GetHeader(header); val != "" {
			proxyReq.Header.Set(header, val)
		}
	}

	// Execute proxy request
	resp, err := p.client.Do(proxyReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "node unreachable",
			"details": err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	// Forward response headers
	for key, vals := range resp.Header {
		for _, val := range vals {
			c.Writer.Header().Add(key, val)
		}
	}

	// Stream response back to client
	c.Status(resp.StatusCode)
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		// Connection may have been closed by client, that's fine
		return
	}
}
