package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/control-plane/internal/cloud"
	"github.com/hanzoai/playground/control-plane/internal/logger"
	"github.com/hanzoai/playground/control-plane/internal/server/middleware"
)

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
