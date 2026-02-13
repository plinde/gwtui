package git

import (
	"fmt"
	"os/exec"
)

// CleanupResult holds the outcome of removing a worktree and its branch.
type CleanupResult struct {
	Worktree Worktree
	Success  bool
	Error    string
}

// RemoveWorktree removes a worktree directory and deletes its branch.
// Uses --force to handle dirty worktrees.
func RemoveWorktree(repoPath string, wt Worktree) CleanupResult {
	result := CleanupResult{Worktree: wt}

	// Remove the worktree
	cmd := exec.Command("git", "worktree", "remove", "--force", wt.Path)
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		result.Error = fmt.Sprintf("worktree remove: %s (%s)", err, string(out))
		return result
	}

	// Delete the branch (use -D to force-delete unmerged branches)
	if wt.Branch != "" {
		cmd = exec.Command("git", "branch", "-D", wt.Branch)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			result.Error = fmt.Sprintf("branch delete: %s (%s)", err, string(out))
			return result
		}
	}

	result.Success = true
	return result
}
