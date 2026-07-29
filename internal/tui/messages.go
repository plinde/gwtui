package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/plinde/gwtui/internal/git"
	gh "github.com/plinde/gwtui/internal/github"
)

const autoRefreshInterval = time.Minute

// loadDoneMsg is sent when worktree + PR data loading completes.
type loadDoneMsg struct {
	worktrees []git.Worktree
	prs       map[string]*gh.PR
	rows      []WorktreeRow
	err       error
	isOrgRoot bool
}

// cleanupDoneMsg is sent when all cleanup operations complete.
type cleanupDoneMsg struct {
	results []git.CleanupResult
}

// doLoad fetches worktrees and PR data concurrently.
func doLoad(repoPath, scopeRepo string) tea.Cmd {
	return func() tea.Msg {
		rows, _, isOrgRoot, err := LoadRows(repoPath, scopeRepo)
		return loadDoneMsg{rows: rows, err: err, isOrgRoot: isOrgRoot}
	}
}

// autoRefreshTickMsg fires when the auto-refresh timer expires.
type autoRefreshTickMsg struct {
	generation uint64
}

// autoRefreshDoneMsg carries background-refresh results without disrupting the UI.
type autoRefreshDoneMsg struct {
	worktrees  []git.Worktree
	prs        map[string]*gh.PR
	rows       []WorktreeRow
	err        error
	generation uint64
	isOrgRoot  bool
}

// scheduleAutoRefresh returns a command that fires autoRefreshTickMsg after the interval.
func scheduleAutoRefresh(generation uint64) tea.Cmd {
	return tea.Tick(autoRefreshInterval, func(time.Time) tea.Msg {
		return autoRefreshTickMsg{generation: generation}
	})
}

// doAutoRefresh performs the same loading as doLoad but returns autoRefreshDoneMsg.
func doAutoRefresh(repoPath, scopeRepo string, generation uint64) tea.Cmd {
	return func() tea.Msg {
		rows, _, isOrgRoot, err := LoadRows(repoPath, scopeRepo)
		return autoRefreshDoneMsg{rows: rows, err: err, generation: generation, isOrgRoot: isOrgRoot}
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
