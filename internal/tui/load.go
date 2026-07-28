package tui

import (
	"fmt"
	"sync"

	"github.com/plinde/gwtui/internal/git"
	gh "github.com/plinde/gwtui/internal/github"
)

// LoadWarning describes non-fatal data that could not be loaded for a repo.
type LoadWarning struct {
	Repo git.Repository
	Err  error
}

// LoadRows loads worktree and PR data for either a single repo or an org root.
func LoadRows(targetPath string) ([]WorktreeRow, []LoadWarning, error) {
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
	var wg sync.WaitGroup

	for i, repo := range target.Repositories {
		wg.Add(1)
		go func(index int, repo git.Repository) {
			defer wg.Done()
			rows, warnings, err := loadRepoRows(repo, target.IsOrg)
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

func loadRepoRows(repo git.Repository, showRepoName bool) ([]WorktreeRow, []LoadWarning, error) {
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

	prs, err := gh.PRsByBranch(repo.Path)
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
