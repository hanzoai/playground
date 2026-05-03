package gitops

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRouter(t *testing.T) (*gin.Engine, *Manager) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	m := NewManager(dir)
	h := NewHandlers(m)

	r := gin.New()
	g := r.Group("/api/v1/spaces/:id/git")
	{
		g.GET("/status", h.Status)
		g.GET("/log", h.Log)
		g.POST("/commit", h.Commit)
		g.GET("/branches", h.ListBranches)
		g.POST("/branches", h.CreateBranch)
		g.POST("/checkout", h.Checkout)
		g.GET("/diff", h.Diff)
		g.GET("/files", h.ListFiles)
		g.GET("/files/*filepath", h.ReadFile)
		g.PUT("/files/*filepath", h.WriteFile)
		g.POST("/push", h.Push)
		g.POST("/pull", h.Pull)
		g.POST("/clone", h.Clone)
	}

	return r, m
}

func TestHandlers_Status_EmptyRepo(t *testing.T) {
	r, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/spaces/s1/git/status", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.NotNil(t, resp["changes"])
}

func TestHandlers_WriteAndReadFile(t *testing.T) {
	r, _ := setupTestRouter(t)

	// Write a file.
	w := httptest.NewRecorder()
	body := bytes.NewBufferString("file content here")
	req, _ := http.NewRequest("PUT", "/api/v1/spaces/s1/git/files/test.txt", body)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Read it back.
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/spaces/s1/git/files/test.txt", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "file content here", resp["content"])
}

func TestHandlers_CommitAndLog(t *testing.T) {
	r, _ := setupTestRouter(t)

	// Write a file first.
	w := httptest.NewRecorder()
	body := bytes.NewBufferString("data")
	req, _ := http.NewRequest("PUT", "/api/v1/spaces/s1/git/files/hello.txt?stage=true", body)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Commit.
	commitBody := map[string]interface{}{
		"message": "test commit",
		"files":   []string{"hello.txt"},
		"author": map[string]string{
			"name":     "Test Agent",
			"agent_id": "agent-99",
		},
	}
	b, _ := json.Marshal(commitBody)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/spaces/s1/git/commit", bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var commitResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &commitResp)
	assert.NotEmpty(t, commitResp["hash"])

	// Log.
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/spaces/s1/git/log?limit=5", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var logResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &logResp)
	commits := logResp["commits"].([]interface{})
	assert.Len(t, commits, 1)
}

func TestHandlers_BranchAndCheckout(t *testing.T) {
	r, _ := setupTestRouter(t)

	// Need an initial commit.
	body := bytes.NewBufferString("init")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/spaces/s1/git/files/init.txt?stage=true", body)
	r.ServeHTTP(w, req)

	commitBody, _ := json.Marshal(map[string]interface{}{
		"message": "init",
		"author":  map[string]string{"name": "t", "email": "t@t.com"},
	})
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/spaces/s1/git/commit", bytes.NewBuffer(commitBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// Create branch.
	branchBody, _ := json.Marshal(map[string]string{"name": "dev"})
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/spaces/s1/git/branches", bytes.NewBuffer(branchBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// List branches.
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/spaces/s1/git/branches", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var brResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &brResp)
	branches := brResp["branches"].([]interface{})
	assert.GreaterOrEqual(t, len(branches), 2)

	// Checkout.
	checkoutBody, _ := json.Marshal(map[string]string{"branch": "dev"})
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/spaces/s1/git/checkout", bytes.NewBuffer(checkoutBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_ListFiles(t *testing.T) {
	r, _ := setupTestRouter(t)

	// Write some files.
	for _, name := range []string{"a.txt", "b.txt"} {
		w := httptest.NewRecorder()
		body := bytes.NewBufferString("content")
		req, _ := http.NewRequest("PUT", "/api/v1/spaces/s1/git/files/"+name, body)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// List files.
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/spaces/s1/git/files?path=.", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	files := resp["files"].([]interface{})
	// Should have a.txt, b.txt (and possibly .git is filtered out).
	assert.GreaterOrEqual(t, len(files), 2)
}

func TestHandlers_Diff(t *testing.T) {
	r, _ := setupTestRouter(t)

	// Write, commit, then modify.
	body := bytes.NewBufferString("v1")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/spaces/s1/git/files/f.txt?stage=true", body)
	r.ServeHTTP(w, req)

	commitBody, _ := json.Marshal(map[string]interface{}{
		"message": "v1",
		"author":  map[string]string{"name": "t", "email": "t@t.com"},
	})
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/spaces/s1/git/commit", bytes.NewBuffer(commitBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// Modify.
	body = bytes.NewBufferString("v2")
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("PUT", "/api/v1/spaces/s1/git/files/f.txt", body)
	r.ServeHTTP(w, req)

	// Diff.
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/spaces/s1/git/diff", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Contains(t, resp["diff"], "f.txt")
}

func TestHandlers_MissingSpaceID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	m := NewManager(dir)
	h := NewHandlers(m)

	// Register without :id param to test missing space ID handling.
	r := gin.New()
	r.GET("/git/status", h.Status)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/git/status", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
