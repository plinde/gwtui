package git

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// Worktree represents a single git worktree entry.
type Worktree struct {
	Path      string // Absolute filesystem path
	Branch    string // Branch name (e.g., "asm/v1.0.3")
	CommitSHA string // Short SHA
	IsMain    bool   // True if this is the main checkout (not a linked worktree)
	IsBare    bool   // True if bare repo entry
}

// List returns all worktrees for the repo at repoPath.
// Parses `git worktree list --porcelain`.
func List(repoPath string) ([]Worktree, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parsePorcelain(string(out), repoPath)
}

func parsePorcelain(output string, repoPath string) ([]Worktree, error) {
	var worktrees []Worktree
	var current Worktree
	mainPath := ""

	// Determine the main worktree path (first "worktree" entry)
	firstEntry := true

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "worktree "):
			if current.Path != "" {
				worktrees = append(worktrees, current)
			}
			current = Worktree{Path: strings.TrimPrefix(line, "worktree ")}
			if firstEntry {
				mainPath = current.Path
				firstEntry = false
			}

		case strings.HasPrefix(line, "HEAD "):
			sha := strings.TrimPrefix(line, "HEAD ")
			if len(sha) > 8 {
				sha = sha[:8]
			}
			current.CommitSHA = sha

		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimPrefix(line, "branch ")
			// Strip refs/heads/ prefix
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")

		case line == "bare":
			current.IsBare = true

		case line == "detached":
			// HEAD is detached, branch stays empty
		}
	}
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	// Determine the default branch to mark the main worktree
	defaultBranch := detectDefaultBranch(repoPath)

	for i := range worktrees {
		wt := &worktrees[i]
		// Main worktree: lives at the repo root path OR is on the default branch
		if wt.Path == mainPath || wt.Branch == defaultBranch {
			wt.IsMain = true
		}
	}

	return worktrees, nil
}

// detectDefaultBranch returns the default branch name (e.g. "main" or "master").
func detectDefaultBranch(repoPath string) string {
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "main" // fallback
	}
	ref := strings.TrimSpace(string(out))
	return filepath.Base(ref)
}
