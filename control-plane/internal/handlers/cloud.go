package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/control-plane/internal/cloud"
	"github.com/hanzoai/playground/control-plane/internal/logger"
	"github.com/hanzoai/playground/control-plane/internal/server/middleware"
)

var pricingHTTPClient = &http.Client{Timeout: 5 * time.Second}

// CloudProvisionHandler creates a new cloud agent in the DOKS cluster.
func CloudProvisionHandler(provisioner *cloud.Provisioner) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req cloud.ProvisionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_request",
				"message": err.Error(),
			})
			return
		}

		// Inject IAM user info if available
		if user := middleware.GetIAMUser(c); user != nil {
			req.Owner = user.Sub
			if req.Org == "" {
				req.Org = user.Organization
			}
		}
		if org := middleware.GetOrganization(c); org != "" && req.Org == "" {
			req.Org = org
		}

		// Pass the user's bearer token for per-user billing.
		// Cloud bots will use this as HANZO_API_KEY so LLM usage
		// is tracked and billed to the launching user's account.
		if auth := c.GetHeader("Authorization"); auth != "" {
			token := strings.TrimPrefix(auth, "Bearer ")
			if token != auth { // had Bearer prefix
				req.UserAPIKey = token
			}
		}
		if req.UserAPIKey == "" {
			if key := c.GetHeader("X-API-Key"); key != "" {
				req.UserAPIKey = key
			}
		}

		result, err := provisioner.Provision(c.Request.Context(), &req)
		if err != nil {
			logger.Logger.Error().Err(err).Str("node_id", req.NodeID).Msg("cloud provision failed")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "provision_failed",
				"message": err.Error(),
			})
			return
		}

		c.JSON(http.StatusCreated, result)
	}
}

// CloudDeprovisionHandler removes a cloud agent from the cluster.
func CloudDeprovisionHandler(provisioner *cloud.Provisioner) gin.HandlerFunc {
	return func(c *gin.Context) {
		nodeID := c.Param("node_id")
		if nodeID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_request",
				"message": "node_id is required",
			})
			return
		}

		if err := provisioner.Deprovision(c.Request.Context(), nodeID); err != nil {
			logger.Logger.Error().Err(err).Str("node_id", nodeID).Msg("cloud deprovision failed")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "deprovision_failed",
				"message": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "deprovisioned",
			"node_id": nodeID,
		})
	}
}

// CloudListNodesHandler lists all cloud agent nodes.
func CloudListNodesHandler(provisioner *cloud.Provisioner) gin.HandlerFunc {
	return func(c *gin.Context) {
		org := c.Query("org")

		// If IAM authenticated, scope to user's org
		if user := middleware.GetIAMUser(c); user != nil && org == "" {
			org = user.Organization
		}

		nodes, err := provisioner.ListNodes(c.Request.Context(), org)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "list_failed",
				"message": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"nodes": nodes,
			"count": len(nodes),
		})
	}
}

// CloudGetNodeHandler returns info about a specific cloud node.
func CloudGetNodeHandler(provisioner *cloud.Provisioner) gin.HandlerFunc {
	return func(c *gin.Context) {
		nodeID := c.Param("node_id")
		if nodeID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_request",
				"message": "node_id is required",
			})
			return
		}

		node, err := provisioner.GetNode(c.Request.Context(), nodeID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "not_found",
				"message": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, node)
	}
}

// CloudGetLogsHandler returns recent logs for a cloud agent.
func CloudGetLogsHandler(provisioner *cloud.Provisioner) gin.HandlerFunc {
	return func(c *gin.Context) {
		nodeID := c.Param("node_id")
		if nodeID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_request",
				"message": "node_id is required",
			})
			return
		}

		tailLines := int64(100)
		if tl := c.Query("tail"); tl != "" {
			// Simple parse, ignore errors
			var n int64
			for _, ch := range tl {
				if ch >= '0' && ch <= '9' {
					n = n*10 + int64(ch-'0')
				}
			}
			if n > 0 && n <= 10000 {
				tailLines = n
			}
		}

		logs, err := provisioner.GetLogs(c.Request.Context(), nodeID, tailLines)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "logs_failed",
				"message": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"node_id": nodeID,
			"logs":    logs,
		})
	}
}

// CloudSyncHandler refreshes the in-memory cloud node list from K8s.
func CloudSyncHandler(provisioner *cloud.Provisioner) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := provisioner.Sync(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "sync_failed",
				"message": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "synced"})
	}
}

// CloudPricingHandler returns available droplet sizes and their hourly cost in cents.
// Fetches from the centralized pricing service; falls back to hardcoded tiers if unreachable.
func CloudPricingHandler(pricingURL string) gin.HandlerFunc {
	type fallbackTier struct {
		Slug     string `json:"slug"`
		VCPUs    int    `json:"vcpus"`
		MemoryMB int    `json:"memoryMB"`
		DiskGB   int    `json:"diskGB"`
		CentsHr  int    `json:"centsPerHour"`
	}

	fallback := gin.H{
		"provider": "digitalocean",
		"region":   "sfo3",
		"tiers": []fallbackTier{
			{Slug: "s-1vcpu-1gb", VCPUs: 1, MemoryMB: 1024, DiskGB: 25, CentsHr: 1},
			{Slug: "s-1vcpu-2gb", VCPUs: 1, MemoryMB: 2048, DiskGB: 50, CentsHr: 2},
			{Slug: "s-2vcpu-2gb", VCPUs: 2, MemoryMB: 2048, DiskGB: 60, CentsHr: 3},
			{Slug: "s-2vcpu-4gb", VCPUs: 2, MemoryMB: 4096, DiskGB: 80, CentsHr: 4},
			{Slug: "s-4vcpu-8gb", VCPUs: 4, MemoryMB: 8192, DiskGB: 160, CentsHr: 7},
			{Slug: "s-8vcpu-16gb", VCPUs: 8, MemoryMB: 16384, DiskGB: 320, CentsHr: 14},
			{Slug: "s-16vcpu-32gb", VCPUs: 16, MemoryMB: 32768, DiskGB: 640, CentsHr: 29},
			{Slug: "g-2vcpu-8gb", VCPUs: 2, MemoryMB: 8192, DiskGB: 25, CentsHr: 7},
			{Slug: "g-4vcpu-16gb", VCPUs: 4, MemoryMB: 16384, DiskGB: 50, CentsHr: 14},
			{Slug: "c-2vcpu-4gb", VCPUs: 2, MemoryMB: 4096, DiskGB: 25, CentsHr: 6},
			{Slug: "c-4vcpu-8gb", VCPUs: 4, MemoryMB: 8192, DiskGB: 50, CentsHr: 13},
		},
	}

	return func(c *gin.Context) {
		resp, err := pricingHTTPClient.Get(pricingURL + "/v1/pricing/compute")
		if err != nil {
			logger.Logger.Warn().Err(err).Msg("pricing service unreachable, using fallback")
			c.JSON(http.StatusOK, fallback)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			logger.Logger.Warn().Int("status", resp.StatusCode).Msg("pricing service returned error, using fallback")
			c.JSON(http.StatusOK, fallback)
			return
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			c.JSON(http.StatusOK, fallback)
			return
		}

		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			c.JSON(http.StatusOK, fallback)
			return
		}
		c.JSON(http.StatusOK, data)
	}
}

// CloudPresetsHandler returns curated spec presets for the LaunchPage.
// Fetches from the centralized pricing service; falls back to hardcoded presets if unreachable.
func CloudPresetsHandler(pricingURL string) gin.HandlerFunc {
	type fallbackPreset struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Slug        string `json:"slug"`
		VCPUs       int    `json:"vcpus"`
		MemoryGB    int    `json:"memory_gb"`
		CentsHr     int    `json:"cents_per_hour"`
		Provider    string `json:"provider"`
	}

	fallbackPresets := gin.H{"presets": []fallbackPreset{
		{ID: "starter", Name: "Starter", Description: "Light tasks, chat bots, simple automations", Slug: "s-1vcpu-2gb", VCPUs: 1, MemoryGB: 2, CentsHr: 2, Provider: "digitalocean"},
		{ID: "pro", Name: "Pro", Description: "Code generation, research, multi-tool agents", Slug: "s-2vcpu-4gb", VCPUs: 2, MemoryGB: 4, CentsHr: 4, Provider: "digitalocean"},
		{ID: "power", Name: "Power", Description: "Heavy workloads, browser automation, large projects", Slug: "s-4vcpu-8gb", VCPUs: 4, MemoryGB: 8, CentsHr: 7, Provider: "digitalocean"},
		{ID: "gpu", Name: "GPU", Description: "ML training, image generation, video processing", Slug: "g-2vcpu-8gb", VCPUs: 2, MemoryGB: 8, CentsHr: 7, Provider: "digitalocean"},
	}}

	return func(c *gin.Context) {
		resp, err := pricingHTTPClient.Get(pricingURL + "/v1/pricing/compute/presets")
		if err != nil {
			logger.Logger.Warn().Err(err).Msg("pricing service unreachable, using fallback presets")
			c.JSON(http.StatusOK, fallbackPresets)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			logger.Logger.Warn().Int("status", resp.StatusCode).Msg("pricing service returned error, using fallback presets")
			c.JSON(http.StatusOK, fallbackPresets)
			return
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			c.JSON(http.StatusOK, fallbackPresets)
			return
		}

		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			c.JSON(http.StatusOK, fallbackPresets)
			return
		}
		c.JSON(http.StatusOK, data)
	}
}

// TeamProvisionHandler provisions an entire team of cloud agents.
func TeamProvisionHandler(provisioner *cloud.Provisioner) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			TeamName  string                  `json:"team_name"`
			Workspace string                  `json:"workspace,omitempty"`
			Agents    []cloud.ProvisionRequest `json:"agents"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_request",
				"message": err.Error(),
			})
			return
		}

		// Inject IAM context
		owner := ""
		org := ""
		if user := middleware.GetIAMUser(c); user != nil {
			owner = user.Sub
			org = user.Organization
		}

		var results []*cloud.ProvisionResult
		var errors []string

		for _, agentReq := range req.Agents {
			if agentReq.Owner == "" {
				agentReq.Owner = owner
			}
			if agentReq.Org == "" {
				agentReq.Org = org
			}
			if agentReq.Workspace == "" {
				agentReq.Workspace = req.Workspace
			}
			if agentReq.Labels == nil {
				agentReq.Labels = map[string]string{}
			}
			agentReq.Labels["playground.hanzo.ai/team"] = req.TeamName

			result, err := provisioner.Provision(c.Request.Context(), &agentReq)
			if err != nil {
				errors = append(errors, err.Error())
				continue
			}
			results = append(results, result)
		}

		c.JSON(http.StatusCreated, gin.H{
			"team_name": req.TeamName,
			"agents":    results,
			"errors":    errors,
			"count":     len(results),
		})
	}
}
