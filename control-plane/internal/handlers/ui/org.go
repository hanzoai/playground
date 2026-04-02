package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/control-plane/internal/logger"
	"github.com/hanzoai/playground/control-plane/internal/server/middleware"
)

// OrgHandler proxies organization CRUD operations to the IAM (Casdoor) backend.
// This avoids CORS issues and keeps IAM endpoints internal.
// Uses IAM client credentials for admin-level operations (org create/delete)
// since regular user tokens may not have admin privileges on the IAM server.
type OrgHandler struct {
	iamURL       string
	clientID     string
	clientSecret string
	client       *http.Client
}

// NewOrgHandler creates an OrgHandler reading IAM config from env.
// Checks both HANZO_AGENTS_IAM_* and IAM_* env var prefixes for compatibility
// with the playground deployment which uses the HANZO_AGENTS_ prefix.
func NewOrgHandler() *OrgHandler {
	iamURL := coalesceEnv(
		"IAM_ENDPOINT",
		"HANZO_AGENTS_IAM_ENDPOINT",
		"PLAYGROUND_IAM_ENDPOINT",
		"IAM_PUBLIC_ENDPOINT",
		"HANZO_AGENTS_IAM_PUBLIC_ENDPOINT",
	)
	if iamURL == "" {
		iamURL = "http://iam.hanzo.svc:80"
	}
	iamURL = strings.TrimRight(iamURL, "/")

	clientID := coalesceEnv("IAM_CLIENT_ID", "HANZO_AGENTS_IAM_CLIENT_ID", "PLAYGROUND_IAM_CLIENT_ID")
	clientSecret := coalesceEnv("IAM_CLIENT_SECRET", "HANZO_AGENTS_IAM_CLIENT_SECRET", "PLAYGROUND_IAM_CLIENT_SECRET")

	logger.Logger.Info().
		Str("iam_url", iamURL).
		Bool("has_client_id", clientID != "").
		Bool("has_client_secret", clientSecret != "").
		Msg("org: initialized IAM handler")

	return &OrgHandler{
		iamURL:       iamURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		client:       &http.Client{Timeout: 15 * time.Second},
	}
}

// coalesceEnv returns the first non-empty env var value from the given keys.
func coalesceEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

// ListOrgs returns organizations the current user belongs to.
// GET /v1/orgs
func (h *OrgHandler) ListOrgs(c *gin.Context) {
	user := middleware.GetIAMUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "authentication required"})
		return
	}

	// Casdoor: GET /api/get-organizations?owner=admin
	// Filter to orgs the user has access to
	targetURL := fmt.Sprintf("%s/api/get-organizations?owner=admin", h.iamURL)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to create request"})
		return
	}
	h.setAuthHeader(req, c)

	resp, err := h.client.Do(req)
	if err != nil {
		logger.Logger.Warn().Err(err).Msg("org: IAM get-organizations request failed")
		// Fall back to returning the user's org from JWT
		org := middleware.GetOrganization(c)
		if org != "" {
			c.JSON(http.StatusOK, gin.H{
				"organizations": []gin.H{{"name": org, "owner": "admin"}},
				"fallback":      true,
			})
			return
		}
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "IAM service unavailable"})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	c.Data(resp.StatusCode, "application/json", body)
}

// GetOrg returns details for a specific organization.
// GET /v1/orgs/:orgId
func (h *OrgHandler) GetOrg(c *gin.Context) {
	orgID := c.Param("orgId")
	if orgID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "orgId is required"})
		return
	}

	targetURL := fmt.Sprintf("%s/api/get-organization?id=admin/%s", h.iamURL, url.PathEscape(orgID))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to create request"})
		return
	}
	h.setAuthHeader(req, c)

	resp, err := h.client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "IAM service unavailable"})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	c.Data(resp.StatusCode, "application/json", body)
}

// CreateOrgRequest is the payload for creating an organization.
type CreateOrgRequest struct {
	Name        string `json:"name" binding:"required"`
	DisplayName string `json:"displayName"`
	WebsiteURL  string `json:"websiteUrl"`
	Favicon     string `json:"favicon"`
}

// CreateOrg creates a new organization via IAM and adds the current user as owner.
// POST /v1/orgs
func (h *OrgHandler) CreateOrg(c *gin.Context) {
	user := middleware.GetIAMUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "authentication required"})
		return
	}

	var input CreateOrgRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	displayName := input.DisplayName
	if displayName == "" {
		displayName = input.Name
	}

	// Build Casdoor organization object
	orgPayload := map[string]interface{}{
		"owner":       "admin",
		"name":        input.Name,
		"displayName": displayName,
		"websiteUrl":  input.WebsiteURL,
		"favicon":     input.Favicon,
	}

	payloadBytes, _ := json.Marshal(orgPayload)
	targetURL := fmt.Sprintf("%s/api/add-organization", h.iamURL)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewReader(payloadBytes))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to create request"})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	h.setAdminAuth(req, c)

	resp, err := h.client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "IAM service unavailable"})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// After successful org creation, add the user as a member
		h.addUserToOrg(c, user, input.Name)
	}

	c.Data(resp.StatusCode, "application/json", body)
}

// UpdateOrg updates an existing organization.
// PUT /v1/orgs/:orgId
func (h *OrgHandler) UpdateOrg(c *gin.Context) {
	orgID := c.Param("orgId")
	if orgID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "orgId is required"})
		return
	}

	// Verify user belongs to this org
	currentOrg := middleware.GetOrganization(c)
	if currentOrg != orgID {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "you can only update your own organization"})
		return
	}

	// Read the body and forward to IAM
	bodyBytes, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "failed to read request body"})
		return
	}

	// Inject owner and name into the payload for Casdoor
	var payload map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid JSON body"})
		return
	}
	payload["owner"] = "admin"
	payload["name"] = orgID
	updatedBytes, _ := json.Marshal(payload)

	targetURL := fmt.Sprintf("%s/api/update-organization", h.iamURL)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, reqErr := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewReader(updatedBytes))
	if reqErr != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to create request"})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	h.setAdminAuth(req, c)

	resp, doErr := h.client.Do(req)
	if doErr != nil {
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "IAM service unavailable"})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	c.Data(resp.StatusCode, "application/json", respBody)
}

// DeleteOrg deletes an organization.
// DELETE /v1/orgs/:orgId
func (h *OrgHandler) DeleteOrg(c *gin.Context) {
	orgID := c.Param("orgId")
	if orgID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "orgId is required"})
		return
	}

	// Verify user belongs to this org
	currentOrg := middleware.GetOrganization(c)
	if currentOrg != orgID {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "you can only delete your own organization"})
		return
	}

	// Casdoor delete-organization expects the org object in the body
	deletePayload := map[string]interface{}{
		"owner": "admin",
		"name":  orgID,
	}
	payloadBytes, _ := json.Marshal(deletePayload)

	targetURL := fmt.Sprintf("%s/api/delete-organization", h.iamURL)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewReader(payloadBytes))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to create request"})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	h.setAdminAuth(req, c)

	resp, doErr := h.client.Do(req)
	if doErr != nil {
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "IAM service unavailable"})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	c.Data(resp.StatusCode, "application/json", body)
}

// GetOrgMembers returns members of an organization.
// GET /v1/orgs/:orgId/members
func (h *OrgHandler) GetOrgMembers(c *gin.Context) {
	orgID := c.Param("orgId")
	if orgID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "orgId is required"})
		return
	}

	// Verify caller belongs to this org
	user := middleware.GetIAMUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "authentication required"})
		return
	}
	currentOrg := middleware.GetOrganization(c)
	if currentOrg != orgID && !user.IsAdmin {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "access denied to this organization"})
		return
	}

	targetURL := fmt.Sprintf("%s/api/get-users?owner=%s", h.iamURL, url.QueryEscape(orgID))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to create request"})
		return
	}
	h.setAuthHeader(req, c)

	resp, doErr := h.client.Do(req)
	if doErr != nil {
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "IAM service unavailable"})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	c.Data(resp.StatusCode, "application/json", body)
}

// setAuthHeader forwards the user's IAM token to the backend.
func (h *OrgHandler) setAuthHeader(req *http.Request, c *gin.Context) {
	if auth := c.GetHeader("Authorization"); auth != "" {
		req.Header.Set("Authorization", auth)
	}
	req.Header.Set("Accept", "application/json")
}

// setAdminAuth sets IAM client credentials for admin-level operations.
// Casdoor accepts ?clientId=X&clientSecret=Y query params for service auth.
// Falls back to the user's token if no client credentials are configured.
func (h *OrgHandler) setAdminAuth(req *http.Request, c *gin.Context) {
	if h.clientID != "" && h.clientSecret != "" {
		q := req.URL.Query()
		q.Set("clientId", h.clientID)
		q.Set("clientSecret", h.clientSecret)
		req.URL.RawQuery = q.Encode()
	} else if auth := c.GetHeader("Authorization"); auth != "" {
		req.Header.Set("Authorization", auth)
	}
	req.Header.Set("Accept", "application/json")
}

// addUserToOrg adds the authenticated user to the given organization.
// This is best-effort — we log failures but don't fail the org creation.
func (h *OrgHandler) addUserToOrg(c *gin.Context, user *middleware.IAMUserInfo, orgName string) {
	if user == nil || user.Sub == "" {
		return
	}

	// Parse user sub (format: "org/username" or just "username")
	username := user.Sub
	owner := orgName
	if parts := strings.SplitN(user.Sub, "/", 2); len(parts) == 2 {
		username = parts[1]
	}

	// Casdoor: add-user-to-organization
	payload := map[string]interface{}{
		"owner":        owner,
		"name":         username,
		"organization": orgName,
	}
	payloadBytes, _ := json.Marshal(payload)

	targetURL := fmt.Sprintf("%s/api/add-user", h.iamURL)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewReader(payloadBytes))
	if err != nil {
		logger.Logger.Warn().Err(err).Str("org", orgName).Msg("org: failed to create add-user request")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	h.setAdminAuth(req, c)

	resp, err := h.client.Do(req)
	if err != nil {
		logger.Logger.Warn().Err(err).Str("org", orgName).Msg("org: failed to add user to org")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		logger.Logger.Warn().
			Int("status", resp.StatusCode).
			Str("body", string(body)).
			Str("org", orgName).
			Msg("org: add-user returned non-success")
	}
}
