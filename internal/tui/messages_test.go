package tui

import (
	"github.com/plinde/gwtui/internal/cache"
	"os"
	"os/exec"
	"path/filepath"
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

func TestDoCleanup_UsesWorktreeRepoPath(t *testing.T) {
	root := t.TempDir()
	repoPath := filepath.Join(root, "owner")
	wrongRepoPath := filepath.Join(root, "wrong")
	worktreePath := filepath.Join(root, "owner--feature")
	if err := os.Mkdir(repoPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(wrongRepoPath, 0o755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoPath)
	runGitForTUITest(t, repoPath, "worktree", "add", "-b", "feature", worktreePath)

	rows := []WorktreeRow{
		{
			Worktree: git.Worktree{
				Path:     worktreePath,
				Branch:   "feature",
				RepoPath: repoPath,
			},
			Selected: true,
		},
	}

	cmd := doCleanup(wrongRepoPath, rows)
	msg := cmd()
	done, ok := msg.(cleanupDoneMsg)
	if !ok {
		t.Fatalf("expected cleanupDoneMsg, got %T", msg)
	}
	if len(done.results) != 1 {
		t.Fatalf("expected 1 cleanup result, got %d", len(done.results))
	}
	if !done.results[0].Success {
		t.Fatalf("expected cleanup through row RepoPath to succeed, got error: %s", done.results[0].Error)
	}
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Fatalf("expected worktree path to be removed, stat err=%v", err)
	}
}

func TestDoLoad_ReturnsNonNilCmd(t *testing.T) {
	cmd := doLoad("/tmp/fakerepo", cache.Options{Disabled: true})
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd from doLoad")
	}
}

func runGitForTUITest(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}

func TestDoLoad_WithTempGitRepo(t *testing.T) {
	repoPath := t.TempDir()

	// Initialize a bare-minimum git repo
	initGitRepo(t, repoPath)

	cmd := doLoad(repoPath, cache.Options{Disabled: true})
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
	if len(done.rows) == 0 {
		t.Error("expected at least one row from initialized repo")
	}
	if done.rows[0].Worktree.RepoPath != repoPath {
		t.Errorf("expected row repo path %q, got %q", repoPath, done.rows[0].Worktree.RepoPath)
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
