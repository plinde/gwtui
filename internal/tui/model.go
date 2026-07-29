package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/plinde/gwtui/internal/git"
)

type phase int

const (
	phaseLoad phase = iota
	phaseList
	phaseConfirm
	phaseConfirmOpenPR
	phaseCleanup
	phaseDone
	phaseHelp
)

type model struct {
	phase       phase
	prevPhase   phase // for returning from help
	repoPath    string
	displayPath string
	scopeRepo   string
	isOrgRoot   bool   // true when launched from an org root
	showRepos   bool   // true when main checkouts should be visible in orgroot mode
	buildInfo   string // version, commit, build time for help screen
	keys        keyMap
	spinner     spinner.Model

	rows         []WorktreeRow
	allRows      []WorktreeRow // full set before filtering
	unsortedRows []WorktreeRow // original order for SortNone restore
	cursor       int
	maxBranch    int
	maxStatus    int
	sortCol      SortColumn
	sortDir      SortDirection

	// Filter state
	filtering    bool            // filter editor is active
	filterText   string          // applied filter query
	filterLocked bool            // filter is applied and editor is dismissed
	filterInput  textinput.Model // editable query buffer

	results       []git.CleanupResult
	loadErr       error
	doneCountdown int

	jumpPath string // set when user presses enter to jump to a worktree

	width  int
	height int

	refreshGeneration uint64
	refreshInFlight   bool
}

// Run launches the TUI. Returns the selected worktree path if the user
// pressed enter to jump, or empty string on normal quit.
// buildInfo is the formatted version/commit/build string for the help screen.
func Run(repoPath, scopePath, scopeRepo string, showRepos bool, buildInfo string) (string, error) {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))

	m := model{
		phase:       phaseLoad,
		repoPath:    repoPath,
		displayPath: scopePath,
		scopeRepo:   strings.TrimSpace(scopeRepo),
		showRepos:   showRepos,
		buildInfo:   buildInfo,
		keys:        defaultKeyMap(),
		spinner:     s,
		sortCol:     SortState,
		sortDir:     SortDesc,
		filterInput: newFilterInput(""),
	}

	// Render TUI on stderr so stdout stays clean for jump path output.
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithOutput(os.Stderr))
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}
	if fm, ok := finalModel.(model); ok && fm.jumpPath != "" {
		return fm.jumpPath, nil
	}
	return "", nil
}

func newFilterInput(value string) textinput.Model {
	input := textinput.New()
	input.Prompt = ""
	input.CharLimit = 256
	input.SetValue(value)
	input.CursorEnd()
	return input
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, doLoad(m.repoPath, m.scopeRepo))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Ctrl+C always quits, even during filter input
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		if !m.filtering && key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case loadDoneMsg:
		return m.handleLoadDone(msg)

	case autoRefreshTickMsg:
		return m.handleAutoRefreshTick(msg)

	case autoRefreshDoneMsg:
		return m.handleAutoRefreshDone(msg)

	case cleanupDoneMsg:
		m.results = msg.results
		m.phase = phaseDone
		if len(m.results) > 0 {
			m.doneCountdown = doneCountdownSeconds
			return m, scheduleDoneCountdown()
		}
		return m, nil

	case doneCountdownTickMsg:
		return m.handleDoneCountdownTick()
	}

	switch m.phase {
	case phaseList:
		return m.updateList(msg)
	case phaseConfirm:
		return m.updateConfirm(msg)
	case phaseConfirmOpenPR:
		return m.updateConfirmOpenPR(msg)
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
		return m.scheduleNextAutoRefresh()
	}
	m.isOrgRoot = msg.isOrgRoot
	m.unsortedRows = msg.rows
	if m.unsortedRows == nil {
		m.unsortedRows = EnrichWorktrees(msg.worktrees, msg.prs)
	}
	if m.sortCol != SortNone {
		m.allRows = sortRows(m.unsortedRows, m.sortCol, m.sortDir)
	} else {
		m.allRows = make([]WorktreeRow, len(m.unsortedRows))
		copy(m.allRows, m.unsortedRows)
	}
	m.rows = m.visibleRows()
	m.maxBranch, m.maxStatus = ColumnWidths(m.rows)
	m.phase = phaseList
	// Start cursor on the first cleanable row
	for i, r := range m.rows {
		if r.Cleanable {
			m.cursor = i
			break
		}
	}
	return m.scheduleNextAutoRefresh()
}

func (m model) handleAutoRefreshTick(msg autoRefreshTickMsg) (tea.Model, tea.Cmd) {
	if msg.generation != m.refreshGeneration {
		return m, nil
	}
	if m.phase == phaseList {
		if m.refreshInFlight {
			return m, nil
		}
		m.refreshInFlight = true
		return m, doAutoRefresh(m.repoPath, m.scopeRepo, msg.generation)
	}
	// Not in list phase — reschedule without loading
	return m.scheduleNextAutoRefresh()
}

func (m model) handleAutoRefreshDone(msg autoRefreshDoneMsg) (tea.Model, tea.Cmd) {
	if msg.generation != m.refreshGeneration {
		return m, nil
	}
	m.refreshInFlight = false

	// If we're no longer in list phase, discard and reschedule
	if m.phase != phaseList {
		return m.scheduleNextAutoRefresh()
	}

	// Silently ignore errors — don't disrupt the UI
	if msg.err != nil {
		return m.scheduleNextAutoRefresh()
	}

	m.isOrgRoot = msg.isOrgRoot

	// Preserve selected state by worktree path from both visible and hidden rows.
	// Branch names collide across repositories in org-wide mode.
	oldSelected := make(map[string]bool)
	for _, r := range m.allRows {
		if r.Selected {
			oldSelected[r.Worktree.Path] = true
		}
	}
	// Also capture any pending selections from the filtered view
	for _, r := range m.rows {
		if r.Selected {
			oldSelected[r.Worktree.Path] = true
		}
	}

	// Build new rows
	newRows := msg.rows
	if newRows == nil {
		newRows = EnrichWorktrees(msg.worktrees, msg.prs)
	}

	// Restore selections
	for i := range newRows {
		if oldSelected[newRows[i].Worktree.Path] {
			newRows[i].Selected = true
		}
	}

	// Preserve unsorted order for SortNone restore
	m.unsortedRows = make([]WorktreeRow, len(newRows))
	copy(m.unsortedRows, newRows)

	// Re-apply current sort
	if m.sortCol != SortNone {
		newRows = sortRows(newRows, m.sortCol, m.sortDir)
	}

	m.allRows = newRows
	// An applied filter remains active across refreshes. Repository scope is
	// always applied first and cannot be cleared by filter editing.
	m.rows = m.visibleRows()
	m.maxBranch, m.maxStatus = ColumnWidths(m.rows)

	// Clamp cursor if list shrank
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}

	return m.scheduleNextAutoRefresh()
}

func (m model) scheduleNextAutoRefresh() (tea.Model, tea.Cmd) {
	m.refreshGeneration++
	return m, scheduleAutoRefresh(m.refreshGeneration)
}

func (m model) invalidateAutoRefresh() model {
	m.refreshGeneration++
	m.refreshInFlight = false
	return m
}

func (m model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Delegate to filter input handler when filtering
	if m.filtering {
		return m.updateFilter(msg)
	}

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
		case key.Matches(msg, m.keys.Top):
			m.cursor = 0
		case key.Matches(msg, m.keys.Bottom):
			m.cursor = len(m.rows) - 1
			m.clampCursor()
		case key.Matches(msg, m.keys.PageUp):
			m.cursor -= m.pageSize()
			m.clampCursor()
		case key.Matches(msg, m.keys.PageDown):
			m.cursor += m.pageSize()
			m.clampCursor()
		case key.Matches(msg, m.keys.Toggle):
			if m.cursor < len(m.rows) && m.rows[m.cursor].Cleanable {
				m.rows[m.cursor].Selected = !m.rows[m.cursor].Selected
			}
		case key.Matches(msg, m.keys.ToggleAll):
			m = m.toggleMergedClosedVisible()
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
		case key.Matches(msg, m.keys.Enter):
			if m.cursor >= 0 && m.cursor < len(m.rows) {
				m.jumpPath = m.rows[m.cursor].Worktree.Path
				return m, tea.Quit
			}
		case key.Matches(msg, m.keys.Refresh):
			m = m.invalidateAutoRefresh()
			m.phase = phaseLoad
			return m, tea.Batch(m.spinner.Tick, doLoad(m.repoPath, m.scopeRepo))
		case key.Matches(msg, m.keys.SortNext):
			m = m.advanceSort(nextSortColumn)
		case key.Matches(msg, m.keys.SortPrev):
			m = m.advanceSort(prevSortColumn)
		case key.Matches(msg, m.keys.SortToggle):
			m = m.toggleSortDir()
		case key.Matches(msg, m.keys.Filter):
			cursorPath := m.cursorPath()
			m = m.syncSelectionsToAll()
			m.filtering = true
			m.filterLocked = false
			m.rows = m.scopedRows()
			m.filterInput.SetValue(m.filterText)
			m.filterInput.CursorEnd()
			m.restoreCursorByPath(cursorPath)
			return m, m.filterInput.Focus()
		case msg.Type == tea.KeyEscape:
			// Clear only a user filter; repository launch scope stays active.
			if m.filterLocked {
				cursorPath := m.cursorPath()
				m = m.syncSelectionsToAll()
				m.filterLocked = false
				m.filterText = ""
				m.filterInput.SetValue("")
				m.rows = m.scopedRows()
				m.maxBranch, m.maxStatus = ColumnWidths(m.rows)
				m.restoreCursorByPath(cursorPath)
			}
		case key.Matches(msg, m.keys.Help):
			m.prevPhase = phaseList
			m.phase = phaseHelp
		}
	}
	return m, nil
}

func (m model) updateFilter(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.Type {
		case tea.KeyEscape:
			// Cancel and clear the user filter within repository launch scope.
			cursorPath := m.cursorPath()
			m = m.syncSelectionsToAll()
			m.filtering = false
			m.filterLocked = false
			m.filterText = ""
			m.filterInput.SetValue("")
			m.filterInput.Blur()
			m.rows = m.scopedRows()
			m.maxBranch, m.maxStatus = ColumnWidths(m.rows)
			m.restoreCursorByPath(cursorPath)
			return m, nil

		case tea.KeyEnter, tea.KeyTab:
			// Apply: lock the trimmed editor value and return control to the list.
			m = m.syncSelectionsToAll()
			cursorPath := m.cursorPath()
			m.filterText = strings.TrimSpace(m.filterInput.Value())
			m.filtering = false
			m.filterLocked = m.filterText != ""
			m.filterInput.Blur()
			m = m.applyFilter()
			m.restoreCursorByPath(cursorPath)
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	return m, cmd
}

// applyFilter applies the user filter within immutable repository launch scope.
func (m model) applyFilter() model {
	m.rows = m.visibleRows()
	m.maxBranch, m.maxStatus = ColumnWidths(m.rows)
	m.clampCursor()
	return m
}

func (m model) scopedRows() []WorktreeRow {
	if m.scopeRepo == "" {
		return m.allRows
	}
	rows := make([]WorktreeRow, 0, len(m.allRows))
	for _, row := range m.allRows {
		if strings.EqualFold(row.Worktree.RepoName, m.scopeRepo) {
			rows = append(rows, row)
		}
	}
	return rows
}

func (m model) visibleRows() []WorktreeRow {
	rows := m.scopedRows()
	if m.filterLocked && m.filterText != "" {
		rows = filterRows(rows, m.filterText)
	}
	// In orgroot mode, suppress main repo checkouts by default in the
	// org-wide view. When scoped to a single repo (scopeRepo != "") the
	// main checkout remains visible.
	// Set show_repos = true in config.toml or pass --show-repos to reveal them.
	if m.isOrgRoot && !m.showRepos && m.scopeRepo == "" {
		filtered := make([]WorktreeRow, 0, len(rows))
		for _, r := range rows {
			if r.State != StateMain {
				filtered = append(filtered, r)
			}
		}
		rows = filtered
	}
	return rows
}

func (m *model) clampCursor() {
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m model) pageSize() int {
	const headerLines = 5
	const footerLines = 3
	if m.height <= 0 {
		if len(m.rows) > 0 {
			return len(m.rows)
		}
		return 1
	}
	available := m.height - headerLines - footerLines
	if available < 1 {
		return 1
	}
	return available
}

func (m model) cursorPath() string {
	if m.cursor >= 0 && m.cursor < len(m.rows) {
		return m.rows[m.cursor].Worktree.Path
	}
	return ""
}

func (m *model) restoreCursorByPath(path string) {
	if path != "" {
		for i := range m.rows {
			if m.rows[i].Worktree.Path == path {
				m.cursor = i
				return
			}
		}
	}
	m.clampCursor()
}

// syncSelectionsToAll propagates selection state from filtered rows back to allRows.
// Only updates allRows entries that are present in the current filtered view,
// leaving selections on hidden (filtered-out) rows untouched.
func (m model) syncSelectionsToAll() model {
	visible := make(map[string]bool)
	for _, r := range m.rows {
		visible[r.Worktree.Path] = r.Selected
	}
	for i := range m.allRows {
		if sel, ok := visible[m.allRows[i].Worktree.Path]; ok {
			m.allRows[i].Selected = sel
		}
	}
	return m
}

func (m model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(msg, m.keys.Enter):
			if len(m.selectedOpenPRRows()) > 0 {
				m.phase = phaseConfirmOpenPR
				return m, nil
			}
			m.phase = phaseCleanup
			return m, tea.Batch(m.spinner.Tick, doCleanup(m.repoPath, m.rows))
		case key.Matches(msg, m.keys.Back):
			m.phase = phaseList
		}
	}
	return m, nil
}

func (m model) updateConfirmOpenPR(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(msg, m.keys.Enter):
			m.phase = phaseCleanup
			return m, tea.Batch(m.spinner.Tick, doCleanup(m.repoPath, m.rows))
		case key.Matches(msg, m.keys.Back):
			m.phase = phaseConfirm
		}
	}
	return m, nil
}

func (m model) selectedOpenPRRows() []WorktreeRow {
	var rows []WorktreeRow
	for _, row := range m.rows {
		if row.Selected && row.PR != nil && strings.EqualFold(row.PR.State, "OPEN") {
			rows = append(rows, row)
		}
	}
	return rows
}

func (m model) updateDone(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		if key.Matches(msg, m.keys.Enter) || key.Matches(msg, m.keys.Back) {
			return m.returnToList()
		}
	}
	return m, nil
}

func (m model) handleDoneCountdownTick() (tea.Model, tea.Cmd) {
	if m.phase != phaseDone {
		return m, nil
	}
	m.doneCountdown--
	if m.doneCountdown <= 0 {
		return m.returnToList()
	}
	return m, scheduleDoneCountdown()
}

// returnToList resets done-screen state and transitions to loading.
func (m model) returnToList() (tea.Model, tea.Cmd) {
	m.results = nil
	m.loadErr = nil
	m.doneCountdown = 0
	m = m.invalidateAutoRefresh()
	m.phase = phaseLoad
	return m, tea.Batch(m.spinner.Tick, doLoad(m.repoPath, m.scopeRepo))
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
	case phaseConfirmOpenPR:
		return m.viewConfirmOpenPR()
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
	contextPath := m.displayPath
	if contextPath == "" {
		contextPath = m.repoPath
	}
	b.WriteString("  " + pathStyle.Render(displayPath(contextPath)))
	if m.scopeRepo != "" {
		b.WriteString("  " + filterActiveStyle.Render("scope: repo:"+m.scopeRepo))
	}
	b.WriteString("\n")
	b.WriteString("  " + renderHeader(m.sortCol, m.sortDir, m.maxBranch, m.maxStatus) + "\n")

	// Calculate visible area using the same value as PageUp/PageDown.
	available := m.pageSize()

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
		b.WriteString("  " + RenderRow(m.rows[i], isCursor, m.maxBranch, m.maxStatus, m.width-rowIndentWidth) + "\n")
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
	if m.filtering {
		b.WriteString("  " + filterPromptStyle.Render("/") + filterInputStyle.Render(m.filterInput.View()) +
			"  " + helpStyle.Render("[enter/tab] apply  [esc] clear  fields: repo:<name>") + "\n")
	} else if m.filterLocked {
		b.WriteString("  " + filterActiveStyle.Render(fmt.Sprintf("filter: %s", m.filterText)) +
			"  " + helpStyle.Render("[/] edit  [esc] clear") + "\n")
	} else {
		b.WriteString("  " + helpStyle.Render("[enter] jump  [space] toggle  [ctrl+a] merged/closed  [a]ll  [n]one  [tab] cleanup  [r]efresh  [</>] sort  [s] asc/desc  [/] filter  [?] help  [q]uit") + "\n")
	}

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

func (m model) viewConfirmOpenPR() string {
	var b strings.Builder
	openRows := m.selectedOpenPRRows()

	b.WriteString("\n")
	b.WriteString("  " + errorStyle.Render(fmt.Sprintf(
		"Open PR warning: %d selected worktree(s) still have an open PR:",
		len(openRows),
	)) + "\n")
	b.WriteString("\n")

	for _, row := range openRows {
		status := fmt.Sprintf("#%d open", row.PR.Number)
		if row.PR.IsDraft {
			status += " (draft)"
		}
		b.WriteString("  " + warningStyle.Render("!") + " " +
			branchStyle.Render(BranchLabel(row)) + "  " +
			stateOpenStyle.Render(status) + "  " +
			pathStyle.Render(CompressPath(row.Worktree.Path)) + "\n")
	}

	b.WriteString("\n")
	b.WriteString("  " + warningStyle.Render("Confirming will remove these worktrees and delete their local branches.") + "\n")
	b.WriteString("\n")
	b.WriteString("  " + helpStyle.Render("[enter] confirm open-PR cleanup  [backspace] previous confirmation  [q] quit") + "\n")

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
	if m.doneCountdown > 0 {
		b.WriteString("  " + helpStyle.Render(fmt.Sprintf("[enter] back to list (%ds)  [q] quit", m.doneCountdown)) + "\n")
	} else {
		b.WriteString("  " + helpStyle.Render("[enter] back to list  [q] quit") + "\n")
	}

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
	b.WriteString("  " + helpKeyStyle.Render("home/g") + "      " + helpDescStyle.Render("Jump to first row") + "\n")
	b.WriteString("  " + helpKeyStyle.Render("end/G") + "       " + helpDescStyle.Render("Jump to last row") + "\n")
	b.WriteString("  " + helpKeyStyle.Render("pgup/pgdn") + "   " + helpDescStyle.Render("Move one visible page") + "\n")
	b.WriteString("\n")

	b.WriteString("  " + helpSectionStyle.Render("Selection") + "\n")
	b.WriteString("  " + helpKeyStyle.Render("space") + "       " + helpDescStyle.Render("Toggle selection (cleanable rows only)") + "\n")
	b.WriteString("  " + helpKeyStyle.Render("ctrl+a") + "      " + helpDescStyle.Render("Toggle visible merged/closed worktrees") + "\n")
	b.WriteString("  " + helpKeyStyle.Render("a") + "           " + helpDescStyle.Render("Select all cleanable worktrees") + "\n")
	b.WriteString("  " + helpKeyStyle.Render("n") + "           " + helpDescStyle.Render("Deselect all") + "\n")
	b.WriteString("\n")

	b.WriteString("  " + helpSectionStyle.Render("Sorting") + "\n")
	b.WriteString("  " + helpKeyStyle.Render(">") + "           " + helpDescStyle.Render("Next sort column (branch → PR# → state → none)") + "\n")
	b.WriteString("  " + helpKeyStyle.Render("<") + "           " + helpDescStyle.Render("Previous sort column (state → PR# → branch → none)") + "\n")
	b.WriteString("  " + helpKeyStyle.Render("s") + "           " + helpDescStyle.Render("Toggle sort direction (asc/desc)") + "\n")
	b.WriteString("\n")

	b.WriteString("  " + helpSectionStyle.Render("Filter") + "\n")
	b.WriteString("  " + helpKeyStyle.Render("/") + "           " + helpDescStyle.Render("Open filter input") + "\n")
	b.WriteString("  " + helpKeyStyle.Render("enter/tab") + "   " + helpDescStyle.Render("Apply filter and return to list") + "\n")
	b.WriteString("  " + helpKeyStyle.Render("repo:<name>") + " " + helpDescStyle.Render("Match owning repository") + "\n")
	b.WriteString("  " + helpKeyStyle.Render("esc") + "         " + helpDescStyle.Render("Clear user filter — keep repository scope") + "\n")
	b.WriteString("\n")

	b.WriteString("  " + helpSectionStyle.Render("Actions") + "\n")
	b.WriteString("  " + helpKeyStyle.Render("enter") + "       " + helpDescStyle.Render("Jump to worktree directory (exit + cd)") + "\n")
	b.WriteString("  " + helpKeyStyle.Render("tab") + "         " + helpDescStyle.Render("Proceed to cleanup confirmation") + "\n")
	b.WriteString("  " + helpKeyStyle.Render("r") + "           " + helpDescStyle.Render("Refresh worktrees and PR status") + "\n")
	b.WriteString("  " + helpKeyStyle.Render("backspace") + "   " + helpDescStyle.Render("Go back") + "\n")
	b.WriteString("\n")

	b.WriteString("  " + helpSectionStyle.Render("States") + "\n")
	b.WriteString("  " + stateMergedStyle.Render("merged") + "      " + helpDescStyle.Render("PR merged — safe to clean") + "\n")
	b.WriteString("  " + stateClosedStyle.Render("closed") + "      " + helpDescStyle.Render("PR closed — safe to clean") + "\n")
	b.WriteString("  " + stateNoPRStyle.Render("no PR") + "       " + helpDescStyle.Render("No associated PR — protected, clean manually") + "\n")
	b.WriteString("  " + stateOpenStyle.Render("open") + "        " + helpDescStyle.Render("PR open — protected, cannot select") + "\n")
	b.WriteString("  " + stateDraftStyle.Render("draft") + "       " + helpDescStyle.Render("PR draft — protected, cannot select") + "\n")
	b.WriteString("  " + stateMainStyle.Render("main") + "        " + helpDescStyle.Render("Main worktree — always protected") + "\n")
	b.WriteString("\n")

	b.WriteString("  " + helpStyle.Render("[?] close help  [q] quit") + "\n")
	if m.buildInfo != "" {
		b.WriteString("\n")
		b.WriteString("  " + dimStyle.Render(m.buildInfo) + "\n")
	}

	return b.String()
}

func (m model) toggleSortDir() model {
	if m.sortCol == SortNone {
		return m
	}
	m = m.syncSelectionsToAll()

	var cursorPath string
	if m.cursor >= 0 && m.cursor < len(m.rows) {
		cursorPath = m.rows[m.cursor].Worktree.Path
	}

	if m.sortDir == SortAsc {
		m.sortDir = SortDesc
	} else {
		m.sortDir = SortAsc
	}
	m.allRows = sortRows(m.allRows, m.sortCol, m.sortDir)
	m.rows = m.visibleRows()

	if cursorPath != "" {
		for i, r := range m.rows {
			if r.Worktree.Path == cursorPath {
				m.cursor = i
				break
			}
		}
	}
	return m
}

func (m model) advanceSort(nextFn func(SortColumn) SortColumn) model {
	// Sync selections from visible rows to allRows before re-sorting
	m = m.syncSelectionsToAll()

	// Track cursor by path
	var cursorPath string
	if m.cursor >= 0 && m.cursor < len(m.rows) {
		cursorPath = m.rows[m.cursor].Worktree.Path
	}

	next := nextFn(m.sortCol)
	if next == m.sortCol {
		// Same column: toggle direction
		if m.sortDir == SortAsc {
			m.sortDir = SortDesc
		} else {
			m.sortDir = SortAsc
		}
	} else {
		m.sortCol = next
		m.sortDir = SortAsc
	}

	if m.sortCol != SortNone {
		m.allRows = sortRows(m.allRows, m.sortCol, m.sortDir)
	} else {
		// Restore original order, preserving selection state
		selected := make(map[string]bool)
		for _, r := range m.allRows {
			if r.Selected {
				selected[r.Worktree.Path] = true
			}
		}
		m.allRows = make([]WorktreeRow, len(m.unsortedRows))
		copy(m.allRows, m.unsortedRows)
		for i := range m.allRows {
			m.allRows[i].Selected = selected[m.allRows[i].Worktree.Path]
		}
	}

	// Reapply repository scope and any active user filter.
	m.rows = m.visibleRows()

	// Restore cursor position
	if cursorPath != "" {
		for i, r := range m.rows {
			if r.Worktree.Path == cursorPath {
				m.cursor = i
				break
			}
		}
	}

	return m
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

// toggleMergedClosedVisible selects every visible merged/closed row unless
// they are all already selected, in which case it clears only that safe set.
// Selections in every other state remain untouched.
func (m model) toggleMergedClosedVisible() model {
	eligible := 0
	allSelected := true
	for _, row := range m.rows {
		if !bulkToggleEligible(row) {
			continue
		}
		eligible++
		if !row.Selected {
			allSelected = false
		}
	}
	if eligible == 0 {
		return m
	}
	for i := range m.rows {
		if bulkToggleEligible(m.rows[i]) {
			m.rows[i].Selected = !allSelected
		}
	}
	return m
}

func bulkToggleEligible(row WorktreeRow) bool {
	return row.State == StateMerged || row.State == StateClosed
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
