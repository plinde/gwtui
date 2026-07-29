package github

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/plinde/gwtui/internal/cache"
)

// stubGH puts a fake `gh` first on PATH that records each invocation and prints
// a fixed PR list, so call counts can be asserted without touching the network.
func stubGH(t *testing.T, body string) (callLog string) {
	t.Helper()
	dir := t.TempDir()
	callLog = filepath.Join(dir, "calls.txt")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"" + callLog + "\"\ncat <<'JSON'\n" + body + "\nJSON\n"
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return callLog
}

func callCount(t *testing.T, logPath string) int {
	t.Helper()
	b, err := os.ReadFile(logPath)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return len(strings.Fields(strings.ReplaceAll(strings.TrimSpace(string(b)), " ", "")))
}

func lines(t *testing.T, logPath string) []string {
	t.Helper()
	b, err := os.ReadFile(logPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

const onePR = `[{"number":1,"title":"T","state":"OPEN","isDraft":false,"headRefName":"feat/x"}]`

// The whole point: a second call inside the TTL must not reach the API. At a
// 15s refresh this is the difference between 4 live fetches an hour and 240.
func TestCachedFetchDoesNotRepeatTheAPICall(t *testing.T) {
	log := stubGH(t, onePR)
	c := cache.Options{Dir: t.TempDir(), TTL: 15 * time.Minute}

	for i := 0; i < 10; i++ {
		prs, err := PRsByBranchCached("/repo/a", ".", c)
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if prs["feat/x"] == nil {
			t.Fatalf("call %d: expected the PR to be returned from cache", i)
		}
	}
	if n := len(lines(t, log)); n != 1 {
		t.Errorf("gh was invoked %d times across 10 cached reads, want 1", n)
	}
}

// Distinct repos must not collide. Keying by bare name would make two org roots
// that each contain an "api" share one entry.
func TestCacheKeysAreScopedPerRepo(t *testing.T) {
	log := stubGH(t, onePR)
	c := cache.Options{Dir: t.TempDir(), TTL: 15 * time.Minute}

	for _, repo := range []string{"/orgs/one/api", "/orgs/two/api"} {
		if _, err := PRsByBranchCached(repo, ".", c); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(lines(t, log)); n != 2 {
		t.Errorf("gh invoked %d times for 2 distinct repos, want 2 (keys collided)", n)
	}
}

// An expired entry refetches.
func TestExpiredEntryRefetches(t *testing.T) {
	log := stubGH(t, onePR)
	c := cache.Options{Dir: t.TempDir(), TTL: time.Nanosecond}

	for i := 0; i < 3; i++ {
		if _, err := PRsByBranchCached("/repo/a", ".", c); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Nanosecond)
	}
	if n := len(lines(t, log)); n != 3 {
		t.Errorf("gh invoked %d times with an expired TTL, want 3", n)
	}
}

// Force is what the manual `r` refresh uses: bypass the read, still write
// through so the next auto-refresh is served from the fresh value.
func TestForceBypassesReadButWritesThrough(t *testing.T) {
	log := stubGH(t, onePR)
	dir := t.TempDir()

	forced := cache.Options{Dir: dir, TTL: 15 * time.Minute, Force: true}
	if _, err := PRsByBranchCached("/repo/a", ".", forced); err != nil {
		t.Fatal(err)
	}
	normal := cache.Options{Dir: dir, TTL: 15 * time.Minute}
	if _, err := PRsByBranchCached("/repo/a", ".", normal); err != nil {
		t.Fatal(err)
	}
	if n := len(lines(t, log)); n != 1 {
		t.Errorf("gh invoked %d times, want 1 — a forced call must still populate the cache", n)
	}
}

// Hitting the rate limit must degrade to stale data, not blank the column.
func TestRateLimitedCallServesStale(t *testing.T) {
	dir := t.TempDir()
	c := cache.Options{Dir: dir, TTL: time.Nanosecond}

	stubGH(t, onePR)
	if _, err := PRsByBranchCached("/repo/a", ".", c); err != nil {
		t.Fatal(err)
	}

	// Now make gh fail the way a 429 does.
	failDir := t.TempDir()
	script := "#!/bin/sh\necho 'API rate limit exceeded' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(failDir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", failDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	time.Sleep(2 * time.Nanosecond)
	prs, err := PRsByBranchCached("/repo/a", ".", c)
	if err != nil {
		t.Fatalf("a rate-limited call should serve stale data, got error: %v", err)
	}
	if prs["feat/x"] == nil {
		t.Error("expected the last-known PR to survive a rate-limit failure")
	}
}

// With no cached value at all there is nothing to serve, so the error surfaces.
func TestFailureWithoutCacheSurfaces(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\necho boom >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if _, err := PRsByBranchCached("/repo/a", ".", cache.Options{Dir: t.TempDir(), TTL: time.Minute}); err == nil {
		t.Error("expected an error when the call fails and nothing is cached")
	}
}

// Disabled means no read, no write, no stale fallback.
func TestDisabledCacheAlwaysCallsLive(t *testing.T) {
	log := stubGH(t, onePR)
	c := cache.Options{Dir: t.TempDir(), Disabled: true}

	for i := 0; i < 3; i++ {
		if _, err := PRsByBranchCached("/repo/a", ".", c); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(lines(t, log)); n != 3 {
		t.Errorf("gh invoked %d times with the cache disabled, want 3", n)
	}
}

// 200 forced two GraphQL pages per repo, since a page caps at 100 nodes. 100 is
// the largest value that still costs one request, so it is the right choice:
// trimming lower would narrow PR coverage without saving anything.
func TestFetchLimitFitsOneGraphQLPage(t *testing.T) {
	if PRFetchLimit > 100 {
		t.Errorf("PRFetchLimit = %d; above 100 the gh CLI paginates and doubles the rate-limit cost", PRFetchLimit)
	}
	log := stubGH(t, onePR)
	if _, err := PRsByBranch("."); err != nil {
		t.Fatal(err)
	}
	got := lines(t, log)
	if len(got) != 1 || !strings.Contains(got[0], "--limit 100") {
		t.Errorf("gh args = %v, want --limit 100", got)
	}
}
