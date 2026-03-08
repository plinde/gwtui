package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/plinde/gwtui/internal/git"
)

type phase int

const (
	phaseLoad phase = iota
	phaseList
	phaseConfirm
	phaseCleanup
	phaseDone
	phaseHelp
)

type model struct {
	phase    phase
	prevPhase phase // for returning from help
	repoPath string
	keys     keyMap
	spinner  spinner.Model

	rows      []WorktreeRow
	cursor    int
	maxBranch int
	maxStatus int

	results []git.CleanupResult
	loadErr error

	width  int
	height int
}

// Run launches the TUI.
func Run(repoPath string) error {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))

	m := model{
		phase:    phaseLoad,
		repoPath: repoPath,
		keys:     defaultKeyMap(),
		spinner:  s,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, doLoad(m.repoPath), scheduleAutoRefresh())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case loadDoneMsg:
		return m.handleLoadDone(msg)

	case autoRefreshTickMsg:
		return m.handleAutoRefreshTick()

	case autoRefreshDoneMsg:
		return m.handleAutoRefreshDone(msg)

	case cleanupDoneMsg:
		m.results = msg.results
		m.phase = phaseDone
		return m, nil
	}

	switch m.phase {
	case phaseList:
		return m.updateList(msg)
	case phaseConfirm:
		return m.updateConfirm(msg)
	case phaseDone:
		return m.updateDone(msg)
	case phaseHelp:
		return m.updateHelp(msg)
	}

	return m, nil
}

func (m model) handleLoadDone(msg loadDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.loadErr = msg.err
		m.phase = phaseDone
		return m, scheduleAutoRefresh()
	}
	m.rows = EnrichWorktrees(msg.worktrees, msg.prs)
	m.maxBranch, m.maxStatus = ColumnWidths(m.rows)
	m.phase = phaseList
	// Start cursor on the first cleanable row
	for i, r := range m.rows {
		if r.Cleanable {
			m.cursor = i
			break
		}
	}
	return m, scheduleAutoRefresh()
}

func (m model) handleAutoRefreshTick() (tea.Model, tea.Cmd) {
	if m.phase == phaseList {
		return m, doAutoRefresh(m.repoPath)
	}
	// Not in list phase — reschedule without loading
	return m, scheduleAutoRefresh()
}

func (m model) handleAutoRefreshDone(msg autoRefreshDoneMsg) (tea.Model, tea.Cmd) {
	// If we're no longer in list phase, discard and reschedule
	if m.phase != phaseList {
		return m, scheduleAutoRefresh()
	}

	// Silently ignore errors — don't disrupt the UI
	if msg.err != nil {
		return m, scheduleAutoRefresh()
	}

	// Preserve selected state by branch name
	oldSelected := make(map[string]bool)
	for _, r := range m.rows {
		if r.Selected {
			oldSelected[r.Worktree.Branch] = true
		}
	}

	// Build new rows
	newRows := EnrichWorktrees(msg.worktrees, msg.prs)

	// Restore selections
	for i := range newRows {
		if oldSelected[newRows[i].Worktree.Branch] {
			newRows[i].Selected = true
		}
	}

	m.rows = newRows
	m.maxBranch, m.maxStatus = ColumnWidths(m.rows)

	// Clamp cursor if list shrank
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}

	return m, scheduleAutoRefresh()
}

func (m model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
		case key.Matches(msg, m.keys.Toggle):
			if m.cursor < len(m.rows) && m.rows[m.cursor].Cleanable {
				m.rows[m.cursor].Selected = !m.rows[m.cursor].Selected
			}
		case key.Matches(msg, m.keys.All):
			for i := range m.rows {
				if m.rows[i].Cleanable {
					m.rows[i].Selected = true
				}
			}
		case key.Matches(msg, m.keys.None):
			for i := range m.rows {
				m.rows[i].Selected = false
			}
		case key.Matches(msg, m.keys.Confirm):
			if m.selectedCount() > 0 {
				m.phase = phaseConfirm
			}
		case key.Matches(msg, m.keys.Refresh):
			m.phase = phaseLoad
			return m, tea.Batch(m.spinner.Tick, doLoad(m.repoPath))
		case key.Matches(msg, m.keys.Help):
			m.prevPhase = phaseList
			m.phase = phaseHelp
		}
	}
	return m, nil
}

func (m model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(msg, m.keys.Enter):
			m.phase = phaseCleanup
			return m, tea.Batch(m.spinner.Tick, doCleanup(m.repoPath, m.rows))
		case key.Matches(msg, m.keys.Back):
			m.phase = phaseList
		}
	}
	return m, nil
}

func (m model) updateDone(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		if key.Matches(msg, m.keys.Enter) || key.Matches(msg, m.keys.Back) {
			m.results = nil
			m.loadErr = nil
			m.phase = phaseLoad
			return m, tea.Batch(m.spinner.Tick, doLoad(m.repoPath))
		}
	}
	return m, nil
}

func (m model) updateHelp(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		if key.Matches(msg, m.keys.Help) || key.Matches(msg, m.keys.Back) || key.Matches(msg, m.keys.Enter) {
			m.phase = m.prevPhase
		}
	}
	return m, nil
}

func (m model) View() string {
	switch m.phase {
	case phaseLoad:
		return m.viewLoad()
	case phaseList:
		return m.viewList()
	case phaseConfirm:
		return m.viewConfirm()
	case phaseCleanup:
		return m.viewCleanup()
	case phaseDone:
		return m.viewDone()
	case phaseHelp:
		return m.viewHelp()
	}
	return ""
}

func (m model) viewLoad() string {
	return "\n  " + m.spinner.View() + " Loading worktrees and PR status...\n"
}

func (m model) viewList() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString("  " + titleStyle.Render("gwtui") + " — Git Worktree Manager\n")
	b.WriteString("  " + pathStyle.Render(displayPath(m.repoPath)) + "\n")
	b.WriteString("\n")

	// Calculate visible area
	headerLines := 4
	footerLines := 3
	available := m.height - headerLines - footerLines
	if available < 1 {
		available = len(m.rows)
	}

	// Scrolling window centered on cursor
	start := 0
	if len(m.rows) > available {
		start = m.cursor - available/2
		if start < 0 {
			start = 0
		}
		if start+available > len(m.rows) {
			start = len(m.rows) - available
		}
	}
	end := start + available
	if end > len(m.rows) {
		end = len(m.rows)
	}

	for i := start; i < end; i++ {
		isCursor := i == m.cursor
		b.WriteString("  " + RenderRow(m.rows[i], isCursor, m.maxBranch, m.maxStatus) + "\n")
	}

	// Scroll indicators
	if start > 0 {
		b.WriteString(dimStyle.Render("  ↑ more above") + "\n")
	}
	if end < len(m.rows) {
		b.WriteString(dimStyle.Render("  ↓ more below") + "\n")
	}

	b.WriteString("\n")
	b.WriteString("  " + m.viewFooter() + "\n")
	b.WriteString("  " + helpStyle.Render("[space] toggle  [a]ll  [n]one  [tab] cleanup  [r]efresh  [?] help  [q]uit") + "\n")

	return b.String()
}

func (m model) viewFooter() string {
	selected := m.selectedCount()
	cleanable := m.cleanableCount()
	total := len(m.rows)

	return statusBarStyle.Render(fmt.Sprintf(
		"%d selected / %d cleanable / %d total",
		selected, cleanable, total,
	))
}

func (m model) viewConfirm() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString("  " + warningStyle.Render(fmt.Sprintf("Will remove %d worktree(s):", m.selectedCount())) + "\n")
	b.WriteString("\n")

	for _, r := range m.rows {
		if !r.Selected {
			continue
		}
		b.WriteString("  " + errorStyle.Render("✗") + " " +
			pathStyle.Render(CompressPath(r.Worktree.Path)) +
			dimStyle.Render(fmt.Sprintf("  (branch: %s)", r.Worktree.Branch)) + "\n")
	}

	b.WriteString("\n")
	b.WriteString("  " + dimStyle.Render("This will: git worktree remove <path> && git branch -D <branch>") + "\n")
	b.WriteString("\n")
	b.WriteString("  " + helpStyle.Render("[enter] confirm  [backspace] go back  [q] quit") + "\n")

	return b.String()
}

func (m model) viewCleanup() string {
	return "\n  " + m.spinner.View() + " Removing worktrees...\n"
}

func (m model) viewDone() string {
	var b strings.Builder

	b.WriteString("\n")

	if m.loadErr != nil {
		b.WriteString("  " + errorStyle.Render("Error: "+m.loadErr.Error()) + "\n")
		b.WriteString("\n")
		b.WriteString("  " + helpStyle.Render("[q] quit") + "\n")
		return b.String()
	}

	if len(m.results) == 0 {
		b.WriteString("  " + dimStyle.Render("No worktrees were removed.") + "\n")
		b.WriteString("\n")
		b.WriteString("  " + helpStyle.Render("[q] quit") + "\n")
		return b.String()
	}

	successes := 0
	failures := 0
	for _, r := range m.results {
		if r.Success {
			successes++
		} else {
			failures++
		}
	}

	b.WriteString("  " + titleStyle.Render("Cleanup Complete") + "\n")
	b.WriteString("\n")

	for _, r := range m.results {
		branch := r.Worktree.Branch
		path := CompressPath(r.Worktree.Path)
		if r.Success {
			b.WriteString("  " + successStyle.Render("✓") + " " + branch + "  " + pathStyle.Render(path) + "\n")
		} else {
			b.WriteString("  " + errorStyle.Render("✗") + " " + branch + "  " + errorStyle.Render(r.Error) + "\n")
		}
	}

	b.WriteString("\n")
	summary := fmt.Sprintf("Removed %d worktree(s).", successes)
	if failures > 0 {
		summary += fmt.Sprintf(" %d error(s).", failures)
	}
	b.WriteString("  " + dimStyle.Render(summary) + "\n")
	b.WriteString("\n")
	b.WriteString("  " + helpStyle.Render("[enter] back to list  [q] quit") + "\n")

	return b.String()
}

func (m model) viewHelp() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString("  " + titleStyle.Render("gwtui") + " — Help\n")
	b.WriteString("\n")

	b.WriteString("  " + helpSectionStyle.Render("Navigation") + "\n")
	b.WriteString("  " + helpKeyStyle.Render("↑/k") + "         " + helpDescStyle.Render("Move cursor up") + "\n")
	b.WriteString("  " + helpKeyStyle.Render("↓/j") + "         " + helpDescStyle.Render("Move cursor down") + "\n")
	b.WriteString("\n")

	b.WriteString("  " + helpSectionStyle.Render("Selection") + "\n")
	b.WriteString("  " + helpKeyStyle.Render("space") + "       " + helpDescStyle.Render("Toggle selection (cleanable rows only)") + "\n")
	b.WriteString("  " + helpKeyStyle.Render("a") + "           " + helpDescStyle.Render("Select all cleanable worktrees") + "\n")
	b.WriteString("  " + helpKeyStyle.Render("n") + "           " + helpDescStyle.Render("Deselect all") + "\n")
	b.WriteString("\n")

	b.WriteString("  " + helpSectionStyle.Render("Actions") + "\n")
	b.WriteString("  " + helpKeyStyle.Render("tab") + "         " + helpDescStyle.Render("Proceed to cleanup confirmation") + "\n")
	b.WriteString("  " + helpKeyStyle.Render("enter") + "       " + helpDescStyle.Render("Confirm cleanup / back to list") + "\n")
	b.WriteString("  " + helpKeyStyle.Render("r") + "           " + helpDescStyle.Render("Refresh worktrees and PR status") + "\n")
	b.WriteString("  " + helpKeyStyle.Render("backspace") + "   " + helpDescStyle.Render("Go back") + "\n")
	b.WriteString("\n")

	b.WriteString("  " + helpSectionStyle.Render("States") + "\n")
	b.WriteString("  " + stateMergedStyle.Render("merged") + "      " + helpDescStyle.Render("PR merged — safe to clean") + "\n")
	b.WriteString("  " + stateClosedStyle.Render("closed") + "      " + helpDescStyle.Render("PR closed — safe to clean") + "\n")
	b.WriteString("  " + stateNoPRStyle.Render("no PR") + "       " + helpDescStyle.Render("No associated PR — review before cleaning") + "\n")
	b.WriteString("  " + stateOpenStyle.Render("open") + "        " + helpDescStyle.Render("PR open — protected, cannot select") + "\n")
	b.WriteString("  " + stateDraftStyle.Render("draft") + "       " + helpDescStyle.Render("PR draft — protected, cannot select") + "\n")
	b.WriteString("  " + stateMainStyle.Render("main") + "        " + helpDescStyle.Render("Main worktree — always protected") + "\n")
	b.WriteString("\n")

	b.WriteString("  " + helpStyle.Render("[?] close help  [q] quit") + "\n")

	return b.String()
}

func (m model) selectedCount() int {
	n := 0
	for _, r := range m.rows {
		if r.Selected {
			n++
		}
	}
	return n
}

func (m model) cleanableCount() int {
	n := 0
	for _, r := range m.rows {
		if r.Cleanable {
			n++
		}
	}
	return n
}

// displayPath returns a path with $HOME replaced by ~ for display.
func displayPath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}
