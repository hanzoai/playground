package handlers

import (
	"net/http"
	"regexp"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hanzoai/playground/control-plane/internal/server/middleware"
	"github.com/hanzoai/playground/control-plane/internal/spaces"
)

// SpaceHandler holds dependencies for Space REST endpoints.
type SpaceHandler struct {
	store spaces.Store
}

// NewSpaceHandler creates a new SpaceHandler.
func NewSpaceHandler(store spaces.Store) *SpaceHandler {
	return &SpaceHandler{store: store}
}

// CreateSpaceRequest is the payload for POST /api/v1/spaces.
type CreateSpaceRequest struct {
	Name        string `json:"name" binding:"required"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

// CreateSpace creates a new Space scoped to the user's IAM org.
func (h *SpaceHandler) CreateSpace(c *gin.Context) {
	user := middleware.GetIAMUser(c)
	orgID := middleware.GetOrganization(c)
	if orgID == "" {
		orgID = "local"
	}
	createdBy := "anonymous"
	if user != nil {
		createdBy = user.Sub
	}

	var req CreateSpaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slug := req.Slug
	if slug == "" {
		slug = slugify(req.Name)
	}

	space := &spaces.Space{
		ID:          uuid.New().String(),
		OrgID:       orgID,
		Name:        req.Name,
		Slug:        slug,
		Description: req.Description,
		CreatedBy:   createdBy,
	}

	if err := h.store.CreateSpace(c.Request.Context(), space); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Add creator as owner
	_ = h.store.AddMember(c.Request.Context(), &spaces.SpaceMember{
		SpaceID: space.ID,
		UserID:  createdBy,
		Role:    "owner",
	})

	c.JSON(http.StatusCreated, space)
}

// ListSpaces returns all spaces for the user's org.
func (h *SpaceHandler) ListSpaces(c *gin.Context) {
	orgID := middleware.GetOrganization(c)
	if orgID == "" {
		orgID = "local"
	}

	result, err := h.store.ListSpaces(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"spaces": result})
}

// GetSpace returns a single space by ID.
func (h *SpaceHandler) GetSpace(c *gin.Context) {
	id := c.Param("id")
	space, err := h.store.GetSpace(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Verify org access
	orgID := middleware.GetOrganization(c)
	if orgID != "" && orgID != "local" && space.OrgID != orgID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	c.JSON(http.StatusOK, space)
}

// UpdateSpace updates a space's name, slug, or description.
func (h *SpaceHandler) UpdateSpace(c *gin.Context) {
	id := c.Param("id")
	space, err := h.store.GetSpace(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	var req CreateSpaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	space.Name = req.Name
	if req.Slug != "" {
		space.Slug = req.Slug
	}
	space.Description = req.Description

	if err := h.store.UpdateSpace(c.Request.Context(), space); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, space)
}

// DeleteSpace removes a space and all its associated data.
func (h *SpaceHandler) DeleteSpace(c *gin.Context) {
	id := c.Param("id")
	if err := h.store.DeleteSpace(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// AddMemberRequest is the payload for POST /api/v1/spaces/:id/members.
type AddMemberRequest struct {
	UserID string `json:"user_id" binding:"required"`
	Role   string `json:"role" binding:"required"`
}

// AddMember adds a user to a space with a given role.
func (h *SpaceHandler) AddMember(c *gin.Context) {
	spaceID := c.Param("id")
	var req AddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !spaces.ValidRoles[req.Role] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role, must be one of: owner, admin, member, viewer"})
		return
	}

	member := &spaces.SpaceMember{
		SpaceID: spaceID,
		UserID:  req.UserID,
		Role:    req.Role,
	}
	if err := h.store.AddMember(c.Request.Context(), member); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, member)
}

// RemoveMember removes a user from a space.
func (h *SpaceHandler) RemoveMember(c *gin.Context) {
	spaceID := c.Param("id")
	userID := c.Param("uid")
	if err := h.store.RemoveMember(c.Request.Context(), spaceID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"removed": true})
}

// ListMembers returns all members of a space.
func (h *SpaceHandler) ListMembers(c *gin.Context) {
	spaceID := c.Param("id")
	members, err := h.store.ListMembers(c.Request.Context(), spaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"members": members})
}

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '-' {
			return unicode.ToLower(r)
		}
		return -1
	}, s)
	s = nonAlphaNum.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}
