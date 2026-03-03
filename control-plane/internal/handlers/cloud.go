package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/control-plane/internal/billing"
	"github.com/hanzoai/playground/control-plane/internal/cloud"
	"github.com/hanzoai/playground/control-plane/internal/logger"
	"github.com/hanzoai/playground/control-plane/internal/server/middleware"
)

var pricingHTTPClient = &http.Client{Timeout: 5 * time.Second}

// CloudProvisionHandler creates a new cloud agent in the DOKS cluster.
// Requires IAM authentication with org context and sufficient billing balance.
func CloudProvisionHandler(provisioner *cloud.Provisioner) gin.HandlerFunc {
	billingClient := billing.NewClient()

	return func(c *gin.Context) {
		// Require IAM authentication with org context
		orgID, ok := middleware.RequireIAMOrg(c)
		if !ok {
			return
		}
		user := middleware.GetIAMUser(c)

		var req cloud.ProvisionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_request",
				"message": err.Error(),
			})
			return
		}

		req.Owner = user.Sub
		req.Org = orgID

		// Determine compute tier for billing check.
		// VM requests (Mac/Windows) use CentsPerHourVM; K8s pods use CentsPerHour.
		// Mac instances have a 24-hour prepaid minimum (Apple macOS licensing requirement).
		// Canonical pricing lives in pricing.hanzo.ai; these are static fallbacks.
		isVMRequest := req.OS == "macos" || req.OS == "windows"
		tierSlug := req.InstanceType
		if tierSlug == "" {
			tierSlug = "s-2vcpu-4gb" // default K8s tier
		}
		var costPerHour int
		if isVMRequest {
			costPerHour = billing.CentsPerHourVM(tierSlug)
		} else {
			costPerHour = billing.CentsPerHour(tierSlug)
		}
		minimumHours := billing.MinimumHours(tierSlug)

		// Extract bearer token for billing API call
		bearerToken := ""
		if auth := c.GetHeader("Authorization"); auth != "" {
			if t := strings.TrimPrefix(auth, "Bearer "); t != auth {
				bearerToken = t
			}
		}

		// Check billing: require minimumHours of compute balance.
		// Commerce stores balances under "org/name" format, not UUID.
		billingUserID := user.Organization + "/" + user.LoginName
		allowance, err := billing.CheckProvisionAllowance(
			c.Request.Context(), billingClient,
			billingUserID, bearerToken, costPerHour, minimumHours,
		)
		if err != nil {
			logger.Logger.Error().Err(err).Msg("billing check failed")
			c.JSON(http.StatusBadGateway, gin.H{
				"error":   "billing_unavailable",
				"message": "Billing service is temporarily unavailable. Please try again.",
			})
			return
		}
		if !allowance.Allowed {
			cloud.RecordBillingCheck("denied")
			resp := gin.H{
				"error":          allowance.Reason,
				"balance_cents":  allowance.BalanceCents,
				"required_cents": allowance.RequiredCents,
				"hours_afford":   allowance.HoursAfford,
				"minimum_hours":  minimumHours,
			}
			if minimumHours == 24 {
				resp["note"] = "macOS instances require a 24-hour minimum due to Apple licensing requirements — this is not a Hanzo limitation."
			}
			c.JSON(http.StatusPaymentRequired, resp)
			return
		}
		cloud.RecordBillingCheck("allowed")

		// Pass the user's bearer token for per-user billing.
		// Cloud bots will use this as HANZO_API_KEY so LLM usage
		// is tracked and billed to the launching user's account.
		if bearerToken != "" {
			req.UserAPIKey = bearerToken
		}
		if req.UserAPIKey == "" {
			if key := c.GetHeader("X-API-Key"); key != "" {
				req.UserAPIKey = key
			}
		}

		provisionStart := time.Now()
		result, err := provisioner.Provision(c.Request.Context(), &req)
		provisionDur := time.Since(provisionStart)
		if err != nil {
			cloud.RecordProvision(tierSlug, req.OS, "error", provisionDur)
			logger.Logger.Error().Err(err).Str("node_id", req.NodeID).Msg("cloud provision failed")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "provision_failed",
				"message": err.Error(),
			})
			return
		}

		cloud.RecordProvision(tierSlug, req.OS, "success", provisionDur)

		// Place a billing hold for the minimum prepaid period.
		holdAmount := costPerHour * minimumHours
		holdDesc := fmt.Sprintf("Cloud compute: %s (%d hrs at $%.2f/hr)",
			result.NodeID, minimumHours, float64(costPerHour)/100.0)
		hold, holdErr := billingClient.CreateHold(
			c.Request.Context(), billingUserID, bearerToken, holdAmount, holdDesc,
		)
		if holdErr != nil {
			// Log the failure but do not block provisioning -- the node is already running.
			// The metering service will continue to record usage regardless of hold status.
			logger.Logger.Error().
				Err(holdErr).
				Str("node_id", result.NodeID).
				Int("amount_cents", holdAmount).
				Msg("failed to create billing hold after provision")
		}

		// Store billing metadata on the CloudNode so deprovision can settle.
		if node, nodeErr := provisioner.GetNode(c.Request.Context(), result.NodeID); nodeErr == nil {
			node.ProvisionedAt = result.CreatedAt
			node.CentsPerHour = costPerHour
			node.BillingUserID = billingUserID
			node.BearerToken = bearerToken
			if hold != nil {
				node.HoldID = hold.ID
			}
		}

		cloud.RecordBillingHold(tierSlug)
		c.JSON(http.StatusCreated, result)
	}
}

// CloudDeprovisionHandler removes a cloud agent from the cluster.
// Requires IAM authentication with org context.
func CloudDeprovisionHandler(provisioner *cloud.Provisioner) gin.HandlerFunc {
	return func(c *gin.Context) {
		org, ok := middleware.RequireIAMOrg(c)
		if !ok {
			return
		}

		nodeID := c.Param("node_id")
		if nodeID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_request",
				"message": "node_id is required",
			})
			return
		}

		// Verify node belongs to org
		node, err := provisioner.GetNode(c.Request.Context(), nodeID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "node not found"})
			return
		}
		if node.Org != "" && node.Org != org {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "node does not belong to your organization"})
			return
		}

		// Settle billing hold with actual usage before tearing down the node.
		billingClient := billing.NewClient()
		if node.HoldID != "" {
			elapsed := time.Since(node.ProvisionedAt)
			elapsedHours := elapsed.Hours()
			actualCents := int(math.Ceil(elapsedHours * float64(node.CentsPerHour)))
			if actualCents < 0 {
				actualCents = 0
			}
			if settleErr := billingClient.SettleHold(
				c.Request.Context(), node.HoldID, node.BearerToken, actualCents,
			); settleErr != nil {
				logger.Logger.Error().
					Err(settleErr).
					Str("node_id", nodeID).
					Str("hold_id", node.HoldID).
					Int("actual_cents", actualCents).
					Msg("failed to settle billing hold on deprovision")
			} else {
				logger.Logger.Info().
					Str("node_id", nodeID).
					Str("hold_id", node.HoldID).
					Int("actual_cents", actualCents).
					Str("elapsed", elapsed.String()).
					Msg("settled billing hold on deprovision")
			}
		}

		if err := provisioner.Deprovision(c.Request.Context(), nodeID); err != nil {
			cloud.RecordDeprovision("error")
			logger.Logger.Error().Err(err).Str("node_id", nodeID).Msg("cloud deprovision failed")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "deprovision_failed",
				"message": err.Error(),
			})
			return
		}

		cloud.RecordDeprovision("success")
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
		org, ok := middleware.RequireOrg(c)
		if !ok {
			return
		}

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

		// Verify node belongs to org
		if node.Org != "" && node.Org != org {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "node does not belong to your organization"})
			return
		}

		c.JSON(http.StatusOK, node)
	}
}

// CloudGetLogsHandler returns recent logs for a cloud agent.
func CloudGetLogsHandler(provisioner *cloud.Provisioner) gin.HandlerFunc {
	return func(c *gin.Context) {
		org, ok := middleware.RequireOrg(c)
		if !ok {
			return
		}

		nodeID := c.Param("node_id")
		if nodeID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_request",
				"message": "node_id is required",
			})
			return
		}

		// Verify node belongs to org
		if node, err := provisioner.GetNode(c.Request.Context(), nodeID); err == nil {
			if node.Org != "" && node.Org != org {
				c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "node does not belong to your organization"})
				return
			}
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
// Requires IAM authentication with org context and sufficient billing balance.
func TeamProvisionHandler(provisioner *cloud.Provisioner) gin.HandlerFunc {
	billingClient := billing.NewClient()

	return func(c *gin.Context) {
		// Require IAM authentication with org context
		orgID, ok := middleware.RequireIAMOrg(c)
		if !ok {
			return
		}
		user := middleware.GetIAMUser(c)

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

		// Calculate total cost for the entire team.
		// Use the highest minimumHours across all agents (e.g. if one agent
		// is Mac with 24h minimum, the whole team check uses 24h).
		totalCentsPerHour := 0
		maxMinimumHours := 1
		for _, agentReq := range req.Agents {
			slug := agentReq.InstanceType
			if slug == "" {
				slug = "s-2vcpu-4gb"
			}
			isVM := agentReq.OS == "macos" || agentReq.OS == "windows"
			if isVM {
				totalCentsPerHour += billing.CentsPerHourVM(slug)
			} else {
				totalCentsPerHour += billing.CentsPerHour(slug)
			}
			if mh := billing.MinimumHours(slug); mh > maxMinimumHours {
				maxMinimumHours = mh
			}
		}

		// Extract bearer token for billing API call
		bearerToken := ""
		if auth := c.GetHeader("Authorization"); auth != "" {
			if t := strings.TrimPrefix(auth, "Bearer "); t != auth {
				bearerToken = t
			}
		}

		// Check billing: require minimumHours of total team compute.
		// Commerce stores balances under "org/name" format, not UUID.
		billingUserID := user.Organization + "/" + user.LoginName
		allowance, err := billing.CheckProvisionAllowance(
			c.Request.Context(), billingClient,
			billingUserID, bearerToken, totalCentsPerHour, maxMinimumHours,
		)
		if err != nil {
			logger.Logger.Error().Err(err).Msg("billing check failed for team provision")
			c.JSON(http.StatusBadGateway, gin.H{
				"error":   "billing_unavailable",
				"message": "Billing service is temporarily unavailable. Please try again.",
			})
			return
		}
		if !allowance.Allowed {
			cloud.RecordBillingCheck("denied")
			c.JSON(http.StatusPaymentRequired, gin.H{
				"error":          allowance.Reason,
				"balance_cents":  allowance.BalanceCents,
				"required_cents": allowance.RequiredCents,
				"hours_afford":   allowance.HoursAfford,
			})
			return
		}
		cloud.RecordBillingCheck("allowed")

		var results []*cloud.ProvisionResult
		var errors []string

		for _, agentReq := range req.Agents {
			agentReq.Owner = user.Sub
			agentReq.Org = orgID
			if agentReq.Workspace == "" {
				agentReq.Workspace = req.Workspace
			}
			if agentReq.Labels == nil {
				agentReq.Labels = map[string]string{}
			}
			agentReq.Labels["playground.hanzo.ai/team"] = req.TeamName

			tierSlug := agentReq.InstanceType
			if tierSlug == "" {
				tierSlug = "s-2vcpu-4gb"
			}
			agentOS := agentReq.OS
			if agentOS == "" {
				agentOS = "linux"
			}

			provisionStart := time.Now()
			result, err := provisioner.Provision(c.Request.Context(), &agentReq)
			provisionDur := time.Since(provisionStart)
			if err != nil {
				cloud.RecordProvision(tierSlug, agentOS, "error", provisionDur)
				errors = append(errors, err.Error())
				continue
			}
			cloud.RecordProvision(tierSlug, agentOS, "success", provisionDur)
			cloud.RecordBillingHold(tierSlug)
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

// CloudBillingBalanceHandler returns the user's billing balance and
// affordability for each preset tier, including VM presets for Mac/Windows.
// Used by frontend to show pricing context. Canonical pricing from pricing.hanzo.ai;
// these are static fallbacks for the billing pre-check.
func CloudBillingBalanceHandler() gin.HandlerFunc {
	billingClient := billing.NewClient()

	type presetAffordability struct {
		Name         string `json:"name"`
		CentsPerHour int    `json:"cents_per_hour"`
		HoursAfford  int    `json:"hours_afford"`
	}

	type vmPreset struct {
		Name            string `json:"name"`
		InstanceType    string `json:"instance_type"`
		OS              string `json:"os"`
		CentsPerHour    int    `json:"cents_per_hour"`
		MinimumHours    int    `json:"minimum_hours"`
		MinimumCostCents int   `json:"minimum_cost_cents"`
		HoursAfford     int    `json:"hours_afford"`
		Note            string `json:"note,omitempty"`
	}

	return func(c *gin.Context) {
		_, ok := middleware.RequireIAMOrg(c)
		if !ok {
			return
		}
		user := middleware.GetIAMUser(c)

		bearerToken := ""
		if auth := c.GetHeader("Authorization"); auth != "" {
			if t := strings.TrimPrefix(auth, "Bearer "); t != auth {
				bearerToken = t
			}
		}

		billingUserID := user.Organization + "/" + user.LoginName
		balance, err := billingClient.GetBalance(c.Request.Context(), billingUserID, bearerToken)
		if err != nil {
			logger.Logger.Error().Err(err).Msg("billing balance check failed")
			c.JSON(http.StatusBadGateway, gin.H{
				"error":   "billing_unavailable",
				"message": "Billing service is temporarily unavailable.",
			})
			return
		}

		availableCents := int(balance.Available)

		// K8s container presets
		presets := []presetAffordability{
			{Name: "starter", CentsPerHour: billing.CentsPerHour("starter")},
			{Name: "pro", CentsPerHour: billing.CentsPerHour("pro")},
			{Name: "power", CentsPerHour: billing.CentsPerHour("power")},
			{Name: "gpu", CentsPerHour: billing.CentsPerHour("gpu")},
		}
		for i := range presets {
			if presets[i].CentsPerHour > 0 {
				presets[i].HoursAfford = availableCents / presets[i].CentsPerHour
			}
		}

		// VM presets for Mac/Windows/Linux VMs (provisioned via Visor).
		// Mac 24h minimum is an Apple macOS licensing requirement, not a Hanzo policy.
		macNote := "24-hour minimum required by Apple macOS licensing — not a Hanzo limitation."
		vmPresets := []vmPreset{
			{Name: "mac-m2", InstanceType: "mac2-m2.metal", OS: "macos",
				CentsPerHour: 65, MinimumHours: 24, MinimumCostCents: 1560, Note: macNote},
			{Name: "mac-m4", InstanceType: "mac-m4.metal", OS: "macos",
				CentsPerHour: 123, MinimumHours: 24, MinimumCostCents: 2952, Note: macNote},
			{Name: "windows", InstanceType: "t3.medium", OS: "windows",
				CentsPerHour: 4, MinimumHours: 1, MinimumCostCents: 4},
			{Name: "linux-vm", InstanceType: "t3.medium", OS: "linux",
				CentsPerHour: 4, MinimumHours: 1, MinimumCostCents: 4},
		}
		for i := range vmPresets {
			if vmPresets[i].CentsPerHour > 0 {
				vmPresets[i].HoursAfford = availableCents / vmPresets[i].CentsPerHour
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"balance_cents": availableCents,
			"currency":      "usd",
			"presets":       presets,
			"vm_presets":    vmPresets,
		})
	}
}
