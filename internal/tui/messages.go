package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/plinde/gwtui/internal/git"
	gh "github.com/plinde/gwtui/internal/github"
)

// loadDoneMsg is sent when worktree + PR data loading completes.
type loadDoneMsg struct {
	worktrees []git.Worktree
	prs       map[string]*gh.PR
	err       error
}

// cleanupDoneMsg is sent when all cleanup operations complete.
type cleanupDoneMsg struct {
	results []git.CleanupResult
}

// doLoad fetches worktrees and PR data concurrently.
func doLoad(repoPath string) tea.Cmd {
	return func() tea.Msg {
		type wtResult struct {
			wts []git.Worktree
			err error
		}
		type prResult struct {
			prs map[string]*gh.PR
			err error
		}

		wtCh := make(chan wtResult, 1)
		prCh := make(chan prResult, 1)

		go func() {
			wts, err := git.List(repoPath)
			wtCh <- wtResult{wts, err}
		}()
		go func() {
			prs, err := gh.PRsByBranch(repoPath)
			prCh <- prResult{prs, err}
		}()

		wt := <-wtCh
		pr := <-prCh

		if wt.err != nil {
			return loadDoneMsg{err: wt.err}
		}
		// PR errors are non-fatal — we just won't have PR info
		prs := pr.prs
		if prs == nil {
			prs = make(map[string]*gh.PR)
		}

		return loadDoneMsg{worktrees: wt.wts, prs: prs}
	}
}

// doCleanup executes worktree removals sequentially.
func doCleanup(repoPath string, rows []WorktreeRow) tea.Cmd {
	return func() tea.Msg {
		var selected []WorktreeRow
		for _, r := range rows {
			if r.Selected {
				selected = append(selected, r)
			}
		}

		var results []git.CleanupResult
		for _, r := range selected {
			result := git.RemoveWorktree(repoPath, r.Worktree)
			results = append(results, result)
		}
		return cleanupDoneMsg{results: results}
	}
}
