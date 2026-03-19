package gitops

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Handlers provides HTTP endpoints for git operations on spaces.
type Handlers struct {
	manager *Manager
}

// NewHandlers creates Handlers backed by the given Manager.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// repo is a helper that resolves the space ID from the request and returns
// the Repo, or writes an error response and returns nil.
func (h *Handlers) repo(c *gin.Context) *Repo {
	spaceID := c.Param("id")
	if spaceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing space id"})
		return nil
	}

	r, err := h.manager.GetOrInit(spaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return nil
	}
	return r
}

// Status returns git status for a space.
// GET /api/v1/spaces/:id/git/status
func (h *Handlers) Status(c *gin.Context) {
	r := h.repo(c)
	if r == nil {
		return
	}

	changes, err := r.Status()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if changes == nil {
		changes = []FileChange{}
	}

	c.JSON(http.StatusOK, gin.H{"changes": changes})
}

// Log returns commit history for a space.
// GET /api/v1/spaces/:id/git/log?limit=20
func (h *Handlers) Log(c *gin.Context) {
	r := h.repo(c)
	if r == nil {
		return
	}

	limit := 20
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	commits, err := r.Log(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if commits == nil {
		commits = []CommitInfo{}
	}

	c.JSON(http.StatusOK, gin.H{"commits": commits})
}

// commitRequest is the JSON body for POST /git/commit.
type commitRequest struct {
	Message string       `json:"message" binding:"required"`
	Files   []string     `json:"files"`
	Author  CommitAuthor `json:"author"`
}

// Commit creates a commit in a space's repo.
// POST /api/v1/spaces/:id/git/commit
func (h *Handlers) Commit(c *gin.Context) {
	r := h.repo(c)
	if r == nil {
		return
	}

	var req commitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Stage files: if specific files listed, add those; otherwise add all.
	if err := r.Add(req.Files...); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "staging: " + err.Error()})
		return
	}

	hash, err := r.Commit(req.Message, req.Author)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"hash": hash.String()})
}

// ListBranches lists all branches.
// GET /api/v1/spaces/:id/git/branches
func (h *Handlers) ListBranches(c *gin.Context) {
	r := h.repo(c)
	if r == nil {
		return
	}

	branches, err := r.ListBranches()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if branches == nil {
		branches = []BranchInfo{}
	}

	c.JSON(http.StatusOK, gin.H{"branches": branches})
}

// createBranchRequest is the JSON body for POST /git/branches.
type createBranchRequest struct {
	Name string `json:"name" binding:"required"`
}

// CreateBranch creates a new branch.
// POST /api/v1/spaces/:id/git/branches
func (h *Handlers) CreateBranch(c *gin.Context) {
	r := h.repo(c)
	if r == nil {
		return
	}

	var req createBranchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := r.Branch(req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"branch": req.Name})
}

// checkoutRequest is the JSON body for POST /git/checkout.
type checkoutRequest struct {
	Branch string `json:"branch" binding:"required"`
}

// Checkout switches branch.
// POST /api/v1/spaces/:id/git/checkout
func (h *Handlers) Checkout(c *gin.Context) {
	r := h.repo(c)
	if r == nil {
		return
	}

	var req checkoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := r.Checkout(req.Branch); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"branch": req.Branch})
}

// Diff returns diff for a space.
// GET /api/v1/spaces/:id/git/diff?from=&to=
func (h *Handlers) Diff(c *gin.Context) {
	r := h.repo(c)
	if r == nil {
		return
	}

	from := c.Query("from")
	to := c.Query("to")

	diff, err := r.Diff(from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"diff": diff})
}

// ListFiles lists files in the workspace directory.
// GET /api/v1/spaces/:id/git/files?path=/
func (h *Handlers) ListFiles(c *gin.Context) {
	r := h.repo(c)
	if r == nil {
		return
	}

	relPath := c.DefaultQuery("path", ".")
	clean := filepath.Clean(relPath)
	if strings.Contains(clean, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path traversal not allowed"})
		return
	}

	fullPath := filepath.Join(r.Path(), clean)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	type fileEntry struct {
		Name  string `json:"name"`
		IsDir bool   `json:"is_dir"`
		Size  int64  `json:"size"`
	}

	files := make([]fileEntry, 0, len(entries))
	for _, e := range entries {
		// Skip .git directory from listings.
		if e.Name() == ".git" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, fileEntry{
			Name:  e.Name(),
			IsDir: e.IsDir(),
			Size:  info.Size(),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		// Directories first, then alphabetical.
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return files[i].Name < files[j].Name
	})

	c.JSON(http.StatusOK, gin.H{"files": files, "path": clean})
}

// ReadFile reads a file from the workspace.
// GET /api/v1/spaces/:id/git/files/*filepath
func (h *Handlers) ReadFile(c *gin.Context) {
	r := h.repo(c)
	if r == nil {
		return
	}

	fp := c.Param("filepath")
	if fp == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing file path"})
		return
	}
	// Strip leading slash from wildcard param.
	fp = strings.TrimPrefix(fp, "/")

	data, err := r.ReadFile(fp)
	if err != nil {
		status := http.StatusInternalServerError
		if os.IsNotExist(err) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"path":    fp,
		"content": string(data),
		"size":    len(data),
	})
}

// WriteFile writes a file and optionally stages it.
// PUT /api/v1/spaces/:id/git/files/*filepath
// Query param: stage=true to auto-stage after writing.
func (h *Handlers) WriteFile(c *gin.Context) {
	r := h.repo(c)
	if r == nil {
		return
	}

	fp := c.Param("filepath")
	if fp == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing file path"})
		return
	}
	fp = strings.TrimPrefix(fp, "/")

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reading body: " + err.Error()})
		return
	}

	if err := r.WriteFile(fp, body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Auto-stage if requested.
	if c.DefaultQuery("stage", "false") == "true" {
		if err := r.Add(fp); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "staging: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"path": fp, "size": len(body)})
}

// cloneRequest is the JSON body for POST /git/clone.
type cloneRequest struct {
	URL    string   `json:"url" binding:"required"`
	Branch string   `json:"branch"`
	Depth  int      `json:"depth"`
	Auth   AuthOpts `json:"auth"`
}

// Clone clones a remote repo into a space.
// POST /api/v1/spaces/:id/git/clone
func (h *Handlers) Clone(c *gin.Context) {
	spaceID := c.Param("id")
	if spaceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing space id"})
		return
	}

	var req cloneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	opts := CloneOpts{
		Branch: req.Branch,
		Depth:  req.Depth,
		Auth:   req.Auth,
	}

	r, err := h.manager.CloneForSpace(c.Request.Context(), spaceID, req.URL, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"space_id": spaceID,
		"path":     r.Path(),
		"url":      req.URL,
	})
}

// pushRequest is the JSON body for POST /git/push.
type pushRequest struct {
	Remote string   `json:"remote"`
	Branch string   `json:"branch"`
	Auth   AuthOpts `json:"auth"`
	Force  bool     `json:"force"`
}

// Push pushes to the remote.
// POST /api/v1/spaces/:id/git/push
func (h *Handlers) Push(c *gin.Context) {
	r := h.repo(c)
	if r == nil {
		return
	}

	var req pushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	opts := PushOpts{
		Remote: req.Remote,
		Branch: req.Branch,
		Auth:   req.Auth,
		Force:  req.Force,
	}

	if err := r.Push(c.Request.Context(), opts); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"pushed": true})
}

// pullRequest is the JSON body for POST /git/pull.
type pullRequest struct {
	Remote string   `json:"remote"`
	Branch string   `json:"branch"`
	Auth   AuthOpts `json:"auth"`
}

// Pull fetches and merges from the remote.
// POST /api/v1/spaces/:id/git/pull
func (h *Handlers) Pull(c *gin.Context) {
	r := h.repo(c)
	if r == nil {
		return
	}

	var req pullRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	opts := PullOpts{
		Remote: req.Remote,
		Branch: req.Branch,
		Auth:   req.Auth,
	}

	if err := r.Pull(c.Request.Context(), opts); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"pulled": true})
}
