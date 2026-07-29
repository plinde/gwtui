package tui

import (
	"fmt"
	"sync"

	"github.com/plinde/gwtui/internal/cache"
	"github.com/plinde/gwtui/internal/git"
	gh "github.com/plinde/gwtui/internal/github"
)

// repoFetchConcurrency bounds how many repositories are queried at once.
//
// This was unbounded: one goroutine per repository, every one firing a `gh pr
// list` simultaneously. That is how the account's GraphQL budget was observed
// at 5004/5000 — you only exceed a hard cap when requests are already in flight
// together when it is reached. Bounding the fan-out also keeps the machine from
// spawning seventeen `gh` processes at once.
const repoFetchConcurrency = 6

// LoadWarning describes non-fatal data that could not be loaded for a repo.
type LoadWarning struct {
	Repo git.Repository
	Err  error
}

// LoadRows loads worktree and PR data for either a single repo or an org root,
// using the default cache policy.
func LoadRows(targetPath string) ([]WorktreeRow, []LoadWarning, error) {
	return LoadRowsCached(targetPath, cache.Options{TTL: cache.DefaultTTL})
}

// LoadRowsCached is LoadRows with an explicit cache policy. Remote PR data is
// read through the cache; local worktree and git state are always recomputed,
// so the view stays live without spending API budget.
func LoadRowsCached(targetPath string, c cache.Options) ([]WorktreeRow, []LoadWarning, error) {
	target, err := git.ResolveTarget(targetPath)
	if err != nil {
		return nil, nil, err
	}

	type result struct {
		index    int
		rows     []WorktreeRow
		warnings []LoadWarning
		err      error
	}

	resultCh := make(chan result, len(target.Repositories))
	sem := make(chan struct{}, repoFetchConcurrency)
	var wg sync.WaitGroup

	for i, repo := range target.Repositories {
		wg.Add(1)
		go func(index int, repo git.Repository) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			rows, warnings, err := loadRepoRows(repo, target.IsOrg, c)
			resultCh <- result{
				index:    index,
				rows:     rows,
				warnings: warnings,
				err:      err,
			}
		}(i, repo)
	}

	wg.Wait()
	close(resultCh)

	rowsByIndex := make([][]WorktreeRow, len(target.Repositories))
	var warnings []LoadWarning
	for r := range resultCh {
		if r.err != nil {
			return nil, warnings, r.err
		}
		rowsByIndex[r.index] = r.rows
		warnings = append(warnings, r.warnings...)
	}

	var rows []WorktreeRow
	for _, repoRows := range rowsByIndex {
		rows = append(rows, repoRows...)
	}
	return rows, warnings, nil
}

func loadRepoRows(repo git.Repository, showRepoName bool, c cache.Options) ([]WorktreeRow, []LoadWarning, error) {
	wts, err := git.List(repo.Path)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: failed to list worktrees: %w", repo.Name, err)
	}

	for i := range wts {
		wts[i].RepoPath = repo.Path
		if showRepoName {
			wts[i].RepoName = repo.Name
		}
	}

	prs, err := gh.PRsByBranchCached(repo.Path, repo.Path, c)
	var warnings []LoadWarning
	if err != nil {
		warnings = append(warnings, LoadWarning{Repo: repo, Err: err})
		prs = make(map[string]*gh.PR)
	}
	if prs == nil {
		prs = make(map[string]*gh.PR)
	}

	return EnrichWorktrees(wts, prs), warnings, nil
}
