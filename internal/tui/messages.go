package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/plinde/gwtui/internal/git"
	gh "github.com/plinde/gwtui/internal/github"
)

const autoRefreshInterval = 15 * time.Second

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

// autoRefreshTickMsg fires when the auto-refresh timer expires.
type autoRefreshTickMsg struct{}

// autoRefreshDoneMsg carries background-refresh results without disrupting the UI.
type autoRefreshDoneMsg struct {
	worktrees []git.Worktree
	prs       map[string]*gh.PR
	err       error
}

// scheduleAutoRefresh returns a command that fires autoRefreshTickMsg after the interval.
func scheduleAutoRefresh() tea.Cmd {
	return tea.Tick(autoRefreshInterval, func(time.Time) tea.Msg {
		return autoRefreshTickMsg{}
	})
}

// doAutoRefresh performs the same loading as doLoad but returns autoRefreshDoneMsg.
func doAutoRefresh(repoPath string) tea.Cmd {
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
			return autoRefreshDoneMsg{err: wt.err}
		}
		prs := pr.prs
		if prs == nil {
			prs = make(map[string]*gh.PR)
		}

		return autoRefreshDoneMsg{worktrees: wt.wts, prs: prs}
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
