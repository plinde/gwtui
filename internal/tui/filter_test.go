package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/plinde/gwtui/internal/git"
	gh "github.com/plinde/gwtui/internal/github"
)

func TestFilterRows_EmptyText(t *testing.T) {
	rows := []WorktreeRow{
		makeRow("alpha", StateNoPR, nil),
		makeRow("bravo", StateMerged, nil),
	}
	result := filterRows(rows, "")
	if len(result) != len(rows) {
		t.Errorf("expected %d rows with empty filter, got %d", len(rows), len(result))
	}
}

func TestFilterRows_ByBranch(t *testing.T) {
	rows := []WorktreeRow{
		makeRow("alpha", StateNoPR, nil),
		makeRow("bravo", StateMerged, nil),
		makeRow("charlie", StateNoPR, nil),
	}
	result := filterRows(rows, "bra")
	if len(result) != 1 {
		t.Fatalf("expected 1 row matching 'bra', got %d", len(result))
	}
	if result[0].Worktree.Branch != "bravo" {
		t.Errorf("expected bravo, got %s", result[0].Worktree.Branch)
	}
}

func TestFilterRows_ByRepositoryName(t *testing.T) {
	rows := []WorktreeRow{
		{Worktree: git.Worktree{RepoName: "api", Branch: "main"}, State: StateMain},
		{Worktree: git.Worktree{RepoName: "web", Branch: "main"}, State: StateMain},
	}
	result := filterRows(rows, "web")
	if len(result) != 1 {
		t.Fatalf("expected 1 row matching repository name 'web', got %d", len(result))
	}
	if result[0].Worktree.RepoName != "web" {
		t.Errorf("expected web repo, got %s", result[0].Worktree.RepoName)
	}
}

func TestFilterRows_ByOrgWideBranchLabel(t *testing.T) {
	rows := []WorktreeRow{
		{Worktree: git.Worktree{RepoName: "api", Branch: "main"}, State: StateMain},
		{Worktree: git.Worktree{RepoName: "web", Branch: "feature/login"}, State: StateNoPR},
	}
	result := filterRows(rows, "web:feature")
	if len(result) != 1 {
		t.Fatalf("expected 1 row matching org-wide branch label, got %d", len(result))
	}
	if got := BranchLabel(result[0]); got != "web:feature/login" {
		t.Errorf("expected web:feature/login, got %s", got)
	}
}

func TestFilterRows_ByScopedRepository(t *testing.T) {
	rows := []WorktreeRow{
		{Worktree: git.Worktree{RepoName: "infrastructure", Branch: "main"}, State: StateMain},
		{Worktree: git.Worktree{RepoName: "web", Branch: "infrastructure-fix"}, State: StateMerged},
	}
	result := filterRows(rows, "repo:INFRA")
	if len(result) != 1 || result[0].Worktree.RepoName != "infrastructure" {
		t.Fatalf("repo filter returned %#v", result)
	}
}

func TestFilterRows_OperandsAreAnded(t *testing.T) {
	rows := []WorktreeRow{
		{Worktree: git.Worktree{RepoName: "infrastructure", Branch: "main"}, State: StateMain},
		{Worktree: git.Worktree{RepoName: "infrastructure", Branch: "cleanup"}, State: StateMerged},
		{Worktree: git.Worktree{RepoName: "web", Branch: "cleanup"}, State: StateMerged},
	}
	result := filterRows(rows, "repo:infra merged")
	if len(result) != 1 || result[0].Worktree.Branch != "cleanup" {
		t.Fatalf("AND filter returned %#v", result)
	}
}

func TestFilterRows_NegatedRepository(t *testing.T) {
	rows := []WorktreeRow{
		{Worktree: git.Worktree{RepoName: "infrastructure", Branch: "main"}, State: StateMain},
		{Worktree: git.Worktree{RepoName: "web", Branch: "main"}, State: StateMain},
	}
	result := filterRows(rows, "-repo:infra")
	if len(result) != 1 || result[0].Worktree.RepoName != "web" {
		t.Fatalf("negated repo filter returned %#v", result)
	}
}

func TestFilterRows_IncompleteRepositoryOperandIsIgnored(t *testing.T) {
	rows := []WorktreeRow{
		{Worktree: git.Worktree{RepoName: "api", Branch: "main"}, State: StateMain},
		{Worktree: git.Worktree{RepoName: "web", Branch: "main"}, State: StateMain},
	}
	if got := filterRows(rows, "repo:"); len(got) != len(rows) {
		t.Fatalf("incomplete repo operand returned %d rows, want %d", len(got), len(rows))
	}
}

func TestFilterRows_UnknownFieldRemainsBareTerm(t *testing.T) {
	rows := []WorktreeRow{
		{Worktree: git.Worktree{Branch: "topic:one"}, State: StateNoPR},
		{Worktree: git.Worktree{Branch: "other"}, State: StateNoPR},
	}
	result := filterRows(rows, "topic:one")
	if len(result) != 1 || result[0].Worktree.Branch != "topic:one" {
		t.Fatalf("unknown field compatibility returned %#v", result)
	}
}

func TestFilterRows_CaseInsensitive(t *testing.T) {
	rows := []WorktreeRow{
		makeRow("Alpha", StateNoPR, nil),
		makeRow("bravo", StateMerged, nil),
	}
	result := filterRows(rows, "ALPHA")
	if len(result) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result))
	}
	if result[0].Worktree.Branch != "Alpha" {
		t.Errorf("expected Alpha, got %s", result[0].Worktree.Branch)
	}
}

func TestFilterRows_ByState(t *testing.T) {
	rows := []WorktreeRow{
		makeRow("alpha", StateNoPR, nil),
		makeRow("bravo", StateMerged, nil),
	}
	result := filterRows(rows, "merged")
	if len(result) != 1 {
		t.Fatalf("expected 1 row matching 'merged', got %d", len(result))
	}
	if result[0].Worktree.Branch != "bravo" {
		t.Errorf("expected bravo, got %s", result[0].Worktree.Branch)
	}
}

func TestFilterRows_ByPath(t *testing.T) {
	rows := []WorktreeRow{
		{Worktree: git.Worktree{Path: "/workspace/foo", Branch: "foo"}, State: StateNoPR},
		{Worktree: git.Worktree{Path: "/workspace/bar", Branch: "bar"}, State: StateNoPR},
	}
	result := filterRows(rows, "foo")
	if len(result) != 1 {
		t.Fatalf("expected 1 row matching path 'foo', got %d", len(result))
	}
}

func TestFilterRows_NoMatch(t *testing.T) {
	rows := []WorktreeRow{
		makeRow("alpha", StateNoPR, nil),
		makeRow("bravo", StateMerged, nil),
	}
	result := filterRows(rows, "zzzzz")
	if len(result) != 0 {
		t.Errorf("expected 0 rows, got %d", len(result))
	}
}

func TestFilterRows_ByPRState(t *testing.T) {
	rows := []WorktreeRow{
		makeRow("alpha", StateActive, &gh.PR{Number: 1, State: "OPEN", HeadRef: "alpha"}),
		makeRow("bravo", StateMerged, &gh.PR{Number: 2, State: "MERGED", HeadRef: "bravo"}),
	}
	result := filterRows(rows, "open")
	if len(result) != 1 {
		t.Fatalf("expected 1 row matching PR state 'open', got %d", len(result))
	}
	if result[0].Worktree.Branch != "alpha" {
		t.Errorf("expected alpha, got %s", result[0].Worktree.Branch)
	}
}

// ---------- Filter integration with model ----------

func newFilterableModel() model {
	rows := []WorktreeRow{
		{Worktree: git.Worktree{Path: "/repo", Branch: "main", IsMain: true}, State: StateMain, Cleanable: false},
		{Worktree: git.Worktree{Path: "/repo--alpha", Branch: "alpha"}, State: StateNoPR, Cleanable: false},
		{Worktree: git.Worktree{Path: "/repo--bravo", Branch: "bravo"}, State: StateMerged, Cleanable: true},
		{Worktree: git.Worktree{Path: "/repo--charlie", Branch: "charlie"}, State: StateNoPR, Cleanable: false},
	}
	allRows := make([]WorktreeRow, len(rows))
	copy(allRows, rows)
	return model{
		phase:       phaseList,
		repoPath:    "/repo",
		keys:        defaultKeyMap(),
		rows:        rows,
		allRows:     allRows,
		cursor:      0,
		maxBranch:   7,
		maxStatus:   6,
		width:       80,
		height:      24,
		filterInput: newFilterInput(""),
	}
}

func TestFilter_SlashEntersFilterMode(t *testing.T) {
	m := newFilterableModel()

	updated, _ := m.Update(runeKey('/'))
	um := updated.(model)

	if !um.filtering {
		t.Error("expected filtering=true after '/'")
	}
	if um.filterLocked {
		t.Error("expected filterLocked=false after '/'")
	}
}

func TestFilter_TypingDoesNotApplyUntilAccepted(t *testing.T) {
	m := newFilterableModel()
	updated, _ := m.Update(runeKey('/'))
	m = updated.(model)

	// Type "alpha"
	for _, r := range "alpha" {
		updated, _ = m.Update(runeKey(r))
		m = updated.(model)
	}

	if m.filterInput.Value() != "alpha" {
		t.Errorf("expected editor value 'alpha', got %q", m.filterInput.Value())
	}
	if m.filterText != "" {
		t.Errorf("typing unexpectedly changed applied filter to %q", m.filterText)
	}
	if len(m.rows) != 4 {
		t.Errorf("expected full list while editing, got %d rows", len(m.rows))
	}
}

func TestFilter_BackspaceRemovesChar(t *testing.T) {
	m := newFilterableModel()
	m.filtering = true
	m.filterInput = newFilterInput("alph")
	m.filterInput.Focus()

	updated, _ := m.Update(specialKey(tea.KeyBackspace))
	um := updated.(model)

	if um.filterInput.Value() != "alp" {
		t.Errorf("expected editor value 'alp', got %q", um.filterInput.Value())
	}
}

func TestFilter_EscCancelsFilter(t *testing.T) {
	m := newFilterableModel()
	m.filtering = true
	m.filterText = "alpha"
	m = m.applyFilter()

	updated, _ := m.Update(specialKey(tea.KeyEscape))
	um := updated.(model)

	if um.filtering {
		t.Error("expected filtering=false after Esc")
	}
	if um.filterLocked {
		t.Error("expected filterLocked=false after Esc")
	}
	if um.filterText != "" {
		t.Errorf("expected empty filterText after Esc, got %q", um.filterText)
	}
	if len(um.rows) != 4 {
		t.Errorf("expected all 4 rows restored after Esc, got %d", len(um.rows))
	}
}

func TestFilter_TabLocksFilter(t *testing.T) {
	m := newFilterableModel()
	updated, _ := m.Update(runeKey('/'))
	m = updated.(model)
	for _, r := range "alpha" {
		updated, _ = m.Update(runeKey(r))
		m = updated.(model)
	}

	updated, _ = m.Update(specialKey(tea.KeyTab))
	um := updated.(model)

	if um.filtering {
		t.Error("expected filtering=false after Tab")
	}
	if !um.filterLocked {
		t.Error("expected filterLocked=true after Tab")
	}
	if um.filterText != "alpha" {
		t.Errorf("expected applied filter alpha, got %q", um.filterText)
	}
	if len(um.rows) != 1 {
		t.Errorf("expected 1 filtered row after Tab lock, got %d", len(um.rows))
	}
}

func TestFilter_EscClearsLockedFilter(t *testing.T) {
	m := newFilterableModel()
	m.filterLocked = true
	m.filterText = "alpha"
	m = m.applyFilter()

	// In normal list mode, Esc should clear locked filter
	updated, _ := m.Update(specialKey(tea.KeyEscape))
	um := updated.(model)

	if um.filterLocked {
		t.Error("expected filterLocked=false after Esc")
	}
	if um.filterText != "" {
		t.Errorf("expected empty filterText, got %q", um.filterText)
	}
	if len(um.rows) != 4 {
		t.Errorf("expected all 4 rows after clearing filter, got %d", len(um.rows))
	}
}

func TestFilter_SlashReentersFilterWhenLocked(t *testing.T) {
	m := newFilterableModel()
	m.filterLocked = true
	m.filterText = "alpha"
	m = m.applyFilter()

	updated, _ := m.Update(runeKey('/'))
	um := updated.(model)

	if !um.filtering {
		t.Error("expected filtering=true after re-pressing /")
	}
	if um.filterLocked {
		t.Error("expected filterLocked=false when re-entering filter")
	}
	// Filter text should be preserved for editing
	if um.filterText != "alpha" {
		t.Errorf("expected filterText='alpha' preserved, got %q", um.filterText)
	}
	if um.filterInput.Value() != "alpha" {
		t.Errorf("expected editor seeded with alpha, got %q", um.filterInput.Value())
	}
}

func TestFilter_QuitBlockedDuringFilter(t *testing.T) {
	m := newFilterableModel()
	updated, _ := m.Update(runeKey('/'))
	m = updated.(model)

	// 'q' should be treated as text input, not quit
	updated, cmd := m.Update(runeKey('q'))
	um := updated.(model)

	if um.filterInput.Value() != "q" {
		t.Errorf("expected 'q' added to editor, got %q", um.filterInput.Value())
	}
	if cmd != nil {
		// Verify it's not a quit command
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Error("'q' should not quit during filter mode")
		}
	}
}

func TestFilter_CursorClampedOnFilter(t *testing.T) {
	m := newFilterableModel()
	m.cursor = 3 // last row
	m.filtering = true

	// Type something that filters to 1 row
	m.filterText = "alpha"
	m = m.applyFilter()

	if m.cursor >= len(m.rows) {
		t.Errorf("cursor %d should be clamped to filtered rows (len=%d)", m.cursor, len(m.rows))
	}
}

func TestFilter_TabWithEmptyTextDoesNotLock(t *testing.T) {
	m := newFilterableModel()
	m.filtering = true
	m.filterText = ""

	updated, _ := m.Update(specialKey(tea.KeyTab))
	um := updated.(model)

	if um.filterLocked {
		t.Error("Tab with empty filter should not set filterLocked")
	}
}

func TestFilter_NavigationWorksOnFilteredResults(t *testing.T) {
	m := newFilterableModel()
	m.filterLocked = true
	m.filterText = "a" // matches alpha and charlie
	m = m.applyFilter()
	m.cursor = 0

	// Move down
	updated, _ := m.Update(specialKey(tea.KeyDown))
	um := updated.(model)

	if um.cursor != 1 {
		t.Errorf("expected cursor=1 after down, got %d", um.cursor)
	}
}

func TestFilter_SelectionWorksOnFilteredResults(t *testing.T) {
	m := newFilterableModel()
	m.filterLocked = true
	m.filterText = "bravo"
	m = m.applyFilter()
	m.cursor = 0

	// Toggle selection on filtered row (bravo is merged/cleanable)
	updated, _ := m.Update(specialKey(tea.KeySpace))
	um := updated.(model)

	if !um.rows[0].Selected {
		t.Error("expected filtered row to be selected after space")
	}
}

// ---------- Bug fix: selection preservation on filtered-out rows ----------

func TestFilter_SelectionsPreservedOnFilteredOutRows(t *testing.T) {
	m := newFilterableModel()

	// Select bravo (index 2) before filtering
	m.rows[2].Selected = true
	m.allRows[2].Selected = true

	// Enter filter mode and filter to only show alpha
	m.filtering = true
	m.filterText = "alpha"
	m = m.applyFilter()

	// Cancel filter with Esc
	updated, _ := m.Update(specialKey(tea.KeyEscape))
	um := updated.(model)

	// bravo's selection should be preserved even though it was hidden
	found := false
	for _, r := range um.rows {
		if r.Worktree.Branch == "bravo" && r.Selected {
			found = true
		}
	}
	if !found {
		t.Error("expected bravo selection preserved after filter cancel, but it was lost")
	}
}

func TestFilter_CtrlATogglesVisibleRowsAndPreservesHiddenSelection(t *testing.T) {
	m := newFilterableModel()
	m.allRows[1].State = StateMerged
	m.allRows[1].Cleanable = true
	m.allRows[1].Selected = true
	m.allRows[2].Selected = true
	m.rows = m.allRows
	m.filterText = "alpha"
	m.filterLocked = true
	m = m.applyFilter()

	updated, _ := m.Update(specialKey(tea.KeyCtrlA))
	m = updated.(model)
	if m.rows[0].Selected {
		t.Fatal("ctrl+a should clear the selected visible cleanable row")
	}

	updated, _ = m.Update(specialKey(tea.KeyEscape))
	got := updated.(model)
	selected := make(map[string]bool)
	for _, row := range got.rows {
		selected[row.Worktree.Branch] = row.Selected
	}
	if selected["alpha"] {
		t.Fatal("visible alpha selection was not cleared")
	}
	if !selected["bravo"] {
		t.Fatal("hidden bravo selection was not preserved")
	}
}

func TestFilter_SelectionsPreservedOnTabLock(t *testing.T) {
	m := newFilterableModel()

	// Select bravo before filtering
	m.rows[2].Selected = true
	m.allRows[2].Selected = true

	// Filter to only show alpha, then Tab to lock.
	updated, _ := m.Update(runeKey('/'))
	m = updated.(model)
	for _, r := range "alpha" {
		updated, _ = m.Update(runeKey(r))
		m = updated.(model)
	}

	updated, _ = m.Update(specialKey(tea.KeyTab))
	um := updated.(model)

	// Now clear filter with Esc — bravo should still be selected
	updated, _ = um.Update(specialKey(tea.KeyEscape))
	um = updated.(model)

	found := false
	for _, r := range um.rows {
		if r.Worktree.Branch == "bravo" && r.Selected {
			found = true
		}
	}
	if !found {
		t.Error("expected bravo selection preserved after Tab lock + Esc clear")
	}
}

// ---------- Bug fix: Ctrl+C during filter mode ----------

func TestFilter_CtrlCQuitsDuringFilter(t *testing.T) {
	m := newFilterableModel()
	m.filtering = true
	m.filterText = "test"

	_, cmd := m.Update(specialKey(tea.KeyCtrlC))
	if cmd == nil {
		t.Fatal("expected quit cmd on Ctrl+C during filter, got nil")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg on Ctrl+C during filter, got %T", msg)
	}
}

// ---------- Additional edge cases ----------

func TestFilter_BackspaceOnEmptyText(t *testing.T) {
	m := newFilterableModel()
	updated, _ := m.Update(runeKey('/'))
	m = updated.(model)

	updated, _ = m.Update(specialKey(tea.KeyBackspace))
	um := updated.(model)

	if um.filterInput.Value() != "" {
		t.Errorf("expected empty editor after backspace on empty, got %q", um.filterInput.Value())
	}
	// Should still show all rows
	if len(um.rows) != 4 {
		t.Errorf("expected 4 rows, got %d", len(um.rows))
	}
}

func TestFilter_EditorSupportsCursorInsertion(t *testing.T) {
	m := newFilterableModel()
	updated, _ := m.Update(runeKey('/'))
	m = updated.(model)
	for _, r := range "repo:infra" {
		updated, _ = m.Update(runeKey(r))
		m = updated.(model)
	}
	m.filterInput.SetCursor(0)

	updated, _ = m.Update(runeKey('-'))
	m = updated.(model)
	if m.filterInput.Value() != "-repo:infra" {
		t.Fatalf("cursor insertion produced %q, want -repo:infra", m.filterInput.Value())
	}
}

func TestFilter_AppliedQueryReturnsNormalListControls(t *testing.T) {
	m := newFilterableModel()
	updated, _ := m.Update(runeKey('/'))
	m = updated.(model)
	for _, r := range "a" {
		updated, _ = m.Update(runeKey(r))
		m = updated.(model)
	}
	updated, _ = m.Update(specialKey(tea.KeyEnter))
	m = updated.(model)

	updated, _ = m.Update(specialKey(tea.KeyDown))
	m = updated.(model)
	if m.cursor != 1 {
		t.Fatalf("Down after filter apply left cursor at %d, want 1", m.cursor)
	}
}

func TestFilter_EnterAppliesFilterAndReturnsToList(t *testing.T) {
	m := newFilterableModel()
	updated, _ := m.Update(runeKey('/'))
	m = updated.(model)
	for _, r := range "alpha" {
		updated, _ = m.Update(runeKey(r))
		m = updated.(model)
	}

	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	um := updated.(model)

	if um.filtering || !um.filterLocked {
		t.Fatalf("Enter did not apply filter: filtering=%v locked=%v", um.filtering, um.filterLocked)
	}
	if um.filterText != "alpha" || len(um.rows) != 1 || um.rows[0].Worktree.Branch != "alpha" {
		t.Fatalf("Enter applied wrong filter: text=%q rows=%#v", um.filterText, um.rows)
	}
	if um.jumpPath != "" || cmd != nil {
		t.Fatal("Enter-to-apply must not jump or quit")
	}
}

func TestFilter_FilterThenSort(t *testing.T) {
	m := newFilterableModel()
	m.filterLocked = true
	m.filterText = "a" // matches main, alpha, charlie (all contain 'a')
	m = m.applyFilter()
	initialCount := len(m.rows)

	// Sort by branch
	updated, _ := m.Update(runeKey('>'))
	um := updated.(model)

	// Filter should still be active after sort
	if len(um.rows) != initialCount {
		t.Errorf("expected %d filtered rows after sort, got %d", initialCount, len(um.rows))
	}
	if !um.filterLocked {
		t.Error("expected filterLocked to remain true after sort")
	}
}
