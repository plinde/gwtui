package tui

import (
	"os"
	"path/filepath"
	"strings"
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

	rows, _, err := LoadRows(root, "")
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

func TestLoadRows_RepositoryScopePrunesSiblingBeforeGraphQL(t *testing.T) {
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
	runGitForTUITest(t, api, "remote", "add", "origin", "git@github.com:Acme/api.git")
	runGitForTUITest(t, web, "remote", "add", "origin", "git@github.com:Acme/web.git")
	runGitForTUITest(t, api, "worktree", "add", "-b", "feature/api", filepath.Join(root, "api--feature"))
	runGitForTUITest(t, web, "worktree", "add", "-b", "feature/web", filepath.Join(root, "web--feature"))

	binDir := filepath.Join(root, "bin")
	if err := os.Mkdir(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	requestPath := filepath.Join(root, "request.json")
	ghPath := filepath.Join(binDir, "gh")
	script := `#!/bin/sh
cat > "$GWTUI_GRAPHQL_REQUEST"
printf '%s\n' '{"data":{"r0":{"b0":{"nodes":[]}},"rateLimit":{"cost":1}}}'
`
	if err := os.WriteFile(ghPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GWTUI_GRAPHQL_REQUEST", requestPath)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	rows, warnings, err := LoadRows(root, "api")
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", warnings)
	}
	if len(rows) != 2 {
		t.Fatalf("expected api main + worktree rows, got %d", len(rows))
	}
	for _, row := range rows {
		if row.Worktree.RepoName != "api" {
			t.Fatalf("loaded sibling repository row: %#v", row.Worktree)
		}
	}

	request, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(request), `"name0":"api"`) {
		t.Fatalf("request did not contain scoped repository: %s", request)
	}
	if strings.Contains(string(request), `"web"`) {
		t.Fatalf("request contained sibling repository: %s", request)
	}
}
