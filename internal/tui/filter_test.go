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
		phase:     phaseList,
		repoPath:  "/repo",
		keys:      defaultKeyMap(),
		rows:      rows,
		allRows:   allRows,
		cursor:    0,
		maxBranch: 7,
		maxStatus: 6,
		width:     80,
		height:    24,
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

func TestFilter_TypingFiltersRows(t *testing.T) {
	m := newFilterableModel()
	m.filtering = true

	// Type "alpha"
	for _, r := range "alpha" {
		updated, _ := m.Update(runeKey(r))
		m = updated.(model)
	}

	if m.filterText != "alpha" {
		t.Errorf("expected filterText='alpha', got %q", m.filterText)
	}
	if len(m.rows) != 1 {
		t.Errorf("expected 1 filtered row, got %d", len(m.rows))
	}
	if m.rows[0].Worktree.Branch != "alpha" {
		t.Errorf("expected alpha row, got %s", m.rows[0].Worktree.Branch)
	}
}

func TestFilter_BackspaceRemovesChar(t *testing.T) {
	m := newFilterableModel()
	m.filtering = true
	m.filterText = "alph"
	m = m.applyFilter()

	updated, _ := m.Update(specialKey(tea.KeyBackspace))
	um := updated.(model)

	if um.filterText != "alp" {
		t.Errorf("expected filterText='alp', got %q", um.filterText)
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
	m.filtering = true
	m.filterText = "alpha"
	m = m.applyFilter()

	updated, _ := m.Update(specialKey(tea.KeyTab))
	um := updated.(model)

	if um.filtering {
		t.Error("expected filtering=false after Tab")
	}
	if !um.filterLocked {
		t.Error("expected filterLocked=true after Tab")
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
}

func TestFilter_QuitBlockedDuringFilter(t *testing.T) {
	m := newFilterableModel()
	m.filtering = true

	// 'q' should be treated as text input, not quit
	updated, cmd := m.Update(runeKey('q'))
	um := updated.(model)

	if um.filterText != "q" {
		t.Errorf("expected 'q' added to filter, got %q", um.filterText)
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
	m.filterText = "alpha"
	m = m.applyFilter()
	m.cursor = 0

	// Toggle selection on filtered row
	updated, _ := m.Update(specialKey(tea.KeySpace))
	um := updated.(model)

	if !um.rows[0].Selected {
		t.Error("expected filtered row to be selected after space")
	}
}
