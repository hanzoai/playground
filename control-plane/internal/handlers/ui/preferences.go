package ui

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/control-plane/internal/server/middleware"
	"github.com/hanzoai/playground/control-plane/internal/storage"
)

// PreferencesHandler handles user preference CRUD operations.
type PreferencesHandler struct {
	storage storage.StorageProvider
}

// NewPreferencesHandler creates a new PreferencesHandler.
func NewPreferencesHandler(s storage.StorageProvider) *PreferencesHandler {
	return &PreferencesHandler{storage: s}
}

// resolveUserID extracts the user ID from the IAM context or falls back to API key hash.
func resolveUserID(c *gin.Context) string {
	if user := middleware.GetIAMUser(c); user != nil && user.Sub != "" {
		return user.Sub
	}
	// Fallback: use a deterministic ID from the API key header
	apiKey := c.GetHeader("X-API-Key")
	if apiKey != "" {
		return "apikey:" + apiKey[:8]
	}
	return "anonymous"
}

// GetPreferencesHandler returns the current user's preferences.
// GET /api/v1/preferences
func (h *PreferencesHandler) GetPreferencesHandler(c *gin.Context) {
	ctx := c.Request.Context()
	userID := resolveUserID(c)

	prefs, err := h.storage.GetUserPreferences(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to get preferences"})
		return
	}

	if prefs == nil {
		// Return defaults for new users
		prefs = storage.DefaultUserPreferences(userID)
	}

	c.JSON(http.StatusOK, prefs)
}

// PutPreferencesHandler upserts the current user's preferences.
// PUT /api/v1/preferences
func (h *PreferencesHandler) PutPreferencesHandler(c *gin.Context) {
	ctx := c.Request.Context()
	userID := resolveUserID(c)

	var prefs storage.UserPreferences
	if err := c.ShouldBindJSON(&prefs); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	// Force the user_id from auth context
	prefs.UserID = userID

	if err := h.storage.SetUserPreferences(ctx, &prefs); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to save preferences"})
		return
	}

	c.JSON(http.StatusOK, &prefs)
}
