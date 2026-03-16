package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupTestRepo creates a temporary git repo with an initial commit.
func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s (%s)", args, err, out)
		}
	}

	run("init", "-b", "main")
	run("commit", "--allow-empty", "-m", "init")

	return dir
}

func TestRemoveWorktree_Success(t *testing.T) {
	repo := setupTestRepo(t)

	// Create a worktree with a new branch
	wtPath := filepath.Join(t.TempDir(), "wt-feature")
	cmd := exec.Command("git", "worktree", "add", "-b", "feature", wtPath)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("worktree add failed: %s (%s)", err, out)
	}

	wt := Worktree{
		Path:   wtPath,
		Branch: "feature",
	}

	result := RemoveWorktree(repo, wt)

	if !result.Success {
		t.Fatalf("expected Success=true, got false; error: %s", result.Error)
	}
	if result.Error != "" {
		t.Errorf("expected no error, got: %s", result.Error)
	}

	// Verify the worktree directory is gone
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("expected worktree directory to be removed, but it still exists")
	}

	// Verify the branch is gone
	cmd = exec.Command("git", "branch", "--list", "feature")
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git branch --list failed: %v", err)
	}
	if len(out) > 0 {
		t.Errorf("expected branch 'feature' to be deleted, but it still exists")
	}
}

func TestRemoveWorktree_EmptyBranch(t *testing.T) {
	repo := setupTestRepo(t)

	// Create a worktree with a branch, but we will pass empty branch to RemoveWorktree
	wtPath := filepath.Join(t.TempDir(), "wt-nobranch")
	cmd := exec.Command("git", "worktree", "add", "-b", "temp-branch", wtPath)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("worktree add failed: %s (%s)", err, out)
	}

	wt := Worktree{
		Path:   wtPath,
		Branch: "", // empty branch name
	}

	result := RemoveWorktree(repo, wt)

	if !result.Success {
		t.Fatalf("expected Success=true, got false; error: %s", result.Error)
	}

	// Worktree directory should be removed
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("expected worktree directory to be removed")
	}

	// Branch should still exist since we passed empty branch name
	cmd = exec.Command("git", "branch", "--list", "temp-branch")
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git branch --list failed: %v", err)
	}
	if len(out) == 0 {
		t.Errorf("expected branch 'temp-branch' to still exist (empty branch skips delete)")
	}
}

func TestRemoveWorktree_NonExistentPath(t *testing.T) {
	repo := setupTestRepo(t)

	wt := Worktree{
		Path:   filepath.Join(repo, "nonexistent-worktree"),
		Branch: "ghost-branch",
	}

	result := RemoveWorktree(repo, wt)

	if result.Success {
		t.Error("expected Success=false for non-existent worktree path")
	}
	if result.Error == "" {
		t.Error("expected Error to be set for non-existent worktree path")
	}
}

func TestRemoveWorktree_ResultContainsInputWorktree(t *testing.T) {
	repo := setupTestRepo(t)

	wt := Worktree{
		Path:      "/some/path",
		Branch:    "some-branch",
		CommitSHA: "abc12345",
		IsMain:    false,
		IsBare:    false,
	}

	result := RemoveWorktree(repo, wt)

	// The result will have Success=false since the path doesn't exist,
	// but the Worktree field should match the input exactly.
	if result.Worktree.Path != wt.Path {
		t.Errorf("expected Worktree.Path=%q, got %q", wt.Path, result.Worktree.Path)
	}
	if result.Worktree.Branch != wt.Branch {
		t.Errorf("expected Worktree.Branch=%q, got %q", wt.Branch, result.Worktree.Branch)
	}
	if result.Worktree.CommitSHA != wt.CommitSHA {
		t.Errorf("expected Worktree.CommitSHA=%q, got %q", wt.CommitSHA, result.Worktree.CommitSHA)
	}
}
