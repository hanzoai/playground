package gitops

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Repo wraps a go-git repository for a space/base workspace.
type Repo struct {
	repo    *git.Repository
	path    string
	spaceID string
}

// Path returns the filesystem path of the repository.
func (r *Repo) Path() string { return r.path }

// SpaceID returns the space this repo belongs to.
func (r *Repo) SpaceID() string { return r.spaceID }

// InitOrOpen initializes a new git repo at path, or opens an existing one.
func InitOrOpen(path, spaceID string) (*Repo, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("create repo dir: %w", err)
	}

	repo, err := git.PlainOpen(path)
	if err == git.ErrRepositoryNotExists {
		repo, err = git.PlainInit(path, false)
		if err != nil {
			return nil, fmt.Errorf("init repo: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	return &Repo{repo: repo, path: path, spaceID: spaceID}, nil
}

// Clone clones a remote repository into the space workspace.
func Clone(ctx context.Context, url, path, spaceID string, opts CloneOpts) (*Repo, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("create clone dir: %w", err)
	}

	cloneOpts := &git.CloneOptions{
		URL:      url,
		Progress: opts.Progress,
	}

	if opts.Branch != "" {
		cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(opts.Branch)
		cloneOpts.SingleBranch = true
	}
	if opts.Depth > 0 {
		cloneOpts.Depth = opts.Depth
	}

	auth, err := opts.Auth.transportAuth()
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}
	cloneOpts.Auth = auth

	repo, err := git.PlainCloneContext(ctx, path, false, cloneOpts)
	if err != nil {
		return nil, fmt.Errorf("clone: %w", err)
	}

	return &Repo{repo: repo, path: path, spaceID: spaceID}, nil
}

// Status returns the working tree status (modified, added, deleted files).
func (r *Repo) Status() ([]FileChange, error) {
	wt, err := r.repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("worktree: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return nil, fmt.Errorf("status: %w", err)
	}

	var changes []FileChange
	for path, s := range status {
		fc := FileChange{Path: path}
		// Use staging status first, fallback to worktree status.
		code := s.Staging
		if code == git.Unmodified {
			code = s.Worktree
		}
		switch code {
		case git.Added:
			fc.Status = "added"
		case git.Modified:
			fc.Status = "modified"
		case git.Deleted:
			fc.Status = "deleted"
		case git.Renamed:
			fc.Status = "renamed"
		case git.Copied:
			fc.Status = "added"
		case git.Untracked:
			fc.Status = "untracked"
		default:
			fc.Status = "modified"
		}
		changes = append(changes, fc)
	}
	return changes, nil
}

// Add stages files by path patterns (like git add).
// If patterns is empty, stages all changes.
func (r *Repo) Add(patterns ...string) error {
	wt, err := r.repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}

	if len(patterns) == 0 {
		// Stage everything.
		_, err = wt.Add(".")
		return err
	}

	for _, p := range patterns {
		if _, err := wt.Add(p); err != nil {
			return fmt.Errorf("add %q: %w", p, err)
		}
	}
	return nil
}

// Commit creates a commit with the given message and author info.
func (r *Repo) Commit(message string, author CommitAuthor) (plumbing.Hash, error) {
	wt, err := r.repo.Worktree()
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("worktree: %w", err)
	}

	email := author.Email
	if email == "" && author.AgentID != "" {
		email = author.AgentID + "@agent.hanzo.bot"
	}
	name := author.Name
	if name == "" {
		name = author.AgentID
	}

	now := time.Now()
	hash, err := wt.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  name,
			Email: email,
			When:  now,
		},
	})
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("commit: %w", err)
	}

	return hash, nil
}

// Log returns recent commit history. max=0 returns all commits.
func (r *Repo) Log(max int) ([]CommitInfo, error) {
	logOpts := &git.LogOptions{
		Order: git.LogOrderCommitterTime,
	}

	iter, err := r.repo.Log(logOpts)
	if err != nil {
		return nil, fmt.Errorf("log: %w", err)
	}
	defer iter.Close()

	var commits []CommitInfo
	count := 0
	err = iter.ForEach(func(c *object.Commit) error {
		if max > 0 && count >= max {
			return fmt.Errorf("stop") // break out of iteration
		}
		commits = append(commits, CommitInfo{
			Hash:        c.Hash.String(),
			Message:     strings.TrimSpace(c.Message),
			AuthorName:  c.Author.Name,
			AuthorEmail: c.Author.Email,
			Timestamp:   c.Author.When.Format(time.RFC3339),
		})
		count++
		return nil
	})
	// The "stop" error is expected for limiting results.
	if err != nil && err.Error() != "stop" {
		return nil, fmt.Errorf("log iterate: %w", err)
	}

	return commits, nil
}

// Branch creates a new branch from HEAD.
func (r *Repo) Branch(name string) error {
	headRef, err := r.repo.Head()
	if err != nil {
		return fmt.Errorf("resolve HEAD: %w", err)
	}

	ref := plumbing.NewHashReference(
		plumbing.NewBranchReferenceName(name),
		headRef.Hash(),
	)
	return r.repo.Storer.SetReference(ref)
}

// Checkout switches to a branch.
func (r *Repo) Checkout(branch string) error {
	wt, err := r.repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}

	return wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branch),
	})
}

// ListBranches returns all local and remote branches.
func (r *Repo) ListBranches() ([]BranchInfo, error) {
	var branches []BranchInfo

	// Local branches.
	localIter, err := r.repo.Branches()
	if err != nil {
		return nil, fmt.Errorf("branches: %w", err)
	}
	defer localIter.Close()
	err = localIter.ForEach(func(ref *plumbing.Reference) error {
		branches = append(branches, BranchInfo{
			Name:     ref.Name().Short(),
			IsRemote: false,
			Hash:     ref.Hash().String(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Remote branches.
	remoteIter, err := r.repo.References()
	if err != nil {
		return nil, fmt.Errorf("references: %w", err)
	}
	defer remoteIter.Close()
	err = remoteIter.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().IsRemote() {
			branches = append(branches, BranchInfo{
				Name:     ref.Name().Short(),
				IsRemote: true,
				Hash:     ref.Hash().String(),
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return branches, nil
}

// Push pushes to the remote.
func (r *Repo) Push(ctx context.Context, opts PushOpts) error {
	remote := opts.Remote
	if remote == "" {
		remote = "origin"
	}

	pushOpts := &git.PushOptions{
		RemoteName: remote,
		Force:      opts.Force,
	}

	if opts.Branch != "" {
		refSpec := config.RefSpec(fmt.Sprintf(
			"refs/heads/%s:refs/heads/%s",
			opts.Branch, opts.Branch,
		))
		pushOpts.RefSpecs = []config.RefSpec{refSpec}
	}

	auth, err := opts.Auth.transportAuth()
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	pushOpts.Auth = auth

	err = r.repo.PushContext(ctx, pushOpts)
	if err == git.NoErrAlreadyUpToDate {
		return nil
	}
	return err
}

// Pull fetches and merges from the remote.
func (r *Repo) Pull(ctx context.Context, opts PullOpts) error {
	wt, err := r.repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}

	remote := opts.Remote
	if remote == "" {
		remote = "origin"
	}

	pullOpts := &git.PullOptions{
		RemoteName: remote,
	}

	if opts.Branch != "" {
		pullOpts.ReferenceName = plumbing.NewBranchReferenceName(opts.Branch)
	}

	auth, err := opts.Auth.transportAuth()
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	pullOpts.Auth = auth

	err = wt.PullContext(ctx, pullOpts)
	if err == git.NoErrAlreadyUpToDate {
		return nil
	}
	return err
}

// Diff returns the diff between the working tree and HEAD.
// When from and to are empty, diffs HEAD against the working tree.
// When from and to are commit hashes, diffs between those commits.
func (r *Repo) Diff(from, to string) (string, error) {
	if from == "" && to == "" {
		return r.diffWorktree()
	}
	return r.diffCommits(from, to)
}

// diffWorktree produces a unified diff of uncommitted changes vs HEAD.
func (r *Repo) diffWorktree() (string, error) {
	headRef, err := r.repo.Head()
	if err != nil {
		// No commits yet -- everything is new.
		return "", nil
	}

	headCommit, err := r.repo.CommitObject(headRef.Hash())
	if err != nil {
		return "", fmt.Errorf("head commit: %w", err)
	}

	headTree, err := headCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("head tree: %w", err)
	}

	// Build a tree from the current worktree index for comparison.
	wt, err := r.repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("worktree: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return "", fmt.Errorf("status: %w", err)
	}

	var buf bytes.Buffer
	for path, s := range status {
		if s.Worktree == git.Unmodified && s.Staging == git.Unmodified {
			continue
		}

		// Get old content from HEAD tree.
		var oldContent string
		f, err := headTree.File(path)
		if err == nil {
			oldContent, _ = f.Contents()
		}

		// Get new content from filesystem.
		var newContent string
		fullPath := filepath.Join(r.path, path)
		data, err := os.ReadFile(fullPath)
		if err == nil {
			newContent = string(data)
		}

		if oldContent != newContent {
			fmt.Fprintf(&buf, "--- a/%s\n+++ b/%s\n", path, path)
			// Simple line diff indicator -- not a full unified diff algorithm,
			// but sufficient for API display. Callers needing patch-level
			// detail should use the commit-to-commit diff path.
			oldLines := strings.Split(oldContent, "\n")
			newLines := strings.Split(newContent, "\n")
			fmt.Fprintf(&buf, "@@ old: %d lines, new: %d lines @@\n", len(oldLines), len(newLines))
			for _, l := range oldLines {
				if l != "" {
					fmt.Fprintf(&buf, "- %s\n", l)
				}
			}
			for _, l := range newLines {
				if l != "" {
					fmt.Fprintf(&buf, "+ %s\n", l)
				}
			}
		}
	}

	return buf.String(), nil
}

// diffCommits returns the diff between two commit hashes.
func (r *Repo) diffCommits(from, to string) (string, error) {
	fromHash := plumbing.NewHash(from)
	toHash := plumbing.NewHash(to)

	fromCommit, err := r.repo.CommitObject(fromHash)
	if err != nil {
		return "", fmt.Errorf("from commit %s: %w", from, err)
	}

	toCommit, err := r.repo.CommitObject(toHash)
	if err != nil {
		return "", fmt.Errorf("to commit %s: %w", to, err)
	}

	fromTree, err := fromCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("from tree: %w", err)
	}

	toTree, err := toCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("to tree: %w", err)
	}

	changes, err := fromTree.Diff(toTree)
	if err != nil {
		return "", fmt.Errorf("diff: %w", err)
	}

	patch, err := changes.Patch()
	if err != nil {
		return "", fmt.Errorf("patch: %w", err)
	}

	return patch.String(), nil
}

// ReadFile reads a file from the working tree.
func (r *Repo) ReadFile(path string) ([]byte, error) {
	clean := filepath.Clean(path)
	if strings.Contains(clean, "..") {
		return nil, fmt.Errorf("path traversal not allowed")
	}
	fullPath := filepath.Join(r.path, clean)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	return data, nil
}

// WriteFile writes a file to the working tree.
func (r *Repo) WriteFile(path string, content []byte) error {
	clean := filepath.Clean(path)
	if strings.Contains(clean, "..") {
		return fmt.Errorf("path traversal not allowed")
	}
	fullPath := filepath.Join(r.path, clean)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	return nil
}
