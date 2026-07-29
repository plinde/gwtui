package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/plinde/gwtui/internal/cache"
	"github.com/plinde/gwtui/internal/git"
	gh "github.com/plinde/gwtui/internal/github"
)

// autoRefreshInterval is the UI tick. It stays short because worktree and git
// state are local and free to recompute; remote PR data behind this tick is
// read through the TTL cache, so a fast tick no longer means fast API spend.
const autoRefreshInterval = 15 * time.Second

// loadDoneMsg is sent when worktree + PR data loading completes.
type loadDoneMsg struct {
	worktrees []git.Worktree
	prs       map[string]*gh.PR
	rows      []WorktreeRow
	err       error
}

// cleanupDoneMsg is sent when all cleanup operations complete.
type cleanupDoneMsg struct {
	results []git.CleanupResult
}

// doLoad fetches worktrees and PR data concurrently.
func doLoad(repoPath string, c cache.Options) tea.Cmd {
	return func() tea.Msg {
		rows, _, err := LoadRowsCached(repoPath, c)
		return loadDoneMsg{rows: rows, err: err}
	}
}

// autoRefreshTickMsg fires when the auto-refresh timer expires.
type autoRefreshTickMsg struct{}

// autoRefreshDoneMsg carries background-refresh results without disrupting the UI.
type autoRefreshDoneMsg struct {
	worktrees []git.Worktree
	prs       map[string]*gh.PR
	rows      []WorktreeRow
	err       error
}

// scheduleAutoRefresh returns a command that fires autoRefreshTickMsg after the interval.
func scheduleAutoRefresh() tea.Cmd {
	return tea.Tick(autoRefreshInterval, func(time.Time) tea.Msg {
		return autoRefreshTickMsg{}
	})
}

// doAutoRefresh performs the same loading as doLoad but returns autoRefreshDoneMsg.
func doAutoRefresh(repoPath string, c cache.Options) tea.Cmd {
	return func() tea.Msg {
		rows, _, err := LoadRowsCached(repoPath, c)
		return autoRefreshDoneMsg{rows: rows, err: err}
	}
}

const doneCountdownSeconds = 5

// doneCountdownTickMsg fires every second while on the done screen.
type doneCountdownTickMsg struct{}

// scheduleDoneCountdown ticks once per second for the done-screen countdown.
func scheduleDoneCountdown() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return doneCountdownTickMsg{}
	})
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
			ownerPath := r.Worktree.RepoPath
			if ownerPath == "" {
				ownerPath = repoPath
			}
			result := git.RemoveWorktree(ownerPath, r.Worktree)
			results = append(results, result)
		}
		return cleanupDoneMsg{results: results}
	}
}
