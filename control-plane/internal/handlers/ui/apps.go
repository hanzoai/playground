package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/control-plane/internal/server/middleware"
)

// AppHandler proxies IAM application CRUD operations.
// Applications are scoped to organizations and represent OAuth clients.
type AppHandler struct {
	iamURL string
	client *http.Client
}

// NewAppHandler creates an AppHandler.
// Checks both HANZO_AGENTS_IAM_* and IAM_* env var prefixes for compatibility.
func NewAppHandler() *AppHandler {
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

	return &AppHandler{
		iamURL: iamURL,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// ListApps returns applications belonging to an organization.
// GET /v1/orgs/:orgId/apps
func (h *AppHandler) ListApps(c *gin.Context) {
	orgID := c.Param("orgId")
	if orgID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "orgId is required"})
		return
	}

	// Verify user has access to this org
	if !h.verifyOrgAccess(c, orgID) {
		return
	}

	targetURL := fmt.Sprintf("%s/api/get-applications?owner=%s", h.iamURL, url.QueryEscape(orgID))

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

// CreateAppRequest is the payload for creating an application.
type CreateAppRequest struct {
	Name         string   `json:"name" binding:"required"`
	DisplayName  string   `json:"displayName"`
	Description  string   `json:"description"`
	HomepageURL  string   `json:"homepageUrl"`
	RedirectURIs []string `json:"redirectUris"`
	GrantTypes   []string `json:"grantTypes"`
	// OAuth provider config
	EnablePassword bool `json:"enablePassword"`
	EnableGoogle   bool `json:"enableGoogle"`
	EnableGitHub   bool `json:"enableGithub"`
}

// CreateApp creates an IAM application under the given organization.
// POST /v1/orgs/:orgId/apps
func (h *AppHandler) CreateApp(c *gin.Context) {
	orgID := c.Param("orgId")
	if orgID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "orgId is required"})
		return
	}
	if !h.verifyOrgAccess(c, orgID) {
		return
	}

	var input CreateAppRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	displayName := input.DisplayName
	if displayName == "" {
		displayName = input.Name
	}

	redirectURIs := input.RedirectURIs
	if len(redirectURIs) == 0 {
		redirectURIs = []string{"http://localhost:3000/callback"}
	}

	grantTypes := input.GrantTypes
	if len(grantTypes) == 0 {
		grantTypes = []string{"authorization_code", "refresh_token"}
	}

	// Build Casdoor application object
	appPayload := map[string]interface{}{
		"owner":        orgID,
		"name":         input.Name,
		"displayName":  displayName,
		"description":  input.Description,
		"homepageUrl":  input.HomepageURL,
		"redirectUris":  redirectURIs,
		"grantTypes":   grantTypes,
		"enablePassword": input.EnablePassword,
	}

	// Configure providers based on request
	providers := []map[string]interface{}{}
	if input.EnableGoogle {
		providers = append(providers, map[string]interface{}{
			"name":      fmt.Sprintf("provider_google_%s", orgID),
			"canSignUp": true,
			"canSignIn": true,
			"canUnlink": true,
			"prompted":  false,
		})
	}
	if input.EnableGitHub {
		providers = append(providers, map[string]interface{}{
			"name":      fmt.Sprintf("provider_github_%s", orgID),
			"canSignUp": true,
			"canSignIn": true,
			"canUnlink": true,
			"prompted":  false,
		})
	}
	if len(providers) > 0 {
		appPayload["providers"] = providers
	}

	payloadBytes, _ := json.Marshal(appPayload)
	targetURL := fmt.Sprintf("%s/api/add-application", h.iamURL)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewReader(payloadBytes))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to create request"})
		return
	}
	req.Header.Set("Content-Type", "application/json")
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

// GetApp returns a specific application.
// GET /v1/orgs/:orgId/apps/:appId
func (h *AppHandler) GetApp(c *gin.Context) {
	orgID := c.Param("orgId")
	appID := c.Param("appId")
	if orgID == "" || appID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "orgId and appId are required"})
		return
	}
	if !h.verifyOrgAccess(c, orgID) {
		return
	}

	targetURL := fmt.Sprintf("%s/api/get-application?id=%s/%s", h.iamURL, url.QueryEscape(orgID), url.QueryEscape(appID))

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

// UpdateApp updates an existing application.
// PUT /v1/orgs/:orgId/apps/:appId
func (h *AppHandler) UpdateApp(c *gin.Context) {
	orgID := c.Param("orgId")
	appID := c.Param("appId")
	if orgID == "" || appID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "orgId and appId are required"})
		return
	}
	if !h.verifyOrgAccess(c, orgID) {
		return
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "failed to read request body"})
		return
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid JSON body"})
		return
	}
	payload["owner"] = orgID
	payload["name"] = appID
	updatedBytes, _ := json.Marshal(payload)

	targetURL := fmt.Sprintf("%s/api/update-application", h.iamURL)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, reqErr := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewReader(updatedBytes))
	if reqErr != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to create request"})
		return
	}
	req.Header.Set("Content-Type", "application/json")
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

// DeleteApp deletes an application.
// DELETE /v1/orgs/:orgId/apps/:appId
func (h *AppHandler) DeleteApp(c *gin.Context) {
	orgID := c.Param("orgId")
	appID := c.Param("appId")
	if orgID == "" || appID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "orgId and appId are required"})
		return
	}
	if !h.verifyOrgAccess(c, orgID) {
		return
	}

	deletePayload := map[string]interface{}{
		"owner": orgID,
		"name":  appID,
	}
	payloadBytes, _ := json.Marshal(deletePayload)

	targetURL := fmt.Sprintf("%s/api/delete-application", h.iamURL)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewReader(payloadBytes))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to create request"})
		return
	}
	req.Header.Set("Content-Type", "application/json")
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

// verifyOrgAccess checks that the authenticated user belongs to the given org.
func (h *AppHandler) verifyOrgAccess(c *gin.Context, orgID string) bool {
	user := middleware.GetIAMUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "authentication required"})
		return false
	}
	currentOrg := middleware.GetOrganization(c)
	// Allow access if user's org matches, or if the user is admin
	if currentOrg != orgID && !user.IsAdmin {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "access denied to this organization"})
		return false
	}
	return true
}

// setAuthHeader forwards the user's IAM token.
func (h *AppHandler) setAuthHeader(req *http.Request, c *gin.Context) {
	if auth := c.GetHeader("Authorization"); auth != "" {
		req.Header.Set("Authorization", auth)
	}
	req.Header.Set("Accept", "application/json")
}
