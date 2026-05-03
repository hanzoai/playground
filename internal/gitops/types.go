package gitops

import (
	"fmt"
	"io"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

// CommitAuthor identifies who made a commit.
type CommitAuthor struct {
	Name    string `json:"name"`     // Agent display name or human name
	Email   string `json:"email"`    // agent-{id}@agent.hanzo.bot or human email
	AgentID string `json:"agent_id"` // Bot ID that made the commit (empty for human)
}

// CloneOpts configures a clone operation.
type CloneOpts struct {
	Branch   string    `json:"branch"`
	Depth    int       `json:"depth"` // 0 = full clone
	Auth     AuthOpts  `json:"auth"`
	Progress io.Writer `json:"-"`
}

// PushOpts configures a push operation.
type PushOpts struct {
	Remote string   `json:"remote"` // default "origin"
	Branch string   `json:"branch"`
	Auth   AuthOpts `json:"auth"`
	Force  bool     `json:"force"`
}

// PullOpts configures a pull operation.
type PullOpts struct {
	Remote string   `json:"remote"`
	Branch string   `json:"branch"`
	Auth   AuthOpts `json:"auth"`
}

// AuthOpts specifies authentication for remote operations.
type AuthOpts struct {
	Type     string `json:"type"`     // "token", "ssh", "basic"
	Token    string `json:"token"`
	Username string `json:"username"`
	Password string `json:"password"`
	SSHKey   []byte `json:"ssh_key"`
}

// FileChange describes a changed file in the working tree.
type FileChange struct {
	Path    string `json:"path"`
	Status  string `json:"status"`   // "added", "modified", "deleted", "renamed", "untracked"
	OldPath string `json:"old_path"` // for renames
}

// CommitInfo is a serialisable summary of a commit.
type CommitInfo struct {
	Hash       string `json:"hash"`
	Message    string `json:"message"`
	AuthorName string `json:"author_name"`
	AuthorEmail string `json:"author_email"`
	Timestamp  string `json:"timestamp"` // RFC3339
}

// BranchInfo describes a branch.
type BranchInfo struct {
	Name     string `json:"name"`
	IsRemote bool   `json:"is_remote"`
	Hash     string `json:"hash"`
}

// transportAuth converts AuthOpts to a go-git transport.AuthMethod.
// Returns nil when no auth is configured (public repos).
func (a *AuthOpts) transportAuth() (transport.AuthMethod, error) {
	switch a.Type {
	case "token":
		return &githttp.BasicAuth{
			Username: "x-token-auth", // GitHub/GitLab accept any username with token
			Password: a.Token,
		}, nil
	case "basic":
		return &githttp.BasicAuth{
			Username: a.Username,
			Password: a.Password,
		}, nil
	case "ssh":
		if len(a.SSHKey) == 0 {
			return nil, fmt.Errorf("ssh auth requires a key")
		}
		keys, err := gitssh.NewPublicKeys("git", a.SSHKey, "")
		if err != nil {
			return nil, fmt.Errorf("invalid ssh key: %w", err)
		}
		return keys, nil
	case "":
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported auth type: %q", a.Type)
	}
}
