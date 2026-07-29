package github

import (
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"

	"github.com/plinde/gwtui/internal/cache"
)

// PRFetchLimit bounds how many PRs are requested per repository.
//
// This was 200. GitHub's GraphQL API caps a connection page at 100 nodes, so
// 200 forced the gh CLI into *two* requests per repository on every refresh —
// double the rate-limit spend on every tick.
//
// 100 rather than a smaller number on purpose: anything from 1 to 100 costs the
// same single request, so trimming below 100 buys nothing and only narrows
// coverage. That matters because this list is matched against live worktree
// branches, and a long-lived branch on a busy repo (infrastructure has ~141 PRs,
// some have more) can sit outside the most recent slice. Missing its PR would
// drop the state badge — including `MERGED`, which is what makes a worktree safe
// to clean up.
//
// Exported so the rate-budget test in package tui can reason about how many
// GraphQL requests a refresh actually costs.
const PRFetchLimit = 100

// PR represents a GitHub pull request.
type PR struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	State   string `json:"state"` // "OPEN", "CLOSED", "MERGED"
	IsDraft bool   `json:"isDraft"`
	HeadRef string `json:"headRefName"`
}

// PRsByBranch returns a map of branch name → PR for the repo at repoPath.
// Uses `gh pr list --state all --json ...`.
//
// This always hits the API. Prefer PRsByBranchCached on any path that runs on
// a timer.
func PRsByBranch(repoPath string) (map[string]*PR, error) {
	cmd := exec.Command("gh", "pr", "list",
		"--state", "all",
		"--limit", strconv.Itoa(PRFetchLimit),
		"--json", "number,title,state,isDraft,headRefName",
	)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var prs []PR
	if err := json.Unmarshal(out, &prs); err != nil {
		return nil, err
	}

	result := make(map[string]*PR, len(prs))
	for i := range prs {
		result[prs[i].HeadRef] = &prs[i]
	}
	return result, nil
}

// PRsByBranchCached is PRsByBranch behind a TTL cache.
//
// This is the difference between gwtui being a well-behaved API client and not
// being one. `gh pr list` is GraphQL, and GitHub's GraphQL budget is 5000/hour
// shared across every tool, machine and token on the account. Re-fetching every
// repository live on each 15-second tick cost roughly 11,000 calls/hour on its
// own — enough to exhaust the entire account budget in about twenty minutes,
// which is what it did.
//
// Reading through the cache turns 60 refreshes/hour into 4 live fetches/hour
// per repository. Local worktree and git state are unaffected: they are cheap,
// local, and still recomputed on every tick, so the view stays live.
//
// A failed live call — most importantly a 429 — serves the last-known value
// instead of surfacing an error, so hitting the limit degrades to slightly
// stale PR data rather than blanking the column.
func PRsByBranchCached(repoKey, repoPath string, c cache.Options) (map[string]*PR, error) {
	key := "prs-" + sanitizeKey(repoKey)

	var cached map[string]*PR
	if hit, _ := cache.Get(c, key, &cached); hit {
		return cached, nil
	}

	prs, err := PRsByBranch(repoPath)
	if err != nil {
		var stale map[string]*PR
		if hit, _ := cache.GetStale(c, key, &stale); hit {
			return stale, nil
		}
		return nil, err
	}

	_ = cache.Set(c, key, prs)
	return prs, nil
}

// sanitizeKey makes a repo identifier safe as part of a cache key. Repo keys
// are paths or owner/name pairs, so separators have to go.
func sanitizeKey(s string) string {
	return strings.NewReplacer("/", "_", "\\", "_", ":", "_", " ", "_").Replace(s)
}
