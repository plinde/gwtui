package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRows_OrgRootAnnotatesRepositoryNames(t *testing.T) {
	root := t.TempDir()
	api := filepath.Join(root, "api")
	web := filepath.Join(root, "web")
	if err := os.Mkdir(api, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(web, 0o755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, api)
	initGitRepo(t, web)

	rows, _, err := LoadRows(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	if got := BranchLabel(rows[0]); got != "api:test-main" {
		t.Errorf("expected first branch label api:test-main, got %q", got)
	}
	if got := BranchLabel(rows[1]); got != "web:test-main" {
		t.Errorf("expected second branch label web:test-main, got %q", got)
	}
	if rows[0].Worktree.RepoPath != api {
		t.Errorf("expected first repo path %q, got %q", api, rows[0].Worktree.RepoPath)
	}
	if rows[1].Worktree.RepoPath != web {
		t.Errorf("expected second repo path %q, got %q", web, rows[1].Worktree.RepoPath)
	}
}
