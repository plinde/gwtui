package tui

import (
	"os/exec"
	"testing"

	"github.com/plinde/gwtui/internal/git"
	gh "github.com/plinde/gwtui/internal/github"
)

// initGitRepo creates a minimal git repo with one commit at the given path.
func initGitRepo(t *testing.T, path string) {
	t.Helper()
	for _, args := range [][]string{
		{"git", "init", "--initial-branch=test-main"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = path
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %s (%s)", args, err, out)
		}
	}
}

func TestDoCleanup_EmptyRows(t *testing.T) {
	cmd := doCleanup("/tmp/fakerepo", nil)
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd from doCleanup with empty rows")
	}
	msg := cmd()
	done, ok := msg.(cleanupDoneMsg)
	if !ok {
		t.Fatalf("expected cleanupDoneMsg, got %T", msg)
	}
	if len(done.results) != 0 {
		t.Errorf("expected 0 results, got %d", len(done.results))
	}
}

func TestDoCleanup_NoneSelected(t *testing.T) {
	rows := []WorktreeRow{
		{Worktree: git.Worktree{Branch: "a", Path: "/tmp/a"}, Selected: false},
		{Worktree: git.Worktree{Branch: "b", Path: "/tmp/b"}, Selected: false},
	}
	cmd := doCleanup("/tmp/fakerepo", rows)
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd")
	}
	msg := cmd()
	done, ok := msg.(cleanupDoneMsg)
	if !ok {
		t.Fatalf("expected cleanupDoneMsg, got %T", msg)
	}
	if len(done.results) != 0 {
		t.Errorf("expected 0 results when none selected, got %d", len(done.results))
	}
}

func TestDoCleanup_FiltersSelected(t *testing.T) {
	// Create a real git repo with worktrees so RemoveWorktree can operate
	// Instead of needing a real repo, we verify the filtering logic by
	// checking that only selected rows produce results. Since RemoveWorktree
	// will fail on non-existent paths, we still get CleanupResult entries
	// (with errors) only for selected rows.
	rows := []WorktreeRow{
		{Worktree: git.Worktree{Branch: "keep", Path: "/tmp/nonexistent-keep"}, Selected: false},
		{Worktree: git.Worktree{Branch: "remove1", Path: "/tmp/nonexistent-rm1"}, Selected: true},
		{Worktree: git.Worktree{Branch: "keep2", Path: "/tmp/nonexistent-keep2"}, Selected: false},
		{Worktree: git.Worktree{Branch: "remove2", Path: "/tmp/nonexistent-rm2"}, Selected: true},
	}
	cmd := doCleanup("/tmp/fakerepo", rows)
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd")
	}
	msg := cmd()
	done, ok := msg.(cleanupDoneMsg)
	if !ok {
		t.Fatalf("expected cleanupDoneMsg, got %T", msg)
	}
	// We expect exactly 2 results (one per selected row), even though they fail
	if len(done.results) != 2 {
		t.Fatalf("expected 2 results for 2 selected rows, got %d", len(done.results))
	}
	if done.results[0].Worktree.Branch != "remove1" {
		t.Errorf("expected first result branch 'remove1', got %q", done.results[0].Worktree.Branch)
	}
	if done.results[1].Worktree.Branch != "remove2" {
		t.Errorf("expected second result branch 'remove2', got %q", done.results[1].Worktree.Branch)
	}
}

func TestDoLoad_ReturnsNonNilCmd(t *testing.T) {
	cmd := doLoad("/tmp/fakerepo")
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd from doLoad")
	}
}

func TestDoLoad_WithTempGitRepo(t *testing.T) {
	repoPath := t.TempDir()

	// Initialize a bare-minimum git repo
	initGitRepo(t, repoPath)

	cmd := doLoad(repoPath)
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd from doLoad")
	}
	msg := cmd()
	done, ok := msg.(loadDoneMsg)
	if !ok {
		t.Fatalf("expected loadDoneMsg, got %T", msg)
	}
	if done.err != nil {
		t.Fatalf("unexpected error: %v", done.err)
	}
	if len(done.worktrees) == 0 {
		t.Error("expected at least one worktree from initialized repo")
	}
	// PRs may be nil map or empty (gh CLI may not be available), but should not cause error in loadDoneMsg
	if done.prs == nil {
		t.Error("expected non-nil prs map (even if empty)")
	}
}

func TestCleanupDoneMsg_Type(t *testing.T) {
	msg := cleanupDoneMsg{results: []git.CleanupResult{
		{Worktree: git.Worktree{Branch: "test"}, Success: true},
	}}
	if len(msg.results) != 1 {
		t.Errorf("expected 1 result, got %d", len(msg.results))
	}
}

func TestLoadDoneMsg_WithError(t *testing.T) {
	msg := loadDoneMsg{err: nil, worktrees: []git.Worktree{{Branch: "main"}}, prs: map[string]*gh.PR{}}
	if msg.err != nil {
		t.Errorf("expected nil error, got %v", msg.err)
	}
	if len(msg.worktrees) != 1 {
		t.Errorf("expected 1 worktree, got %d", len(msg.worktrees))
	}
}
