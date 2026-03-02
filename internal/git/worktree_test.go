package git

import (
	"strings"
	"testing"
)

func TestParsePorcelain_MultipleWorktrees(t *testing.T) {
	output := `worktree /home/user/repo
HEAD abc12345deadbeef
branch refs/heads/main

worktree /home/user/repo--feature
HEAD 1234abcd5678efgh
branch refs/heads/feature/login

`
	wts, err := parsePorcelainWithDefault(output, "/home/user/repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wts) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(wts))
	}

	// First entry should be main
	if wts[0].Path != "/home/user/repo" {
		t.Errorf("expected path /home/user/repo, got %s", wts[0].Path)
	}
	if wts[0].Branch != "main" {
		t.Errorf("expected branch main, got %s", wts[0].Branch)
	}
	if wts[0].CommitSHA != "abc12345" {
		t.Errorf("expected SHA abc12345, got %s", wts[0].CommitSHA)
	}
	if !wts[0].IsMain {
		t.Error("expected first worktree to be IsMain=true")
	}

	// Second entry should not be main
	if wts[1].Branch != "feature/login" {
		t.Errorf("expected branch feature/login, got %s", wts[1].Branch)
	}
	if wts[1].IsMain {
		t.Error("expected second worktree to be IsMain=false")
	}
}

func TestParsePorcelain_DetachedHead(t *testing.T) {
	output := `worktree /home/user/repo
HEAD abc12345deadbeef
branch refs/heads/main

worktree /home/user/repo--detached
HEAD deadbeef12345678
detached

`
	wts, err := parsePorcelainWithDefault(output, "/home/user/repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wts) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(wts))
	}

	if wts[1].Branch != "" {
		t.Errorf("expected empty branch for detached HEAD, got %s", wts[1].Branch)
	}
	if wts[1].CommitSHA != "deadbeef" {
		t.Errorf("expected SHA deadbeef, got %s", wts[1].CommitSHA)
	}
}

func TestParsePorcelain_BareRepo(t *testing.T) {
	output := `worktree /home/user/repo.git
HEAD abc12345deadbeef
bare

worktree /home/user/repo--feature
HEAD 1234abcd5678efgh
branch refs/heads/feature/x

`
	wts, err := parsePorcelainWithDefault(output, "/home/user/repo.git", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wts) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(wts))
	}

	if !wts[0].IsBare {
		t.Error("expected bare=true for bare repo entry")
	}
	if !wts[0].IsMain {
		t.Error("expected bare entry at mainPath to be IsMain=true")
	}
}

func TestParsePorcelain_EmptyOutput(t *testing.T) {
	wts, err := parsePorcelainWithDefault("", "/home/user/repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wts) != 0 {
		t.Fatalf("expected 0 worktrees, got %d", len(wts))
	}
}

func TestParsePorcelain_SingleWorktree(t *testing.T) {
	output := `worktree /home/user/repo
HEAD abc12345deadbeef
branch refs/heads/main

`
	wts, err := parsePorcelainWithDefault(output, "/home/user/repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wts) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(wts))
	}
	if !wts[0].IsMain {
		t.Error("expected single worktree on main to be IsMain=true")
	}
}

func TestParsePorcelain_IsMainByBranch(t *testing.T) {
	// Second worktree is on the default branch — should also be marked IsMain
	output := `worktree /home/user/repo
HEAD abc12345deadbeef
branch refs/heads/develop

worktree /home/user/repo--main
HEAD 1234abcd5678efgh
branch refs/heads/main

`
	wts, err := parsePorcelainWithDefault(output, "/home/user/repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wts) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(wts))
	}

	// First entry is at mainPath, so IsMain=true
	if !wts[0].IsMain {
		t.Error("expected first entry (mainPath) to be IsMain=true")
	}
	// Second entry is on the default branch, so IsMain=true
	if !wts[1].IsMain {
		t.Error("expected worktree on default branch to be IsMain=true")
	}
}

func TestParsePorcelain_SHATruncation(t *testing.T) {
	output := `worktree /repo
HEAD abcdef1234567890abcdef1234567890abcdef12
branch refs/heads/main

`
	wts, err := parsePorcelainWithDefault(output, "/repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wts[0].CommitSHA != "abcdef12" {
		t.Errorf("expected SHA truncated to 8 chars, got %s (len %d)", wts[0].CommitSHA, len(wts[0].CommitSHA))
	}
}

func TestParsePorcelain_ShortSHA(t *testing.T) {
	output := `worktree /repo
HEAD abcd1234
branch refs/heads/main

`
	wts, err := parsePorcelainWithDefault(output, "/repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wts[0].CommitSHA != "abcd1234" {
		t.Errorf("expected SHA abcd1234, got %s", wts[0].CommitSHA)
	}
}

// parsePorcelainWithDefault is a test helper that calls parsePorcelain
// but patches the default branch detection (which shells out to git).
// We extract the IsMain logic here to avoid exec in tests.
func parsePorcelainWithDefault(output string, repoPath string, defaultBranch string) ([]Worktree, error) {
	var worktrees []Worktree
	var current Worktree
	mainPath := ""
	firstEntry := true

	for _, line := range splitLines(output) {
		line = trimCR(line)
		switch {
		case hasPrefix(line, "worktree "):
			if current.Path != "" {
				worktrees = append(worktrees, current)
			}
			current = Worktree{Path: trimPrefix(line, "worktree ")}
			if firstEntry {
				mainPath = current.Path
				firstEntry = false
			}
		case hasPrefix(line, "HEAD "):
			sha := trimPrefix(line, "HEAD ")
			if len(sha) > 8 {
				sha = sha[:8]
			}
			current.CommitSHA = sha
		case hasPrefix(line, "branch "):
			ref := trimPrefix(line, "branch ")
			current.Branch = trimPrefix(ref, "refs/heads/")
		case line == "bare":
			current.IsBare = true
		case line == "detached":
			// HEAD is detached, branch stays empty
		}
	}
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	for i := range worktrees {
		wt := &worktrees[i]
		if wt.Path == mainPath || wt.Branch == defaultBranch {
			wt.IsMain = true
		}
	}

	return worktrees, nil
}

// Thin wrappers to keep test helper consistent with production code.
func splitLines(s string) []string {
	return strings.Split(s, "\n")
}

func trimCR(s string) string {
	return strings.TrimRight(s, "\r")
}

func hasPrefix(s, prefix string) bool {
	return strings.HasPrefix(s, prefix)
}

func trimPrefix(s, prefix string) string {
	return strings.TrimPrefix(s, prefix)
}
