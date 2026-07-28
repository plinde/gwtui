package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInferOrgFromPath(t *testing.T) {
	org, root, err := inferOrgFromPath("/workspace/github.com/Acme/infrastructure/pkg")
	if err != nil {
		t.Fatal(err)
	}
	if org != "Acme" {
		t.Fatalf("org = %q, want Acme", org)
	}
	if root != "/workspace/github.com/Acme" {
		t.Fatalf("root = %q, want /workspace/github.com/Acme", root)
	}
}

func TestResolveLaunchFromOrgRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "github.com", "Acme")
	initLaunchRepo(t, filepath.Join(root, "infrastructure"))

	scope, err := resolveLaunch(root, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if scope.targetPath != root || scope.initialFilter != "" {
		t.Fatalf("scope = %#v, want root with no filter", scope)
	}
}

func TestResolveLaunchFromRepositoryInfersOrgAndFilter(t *testing.T) {
	root := filepath.Join(t.TempDir(), "github.com", "Acme")
	repo := filepath.Join(root, "infrastructure")
	initLaunchRepo(t, repo)

	scope, err := resolveLaunch(filepath.Join(repo, "internal"), "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if scope.targetPath != root {
		t.Fatalf("targetPath = %q, want %q", scope.targetPath, root)
	}
	if scope.initialFilter != "repo:infrastructure" {
		t.Fatalf("initialFilter = %q, want repo:infrastructure", scope.initialFilter)
	}
}

func TestResolveLaunchFromLinkedWorktreeUsesOwningRepository(t *testing.T) {
	root := filepath.Join(t.TempDir(), "github.com", "Acme")
	repo := filepath.Join(root, "infrastructure")
	worktree := filepath.Join(root, "infrastructure--feature")
	initLaunchRepo(t, repo)
	runLaunchGit(t, repo, "worktree", "add", "-b", "feature", worktree)

	scope, err := resolveLaunch(worktree, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if scope.targetPath != root {
		t.Fatalf("targetPath = %q, want %q", scope.targetPath, root)
	}
	if scope.initialFilter != "repo:infrastructure" {
		t.Fatalf("initialFilter = %q, want repo:infrastructure", scope.initialFilter)
	}
}

func TestResolveLaunchExplicitOrgAndRepo(t *testing.T) {
	root := filepath.Join(t.TempDir(), "github.com", "Acme")
	initLaunchRepo(t, filepath.Join(root, "infrastructure"))

	scope, err := resolveLaunch(root, "", "Acme", "infrastructure")
	if err != nil {
		t.Fatal(err)
	}
	if scope.targetPath != root || scope.initialFilter != "repo:infrastructure" {
		t.Fatalf("scope = %#v", scope)
	}
}

func TestResolveLaunchLegacyRepoPath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "github.com", "Acme")
	repo := filepath.Join(root, "infrastructure")
	initLaunchRepo(t, repo)

	scope, err := resolveLaunch(root, "", "", "infrastructure")
	if err != nil {
		t.Fatal(err)
	}
	if scope.targetPath != repo || scope.initialFilter != "" {
		t.Fatalf("scope = %#v, want legacy repo path", scope)
	}
}

func TestResolveLaunchRejectsAmbiguousInputs(t *testing.T) {
	_, err := resolveLaunch("/workspace", "repo", "Acme", "")
	if err == nil {
		t.Fatal("expected positional path plus --org to fail")
	}

	_, err = resolveLaunch("/workspace", "", "/workspace/org", "team/repo")
	if err == nil {
		t.Fatal("expected --repo path with --org to fail")
	}
}

func initLaunchRepo(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(path, "internal"), 0o755); err != nil {
		t.Fatal(err)
	}
	runLaunchGit(t, path, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(path, "README.md"), []byte("test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runLaunchGit(t, path, "add", "README.md")
	runLaunchGit(t, path, "-c", "user.name=gwtui", "-c", "user.email=gwtui@example.com", "commit", "-m", "init")
}

func runLaunchGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}
