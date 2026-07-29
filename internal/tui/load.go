package tui

import (
	"fmt"
	"strings"
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
// When scopeRepo is set, sibling repositories are pruned before GitHub loading.
func LoadRows(targetPath, scopeRepo string) ([]WorktreeRow, []LoadWarning, error) {
	target, err := git.ResolveTarget(targetPath)
	if err != nil {
		return nil, nil, err
	}

	repositories := target.Repositories
	if scopeRepo = strings.TrimSpace(scopeRepo); scopeRepo != "" {
		repositories = nil
		for _, repo := range target.Repositories {
			if strings.EqualFold(repo.Name, scopeRepo) {
				repositories = append(repositories, repo)
				break
			}
		}
		if len(repositories) == 0 {
			return nil, nil, fmt.Errorf("repository %q is not a direct checkout under %s", scopeRepo, target.Root)
		}
	}

	type result struct {
		index int
		repo  git.Repository
		wts   []git.Worktree
		err   error
	}

	resultCh := make(chan result, len(repositories))
	var wg sync.WaitGroup

	for i, repo := range repositories {
		wg.Add(1)
		go func(index int, repo git.Repository) {
			defer wg.Done()
			wts, err := git.List(repo.Path)
			if err == nil {
				for i := range wts {
					wts[i].RepoPath = repo.Path
					if target.IsOrg {
						wts[i].RepoName = repo.Name
					}
				}
			}
			resultCh <- result{
				index: index,
				repo:  repo,
				wts:   wts,
				err:   err,
			}
		}(i, repo)
	}

	wg.Wait()
	close(resultCh)

	results := make([]result, len(repositories))
	for r := range resultCh {
		if r.err != nil {
			return nil, nil, fmt.Errorf("%s: failed to list worktrees: %w", r.repo.Name, r.err)
		}
		results[r.index] = r
	}

	queries := make([]gh.RepositoryBranches, 0, len(results))
	for _, result := range results {
		branches := make([]string, 0, len(result.wts))
		for _, wt := range result.wts {
			if !wt.IsMain && wt.Branch != "" {
				branches = append(branches, wt.Branch)
			}
		}
		queries = append(queries, gh.RepositoryBranches{
			Key:      result.repo.Path,
			Path:     result.repo.Path,
			Branches: branches,
		})
	}

	prsByRepo, queryErrors := gh.PRsByBranches(queries)
	var warnings []LoadWarning
	var rows []WorktreeRow
	for _, result := range results {
		if err := queryErrors[result.repo.Path]; err != nil {
			warnings = append(warnings, LoadWarning{Repo: result.repo, Err: err})
		}
		prs := prsByRepo[result.repo.Path]
		if prs == nil {
			prs = make(map[string]*gh.PR)
		}
		rows = append(rows, EnrichWorktrees(result.wts, prs)...)
	}
	return rows, warnings, nil
}
