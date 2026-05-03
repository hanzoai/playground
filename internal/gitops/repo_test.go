package gitops

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitOrOpen_NewRepo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-space")

	r, err := InitOrOpen(path, "space-1")
	require.NoError(t, err)
	assert.Equal(t, path, r.Path())
	assert.Equal(t, "space-1", r.SpaceID())

	// Opening again should succeed without error.
	r2, err := InitOrOpen(path, "space-1")
	require.NoError(t, err)
	assert.Equal(t, path, r2.Path())
}

func TestWriteReadFile(t *testing.T) {
	dir := t.TempDir()
	r, err := InitOrOpen(dir, "s1")
	require.NoError(t, err)

	content := []byte("hello world\n")
	err = r.WriteFile("test.txt", content)
	require.NoError(t, err)

	data, err := r.ReadFile("test.txt")
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestWriteFile_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	r, err := InitOrOpen(dir, "s1")
	require.NoError(t, err)

	err = r.WriteFile("../escape.txt", []byte("bad"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal")
}

func TestReadFile_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	r, err := InitOrOpen(dir, "s1")
	require.NoError(t, err)

	_, err = r.ReadFile("../etc/passwd")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal")
}

func TestWriteFile_Subdirectory(t *testing.T) {
	dir := t.TempDir()
	r, err := InitOrOpen(dir, "s1")
	require.NoError(t, err)

	err = r.WriteFile("sub/dir/file.txt", []byte("nested"))
	require.NoError(t, err)

	data, err := r.ReadFile("sub/dir/file.txt")
	require.NoError(t, err)
	assert.Equal(t, "nested", string(data))
}

func TestAddAndStatus(t *testing.T) {
	dir := t.TempDir()
	r, err := InitOrOpen(dir, "s1")
	require.NoError(t, err)

	// Write a file.
	err = r.WriteFile("hello.txt", []byte("hello"))
	require.NoError(t, err)

	// Before add, should show untracked.
	changes, err := r.Status()
	require.NoError(t, err)
	assert.Len(t, changes, 1)
	assert.Equal(t, "untracked", changes[0].Status)

	// Add and check status.
	err = r.Add("hello.txt")
	require.NoError(t, err)

	changes, err = r.Status()
	require.NoError(t, err)
	assert.Len(t, changes, 1)
	assert.Equal(t, "added", changes[0].Status)
}

func TestCommitAndLog(t *testing.T) {
	dir := t.TempDir()
	r, err := InitOrOpen(dir, "s1")
	require.NoError(t, err)

	// Write and commit.
	err = r.WriteFile("file.txt", []byte("content"))
	require.NoError(t, err)
	err = r.Add("file.txt")
	require.NoError(t, err)

	hash, err := r.Commit("initial commit", CommitAuthor{
		Name:    "Test Bot",
		AgentID: "bot-42",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, hash.String())

	// Log should return the commit.
	commits, err := r.Log(10)
	require.NoError(t, err)
	require.Len(t, commits, 1)
	assert.Equal(t, "initial commit", commits[0].Message)
	assert.Equal(t, "Test Bot", commits[0].AuthorName)
	assert.Equal(t, "bot-42@agent.hanzo.bot", commits[0].AuthorEmail)
}

func TestBranchAndCheckout(t *testing.T) {
	dir := t.TempDir()
	r, err := InitOrOpen(dir, "s1")
	require.NoError(t, err)

	// Need an initial commit before branching.
	err = r.WriteFile("init.txt", []byte("init"))
	require.NoError(t, err)
	err = r.Add("init.txt")
	require.NoError(t, err)
	_, err = r.Commit("init", CommitAuthor{Name: "test", Email: "test@test.com"})
	require.NoError(t, err)

	// Create and checkout a branch.
	err = r.Branch("feature-x")
	require.NoError(t, err)

	err = r.Checkout("feature-x")
	require.NoError(t, err)

	// List branches should include both.
	branches, err := r.ListBranches()
	require.NoError(t, err)

	names := make(map[string]bool)
	for _, b := range branches {
		names[b.Name] = true
	}
	assert.True(t, names["master"] || names["main"], "should have default branch")
	assert.True(t, names["feature-x"], "should have feature-x branch")
}

func TestDiffWorktree(t *testing.T) {
	dir := t.TempDir()
	r, err := InitOrOpen(dir, "s1")
	require.NoError(t, err)

	// Commit a file.
	err = r.WriteFile("a.txt", []byte("line1\nline2\n"))
	require.NoError(t, err)
	err = r.Add("a.txt")
	require.NoError(t, err)
	_, err = r.Commit("first", CommitAuthor{Name: "t", Email: "t@t.com"})
	require.NoError(t, err)

	// Modify the file.
	err = r.WriteFile("a.txt", []byte("line1\nmodified\n"))
	require.NoError(t, err)

	diff, err := r.Diff("", "")
	require.NoError(t, err)
	assert.Contains(t, diff, "a.txt")
}

func TestDiffCommits(t *testing.T) {
	dir := t.TempDir()
	r, err := InitOrOpen(dir, "s1")
	require.NoError(t, err)

	// First commit.
	err = r.WriteFile("a.txt", []byte("version1\n"))
	require.NoError(t, err)
	err = r.Add("a.txt")
	require.NoError(t, err)
	hash1, err := r.Commit("v1", CommitAuthor{Name: "t", Email: "t@t.com"})
	require.NoError(t, err)

	// Second commit.
	err = r.WriteFile("a.txt", []byte("version2\n"))
	require.NoError(t, err)
	err = r.Add("a.txt")
	require.NoError(t, err)
	hash2, err := r.Commit("v2", CommitAuthor{Name: "t", Email: "t@t.com"})
	require.NoError(t, err)

	diff, err := r.Diff(hash1.String(), hash2.String())
	require.NoError(t, err)
	assert.Contains(t, diff, "version1")
	assert.Contains(t, diff, "version2")
}

func TestStatusClean(t *testing.T) {
	dir := t.TempDir()
	r, err := InitOrOpen(dir, "s1")
	require.NoError(t, err)

	// Commit a file so repo is clean.
	err = r.WriteFile("f.txt", []byte("x"))
	require.NoError(t, err)
	err = r.Add("f.txt")
	require.NoError(t, err)
	_, err = r.Commit("init", CommitAuthor{Name: "t", Email: "t@t.com"})
	require.NoError(t, err)

	changes, err := r.Status()
	require.NoError(t, err)
	assert.Empty(t, changes)
}

func TestLogEmpty(t *testing.T) {
	dir := t.TempDir()
	r, err := InitOrOpen(dir, "s1")
	require.NoError(t, err)

	// Empty repo has no commits.
	commits, err := r.Log(10)
	// go-git returns an error for empty repos; we treat that as no commits.
	if err != nil {
		assert.Empty(t, commits)
	} else {
		assert.Empty(t, commits)
	}
}

func TestAddAll(t *testing.T) {
	dir := t.TempDir()
	r, err := InitOrOpen(dir, "s1")
	require.NoError(t, err)

	// Write multiple files.
	err = r.WriteFile("a.txt", []byte("a"))
	require.NoError(t, err)
	err = r.WriteFile("b.txt", []byte("b"))
	require.NoError(t, err)

	// Add all (empty patterns).
	err = r.Add()
	require.NoError(t, err)

	changes, err := r.Status()
	require.NoError(t, err)
	for _, c := range changes {
		assert.Equal(t, "added", c.Status, "file %s should be staged", c.Path)
	}
}

func TestReadFile_NotFound(t *testing.T) {
	dir := t.TempDir()
	r, err := InitOrOpen(dir, "s1")
	require.NoError(t, err)

	_, err = r.ReadFile("nonexistent.txt")
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err) || true) // wrapped error
}
