package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/plinde/gwtui/internal/cache"
)

// initRepo creates a minimal git repo with one commit, so ResolveTarget finds it.
func initRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e",
	)
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"commit", "-q", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
}

// The fan-out used to be unbounded — one goroutine per repository, every one
// firing `gh pr list` at the same instant. That is how the account's GraphQL
// budget was observed at 5004/5000: a hard cap is only exceeded when requests
// are already in flight together when it is reached.
//
// The stub records an "S" when it starts and an "E" when it finishes, so the
// running depth of the S/E sequence is the observed concurrency.
func TestRepoFanOutIsBounded(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	root := t.TempDir()
	const repos = 12
	for i := 0; i < repos; i++ {
		initRepo(t, filepath.Join(root, "repo"+string(rune('a'+i))))
	}

	binDir := t.TempDir()
	log := filepath.Join(binDir, "seq.txt")
	script := "#!/bin/sh\n" +
		"printf 'S' >> " + log + "\n" +
		"sleep 0.2\n" +
		"printf 'E' >> " + log + "\n" +
		"echo '[]'\n"
	if err := os.WriteFile(filepath.Join(binDir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _, _ = LoadRowsCached(root, cache.Options{Dir: t.TempDir(), Disabled: true})
	}()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("LoadRowsCached did not finish")
	}

	b, err := os.ReadFile(log)
	if err != nil {
		t.Skipf("stub gh was never invoked (%v) — nothing to measure", err)
	}
	seq := string(b)
	if strings.Count(seq, "S") < repos {
		t.Skipf("expected %d repos to be queried, saw %d", repos, strings.Count(seq, "S"))
	}

	depth, maxDepth := 0, 0
	for _, ch := range seq {
		if ch == 'S' {
			depth++
			if depth > maxDepth {
				maxDepth = depth
			}
		} else if ch == 'E' {
			depth--
		}
	}
	if maxDepth > repoFetchConcurrency {
		t.Errorf("observed %d concurrent gh calls across %d repos, bound is %d",
			maxDepth, repos, repoFetchConcurrency)
	}
	if maxDepth < 2 {
		t.Errorf("observed %d concurrent calls — the load went fully serial, which is a different bug", maxDepth)
	}
}

func TestRepoFetchConcurrencyIsSane(t *testing.T) {
	if repoFetchConcurrency < 1 {
		t.Fatalf("repoFetchConcurrency = %d, would deadlock", repoFetchConcurrency)
	}
	if repoFetchConcurrency > 10 {
		t.Errorf("repoFetchConcurrency = %d — high enough to overshoot the rate limit again", repoFetchConcurrency)
	}
}
