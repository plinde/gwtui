package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestResolveTarget_SingleRepository(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "app")
	initGitRepoForDiscovery(t, repo)

	target, err := ResolveTarget(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if target.IsOrg {
		t.Fatal("expected single repository target, got org target")
	}
	if target.Root != repo {
		t.Errorf("expected root %q, got %q", repo, target.Root)
	}
	if len(target.Repositories) != 1 {
		t.Fatalf("expected 1 repository, got %d", len(target.Repositories))
	}
	if target.Repositories[0].Name != "app" {
		t.Errorf("expected repository name app, got %q", target.Repositories[0].Name)
	}
	if target.Repositories[0].Path != repo {
		t.Errorf("expected repository path %q, got %q", repo, target.Repositories[0].Path)
	}
}

func TestResolveTarget_OrgRootDiscoversDirectCheckouts(t *testing.T) {
	root := t.TempDir()
	initGitRepoForDiscovery(t, filepath.Join(root, "api"))
	initGitRepoForDiscovery(t, filepath.Join(root, "web"))
	if err := os.Mkdir(filepath.Join(root, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}

	target, err := ResolveTarget(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !target.IsOrg {
		t.Fatal("expected org target")
	}
	if len(target.Repositories) != 2 {
		t.Fatalf("expected 2 repositories, got %d", len(target.Repositories))
	}
	if target.Repositories[0].Name != "api" || target.Repositories[1].Name != "web" {
		t.Fatalf("expected sorted repositories api, web; got %#v", target.Repositories)
	}
}

func TestResolveTarget_OrgRootSkipsLinkedWorktrees(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "app")
	worktree := filepath.Join(root, "app--feature")
	initGitRepoForDiscovery(t, repo)
	gitCommand(t, repo, "worktree", "add", "-b", "feature", worktree)

	target, err := ResolveTarget(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(target.Repositories) != 1 {
		t.Fatalf("expected 1 repository, got %d: %#v", len(target.Repositories), target.Repositories)
	}
	if target.Repositories[0].Name != "app" {
		t.Errorf("expected only app checkout, got %q", target.Repositories[0].Name)
	}
}

func TestResolveTarget_OrgRootIgnoresNestedCheckouts(t *testing.T) {
	root := t.TempDir()
	initGitRepoForDiscovery(t, filepath.Join(root, "api"))
	initGitRepoForDiscovery(t, filepath.Join(root, "team", "service"))

	target, err := ResolveTarget(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(target.Repositories) != 1 {
		t.Fatalf("expected only direct checkouts, got %d: %#v", len(target.Repositories), target.Repositories)
	}
	if target.Repositories[0].Name != "api" {
		t.Errorf("expected only api checkout, got %q", target.Repositories[0].Name)
	}
}

func TestResolveTarget_NoRepositories(t *testing.T) {
	root := t.TempDir()

	_, err := ResolveTarget(root)
	if err == nil {
		t.Fatal("expected error")
	}
}

func initGitRepoForDiscovery(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, path, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(path, "README.md"), []byte("test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, path, "add", "README.md")
	gitCommand(t, path, "-c", "user.name=gwtui", "-c", "user.email=gwtui@example.com", "commit", "-m", "init")
}

func gitCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}
