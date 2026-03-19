package tui

import (
	"errors"
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/plinde/gwtui/internal/git"
	gh "github.com/plinde/gwtui/internal/github"
)

// ---------- helpers ----------

// newTestModel returns a model in phaseList with a standard set of rows.
func newTestModel() model {
	rows := []WorktreeRow{
		{Worktree: git.Worktree{Path: "/repo", Branch: "main", IsMain: true}, State: StateMain, Cleanable: false},
		{Worktree: git.Worktree{Path: "/repo--a", Branch: "a"}, State: StateMerged, Cleanable: true},
		{Worktree: git.Worktree{Path: "/repo--b", Branch: "b"}, State: StateNoPR, Cleanable: true},
		{Worktree: git.Worktree{Path: "/repo--c", Branch: "c"}, State: StateActive, Cleanable: false},
	}
	rowsCopy := make([]WorktreeRow, len(rows))
	copy(rowsCopy, rows)
	return model{
		phase:     phaseList,
		repoPath:  "/repo",
		keys:      defaultKeyMap(),
		rows:      rows,
		allRows:   rowsCopy,
		cursor:    1, // first cleanable
		maxBranch: 4,
		maxStatus: 5,
		width:     80,
		height:    24,
	}
}

func runeKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func specialKey(k tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: k}
}

// ---------- displayPath ----------

func TestDisplayPath_WithHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}
	got := displayPath(home + "/projects/gwtui")
	want := "~/projects/gwtui"
	if got != want {
		t.Errorf("displayPath(%q) = %q, want %q", home+"/projects/gwtui", got, want)
	}
}

func TestDisplayPath_WithoutHome(t *testing.T) {
	got := displayPath("/tmp/something")
	if got != "/tmp/something" {
		t.Errorf("displayPath(%q) = %q, want unchanged", "/tmp/something", got)
	}
}

func TestDisplayPath_Empty(t *testing.T) {
	got := displayPath("")
	if got != "" {
		t.Errorf("displayPath(%q) = %q, want empty", "", got)
	}
}

// ---------- selectedCount ----------

func TestSelectedCount_Zero(t *testing.T) {
	m := newTestModel()
	if n := m.selectedCount(); n != 0 {
		t.Errorf("expected 0 selected, got %d", n)
	}
}

func TestSelectedCount_Some(t *testing.T) {
	m := newTestModel()
	m.rows[1].Selected = true
	if n := m.selectedCount(); n != 1 {
		t.Errorf("expected 1 selected, got %d", n)
	}
}

func TestSelectedCount_All(t *testing.T) {
	m := newTestModel()
	for i := range m.rows {
		m.rows[i].Selected = true
	}
	if n := m.selectedCount(); n != len(m.rows) {
		t.Errorf("expected %d selected, got %d", len(m.rows), n)
	}
}

// ---------- cleanableCount ----------

func TestCleanableCount_Mix(t *testing.T) {
	m := newTestModel()
	// rows: main(not cleanable), a(cleanable), b(cleanable), c(not cleanable)
	if n := m.cleanableCount(); n != 2 {
		t.Errorf("expected 2 cleanable, got %d", n)
	}
}

func TestCleanableCount_None(t *testing.T) {
	m := model{
		rows: []WorktreeRow{
			{Cleanable: false},
			{Cleanable: false},
		},
	}
	if n := m.cleanableCount(); n != 0 {
		t.Errorf("expected 0 cleanable, got %d", n)
	}
}

func TestCleanableCount_AllCleanable(t *testing.T) {
	m := model{
		rows: []WorktreeRow{
			{Cleanable: true},
			{Cleanable: true},
			{Cleanable: true},
		},
	}
	if n := m.cleanableCount(); n != 3 {
		t.Errorf("expected 3 cleanable, got %d", n)
	}
}

// ---------- loadDoneMsg handling ----------

func TestLoadDone_Success(t *testing.T) {
	m := model{
		phase:    phaseLoad,
		repoPath: "/repo",
		keys:     defaultKeyMap(),
	}

	wts := []git.Worktree{
		{Path: "/repo", Branch: "main", IsMain: true},
		{Path: "/repo--feat", Branch: "feat"},
	}
	prs := map[string]*gh.PR{
		"feat": {Number: 1, State: "MERGED", HeadRef: "feat"},
	}

	msg := loadDoneMsg{worktrees: wts, prs: prs}
	updated, cmd := m.Update(msg)
	um := updated.(model)

	if um.phase != phaseList {
		t.Errorf("expected phaseList, got %d", um.phase)
	}
	if len(um.rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(um.rows))
	}
	if um.loadErr != nil {
		t.Errorf("expected no error, got %v", um.loadErr)
	}
	// cmd is non-nil because handleLoadDone schedules auto-refresh
	if cmd == nil {
		t.Error("expected non-nil cmd (auto-refresh scheduled)")
	}
	// Cursor should be on the first cleanable row (index 1, the merged one)
	if um.cursor != 1 {
		t.Errorf("expected cursor=1 (first cleanable), got %d", um.cursor)
	}
}

func TestLoadDone_Error(t *testing.T) {
	m := model{
		phase:    phaseLoad,
		repoPath: "/repo",
		keys:     defaultKeyMap(),
	}

	msg := loadDoneMsg{err: errors.New("git not found")}
	updated, _ := m.Update(msg)
	um := updated.(model)

	if um.phase != phaseDone {
		t.Errorf("expected phaseDone on error, got %d", um.phase)
	}
	if um.loadErr == nil {
		t.Error("expected loadErr to be set")
	}
	if um.loadErr.Error() != "git not found" {
		t.Errorf("expected error 'git not found', got %q", um.loadErr.Error())
	}
}

func TestLoadDone_CursorOnFirstCleanable(t *testing.T) {
	m := model{
		phase:    phaseLoad,
		repoPath: "/repo",
		keys:     defaultKeyMap(),
	}

	// First two are non-cleanable (main, open PR), third is cleanable
	wts := []git.Worktree{
		{Path: "/repo", Branch: "main", IsMain: true},
		{Path: "/repo--open", Branch: "open"},
		{Path: "/repo--merged", Branch: "merged"},
	}
	prs := map[string]*gh.PR{
		"open":   {Number: 1, State: "OPEN", HeadRef: "open"},
		"merged": {Number: 2, State: "MERGED", HeadRef: "merged"},
	}

	msg := loadDoneMsg{worktrees: wts, prs: prs}
	updated, _ := m.Update(msg)
	um := updated.(model)

	if um.cursor != 2 {
		t.Errorf("expected cursor=2 (first cleanable), got %d", um.cursor)
	}
}

// ---------- List phase key events ----------

func TestList_SpaceTogglesSelection(t *testing.T) {
	m := newTestModel()
	m.cursor = 1 // cleanable row

	updated, _ := m.Update(specialKey(tea.KeySpace))
	um := updated.(model)
	if !um.rows[1].Selected {
		t.Error("expected row 1 to be selected after space")
	}

	// Toggle off
	updated, _ = um.Update(specialKey(tea.KeySpace))
	um = updated.(model)
	if um.rows[1].Selected {
		t.Error("expected row 1 to be deselected after second space")
	}
}

func TestList_SpaceOnNonCleanable(t *testing.T) {
	m := newTestModel()
	m.cursor = 0 // main worktree, not cleanable

	updated, _ := m.Update(specialKey(tea.KeySpace))
	um := updated.(model)
	if um.rows[0].Selected {
		t.Error("space should not select non-cleanable row")
	}
}

func TestList_UpDown(t *testing.T) {
	m := newTestModel()
	m.cursor = 1

	// Down
	updated, _ := m.Update(specialKey(tea.KeyDown))
	um := updated.(model)
	if um.cursor != 2 {
		t.Errorf("expected cursor=2 after down, got %d", um.cursor)
	}

	// Up
	updated, _ = um.Update(specialKey(tea.KeyUp))
	um = updated.(model)
	if um.cursor != 1 {
		t.Errorf("expected cursor=1 after up, got %d", um.cursor)
	}
}

func TestList_JK_VimKeys(t *testing.T) {
	m := newTestModel()
	m.cursor = 1

	// j = down
	updated, _ := m.Update(runeKey('j'))
	um := updated.(model)
	if um.cursor != 2 {
		t.Errorf("expected cursor=2 after 'j', got %d", um.cursor)
	}

	// k = up
	updated, _ = um.Update(runeKey('k'))
	um = updated.(model)
	if um.cursor != 1 {
		t.Errorf("expected cursor=1 after 'k', got %d", um.cursor)
	}
}

func TestList_TabToConfirm_WithSelections(t *testing.T) {
	m := newTestModel()
	m.rows[1].Selected = true

	updated, _ := m.Update(specialKey(tea.KeyTab))
	um := updated.(model)
	if um.phase != phaseConfirm {
		t.Errorf("expected phaseConfirm, got %d", um.phase)
	}
}

func TestList_TabToConfirm_NoSelections(t *testing.T) {
	m := newTestModel()
	// No rows selected

	updated, _ := m.Update(specialKey(tea.KeyTab))
	um := updated.(model)
	if um.phase != phaseList {
		t.Errorf("expected to stay in phaseList when nothing selected, got %d", um.phase)
	}
}

func TestList_HelpKey(t *testing.T) {
	m := newTestModel()

	updated, _ := m.Update(runeKey('?'))
	um := updated.(model)
	if um.phase != phaseHelp {
		t.Errorf("expected phaseHelp, got %d", um.phase)
	}
	if um.prevPhase != phaseList {
		t.Errorf("expected prevPhase=phaseList, got %d", um.prevPhase)
	}
}

func TestList_SelectAll(t *testing.T) {
	m := newTestModel()

	updated, _ := m.Update(runeKey('a'))
	um := updated.(model)

	for i, r := range um.rows {
		if r.Cleanable && !r.Selected {
			t.Errorf("row %d: expected selected (cleanable=true)", i)
		}
		if !r.Cleanable && r.Selected {
			t.Errorf("row %d: should not be selected (cleanable=false)", i)
		}
	}
}

func TestList_DeselectAll(t *testing.T) {
	m := newTestModel()
	m.rows[1].Selected = true
	m.rows[2].Selected = true

	updated, _ := m.Update(runeKey('n'))
	um := updated.(model)

	for i, r := range um.rows {
		if r.Selected {
			t.Errorf("row %d: expected deselected after 'n'", i)
		}
	}
}

func TestList_Quit(t *testing.T) {
	m := newTestModel()

	_, cmd := m.Update(runeKey('q'))
	if cmd == nil {
		t.Fatal("expected quit cmd, got nil")
	}
	// Execute the cmd and verify it returns a tea.QuitMsg
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestList_Refresh(t *testing.T) {
	m := newTestModel()

	updated, cmd := m.Update(runeKey('r'))
	um := updated.(model)

	if um.phase != phaseLoad {
		t.Errorf("expected phaseLoad after refresh, got %d", um.phase)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for refresh (batch of spinner + load)")
	}
}

// ---------- Enter-to-jump ----------

func TestList_EnterSetsJumpPathAndQuits(t *testing.T) {
	m := newTestModel()
	m.cursor = 2 // row "b" at /repo--b

	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	um := updated.(model)

	if um.jumpPath != "/repo--b" {
		t.Errorf("expected jumpPath=/repo--b, got %q", um.jumpPath)
	}
	if cmd == nil {
		t.Fatal("expected quit cmd, got nil")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestList_EnterOnMainRow(t *testing.T) {
	m := newTestModel()
	m.cursor = 0 // main row at /repo

	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	um := updated.(model)

	// Should still jump — enter works on any row
	if um.jumpPath != "/repo" {
		t.Errorf("expected jumpPath=/repo, got %q", um.jumpPath)
	}
	if cmd == nil {
		t.Fatal("expected quit cmd, got nil")
	}
}

func TestList_EnterWithEmptyRows(t *testing.T) {
	m := model{
		phase:  phaseList,
		keys:   defaultKeyMap(),
		rows:   []WorktreeRow{},
		cursor: 0,
	}

	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	um := updated.(model)

	if um.jumpPath != "" {
		t.Errorf("expected empty jumpPath with no rows, got %q", um.jumpPath)
	}
	if cmd != nil {
		t.Error("expected nil cmd with empty rows")
	}
}

// ---------- Confirm phase key events ----------

func TestConfirm_EnterStartsCleanup(t *testing.T) {
	m := newTestModel()
	m.phase = phaseConfirm
	m.rows[1].Selected = true

	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	um := updated.(model)

	if um.phase != phaseCleanup {
		t.Errorf("expected phaseCleanup, got %d", um.phase)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for cleanup batch")
	}
}

func TestConfirm_BackspaceReturnsToList(t *testing.T) {
	m := newTestModel()
	m.phase = phaseConfirm

	updated, _ := m.Update(specialKey(tea.KeyBackspace))
	um := updated.(model)

	if um.phase != phaseList {
		t.Errorf("expected phaseList, got %d", um.phase)
	}
}

func TestConfirm_Quit(t *testing.T) {
	m := newTestModel()
	m.phase = phaseConfirm

	_, cmd := m.Update(runeKey('q'))
	if cmd == nil {
		t.Fatal("expected quit cmd in confirm phase")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

// ---------- Done phase key events ----------

func TestDone_EnterReloads(t *testing.T) {
	m := newTestModel()
	m.phase = phaseDone
	m.results = []git.CleanupResult{{Worktree: git.Worktree{Branch: "a"}, Success: true}}

	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	um := updated.(model)

	if um.phase != phaseLoad {
		t.Errorf("expected phaseLoad after enter in done phase, got %d", um.phase)
	}
	if um.results != nil {
		t.Error("expected results to be cleared")
	}
	if um.loadErr != nil {
		t.Error("expected loadErr to be cleared")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for reload")
	}
}

func TestDone_BackspaceReloads(t *testing.T) {
	m := newTestModel()
	m.phase = phaseDone

	updated, cmd := m.Update(specialKey(tea.KeyBackspace))
	um := updated.(model)

	if um.phase != phaseLoad {
		t.Errorf("expected phaseLoad after backspace in done phase, got %d", um.phase)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for reload")
	}
}

// ---------- Help phase key events ----------

func TestHelp_QuestionMarkReturns(t *testing.T) {
	m := newTestModel()
	m.phase = phaseHelp
	m.prevPhase = phaseList

	updated, _ := m.Update(runeKey('?'))
	um := updated.(model)

	if um.phase != phaseList {
		t.Errorf("expected return to phaseList, got %d", um.phase)
	}
}

func TestHelp_BackspaceReturns(t *testing.T) {
	m := newTestModel()
	m.phase = phaseHelp
	m.prevPhase = phaseList

	updated, _ := m.Update(specialKey(tea.KeyBackspace))
	um := updated.(model)

	if um.phase != phaseList {
		t.Errorf("expected return to phaseList, got %d", um.phase)
	}
}

func TestHelp_EnterReturns(t *testing.T) {
	m := newTestModel()
	m.phase = phaseHelp
	m.prevPhase = phaseList

	updated, _ := m.Update(specialKey(tea.KeyEnter))
	um := updated.(model)

	if um.phase != phaseList {
		t.Errorf("expected return to phaseList, got %d", um.phase)
	}
}

// ---------- Cursor bounds ----------

func TestCursorBounds_UpAtZero(t *testing.T) {
	m := newTestModel()
	m.cursor = 0

	updated, _ := m.Update(specialKey(tea.KeyUp))
	um := updated.(model)

	if um.cursor != 0 {
		t.Errorf("cursor should stay at 0, got %d", um.cursor)
	}
}

func TestCursorBounds_DownAtMax(t *testing.T) {
	m := newTestModel()
	m.cursor = len(m.rows) - 1 // last row

	updated, _ := m.Update(specialKey(tea.KeyDown))
	um := updated.(model)

	if um.cursor != len(m.rows)-1 {
		t.Errorf("cursor should stay at %d, got %d", len(m.rows)-1, um.cursor)
	}
}

func TestCursorBounds_EmptyRows(t *testing.T) {
	m := model{
		phase: phaseList,
		keys:  defaultKeyMap(),
		rows:  []WorktreeRow{},
	}

	// Down with empty rows should not panic
	updated, _ := m.Update(specialKey(tea.KeyDown))
	um := updated.(model)
	if um.cursor != 0 {
		t.Errorf("cursor should stay at 0 with empty rows, got %d", um.cursor)
	}
}

// ---------- WindowSizeMsg ----------

func TestWindowSizeMsg(t *testing.T) {
	m := newTestModel()
	m.width = 0
	m.height = 0

	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updated, cmd := m.Update(msg)
	um := updated.(model)

	if um.width != 120 {
		t.Errorf("expected width=120, got %d", um.width)
	}
	if um.height != 40 {
		t.Errorf("expected height=40, got %d", um.height)
	}
	if cmd != nil {
		t.Errorf("expected nil cmd for WindowSizeMsg, got %v", cmd)
	}
}

// ---------- cleanupDoneMsg ----------

func TestCleanupDoneMsg(t *testing.T) {
	m := newTestModel()
	m.phase = phaseCleanup

	results := []git.CleanupResult{
		{Worktree: git.Worktree{Branch: "a"}, Success: true},
		{Worktree: git.Worktree{Branch: "b"}, Success: false, Error: "failed"},
	}
	msg := cleanupDoneMsg{results: results}
	updated, _ := m.Update(msg)
	um := updated.(model)

	if um.phase != phaseDone {
		t.Errorf("expected phaseDone after cleanupDoneMsg, got %d", um.phase)
	}
	if len(um.results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(um.results))
	}
	if !um.results[0].Success {
		t.Error("expected first result to be success")
	}
	if um.results[1].Success {
		t.Error("expected second result to be failure")
	}
}

// ---------- View methods return non-empty ----------

func TestView_Load(t *testing.T) {
	m := model{phase: phaseLoad}
	v := m.View()
	if v == "" {
		t.Error("viewLoad() returned empty string")
	}
}

func TestView_List(t *testing.T) {
	m := newTestModel()
	v := m.View()
	if v == "" {
		t.Error("viewList() returned empty string")
	}
}

func TestView_Confirm(t *testing.T) {
	m := newTestModel()
	m.phase = phaseConfirm
	m.rows[1].Selected = true
	v := m.View()
	if v == "" {
		t.Error("viewConfirm() returned empty string")
	}
}

func TestView_Cleanup(t *testing.T) {
	m := model{phase: phaseCleanup}
	v := m.View()
	if v == "" {
		t.Error("viewCleanup() returned empty string")
	}
}

func TestView_Done_WithResults(t *testing.T) {
	m := model{
		phase: phaseDone,
		results: []git.CleanupResult{
			{Worktree: git.Worktree{Branch: "a", Path: "/repo--a"}, Success: true},
		},
	}
	v := m.View()
	if v == "" {
		t.Error("viewDone() with results returned empty string")
	}
}

func TestView_Done_WithError(t *testing.T) {
	m := model{
		phase:   phaseDone,
		loadErr: errors.New("something went wrong"),
	}
	v := m.View()
	if v == "" {
		t.Error("viewDone() with error returned empty string")
	}
}

func TestView_Done_NoResults(t *testing.T) {
	m := model{phase: phaseDone}
	v := m.View()
	if v == "" {
		t.Error("viewDone() with no results returned empty string")
	}
}

func TestView_Help(t *testing.T) {
	m := model{phase: phaseHelp}
	v := m.View()
	if v == "" {
		t.Error("viewHelp() returned empty string")
	}
}

func TestView_UnknownPhase(t *testing.T) {
	m := model{phase: phase(99)}
	v := m.View()
	if v != "" {
		t.Errorf("expected empty string for unknown phase, got %q", v)
	}
}

// ---------- toggleSortDir ----------

func newSortableModel() model {
	rows := []WorktreeRow{
		{Worktree: git.Worktree{Path: "/repo", Branch: "main", IsMain: true}, State: StateMain},
		{Worktree: git.Worktree{Path: "/repo--alpha", Branch: "alpha"}, State: StateNoPR, Cleanable: true},
		{Worktree: git.Worktree{Path: "/repo--bravo", Branch: "bravo"}, State: StateMerged, Cleanable: true},
		{Worktree: git.Worktree{Path: "/repo--charlie", Branch: "charlie"}, State: StateNoPR, Cleanable: true},
	}
	rowsCopy := make([]WorktreeRow, len(rows))
	copy(rowsCopy, rows)
	return model{
		phase:        phaseList,
		repoPath:     "/repo",
		keys:         defaultKeyMap(),
		rows:         rows,
		allRows:      rowsCopy,
		unsortedRows: rows,
		cursor:       1,
		maxBranch:     7,
		maxStatus:     6,
		sortCol:      SortBranch,
		sortDir:      SortAsc,
		width:        80,
		height:       24,
	}
}

func TestToggleSortDir_AscToDesc(t *testing.T) {
	m := newSortableModel()
	m.sortDir = SortAsc

	m = m.toggleSortDir()
	if m.sortDir != SortDesc {
		t.Errorf("expected SortDesc, got %d", m.sortDir)
	}
}

func TestToggleSortDir_DescToAsc(t *testing.T) {
	m := newSortableModel()
	m.sortDir = SortDesc

	m = m.toggleSortDir()
	if m.sortDir != SortAsc {
		t.Errorf("expected SortAsc, got %d", m.sortDir)
	}
}

func TestToggleSortDir_NoopWhenSortNone(t *testing.T) {
	m := newSortableModel()
	m.sortCol = SortNone
	m.sortDir = SortAsc

	m = m.toggleSortDir()
	if m.sortDir != SortAsc {
		t.Errorf("expected SortAsc unchanged when SortNone, got %d", m.sortDir)
	}
}

func TestToggleSortDir_PreservesCursorByPath(t *testing.T) {
	m := newSortableModel()
	m.cursor = 2 // bravo at /repo--bravo

	m = m.toggleSortDir()
	// After toggling, cursor should track the same row by path
	if m.rows[m.cursor].Worktree.Path != "/repo--bravo" {
		t.Errorf("expected cursor to track /repo--bravo, got %s", m.rows[m.cursor].Worktree.Path)
	}
}

// ---------- advanceSort ----------

func TestAdvanceSort_NoneToFirst(t *testing.T) {
	m := newSortableModel()
	m.sortCol = SortNone
	m.sortDir = SortAsc

	m = m.advanceSort(nextSortColumn)
	if m.sortCol != SortBranch {
		t.Errorf("expected SortBranch, got %d", m.sortCol)
	}
	if m.sortDir != SortAsc {
		t.Errorf("expected SortAsc for new column, got %d", m.sortDir)
	}
}

func TestAdvanceSort_CyclesThroughColumns(t *testing.T) {
	m := newSortableModel()
	m.sortCol = SortNone

	m = m.advanceSort(nextSortColumn) // → Branch
	if m.sortCol != SortBranch {
		t.Errorf("step 1: expected SortBranch, got %d", m.sortCol)
	}
	m = m.advanceSort(nextSortColumn) // → PRNum
	if m.sortCol != SortPRNum {
		t.Errorf("step 2: expected SortPRNum, got %d", m.sortCol)
	}
	m = m.advanceSort(nextSortColumn) // → State
	if m.sortCol != SortState {
		t.Errorf("step 3: expected SortState, got %d", m.sortCol)
	}
	m = m.advanceSort(nextSortColumn) // → None
	if m.sortCol != SortNone {
		t.Errorf("step 4: expected SortNone, got %d", m.sortCol)
	}
}

func TestAdvanceSort_RestoresOriginalOrderOnNone(t *testing.T) {
	m := newSortableModel()
	originalOrder := make([]string, len(m.rows))
	for i, r := range m.rows {
		originalOrder[i] = r.Worktree.Branch
	}

	// Sort by branch, then cycle back to None
	m = m.advanceSort(nextSortColumn) // Branch
	m = m.advanceSort(nextSortColumn) // PRNum
	m = m.advanceSort(nextSortColumn) // State
	m = m.advanceSort(nextSortColumn) // None

	for i, r := range m.rows {
		if r.Worktree.Branch != originalOrder[i] {
			t.Errorf("position %d: expected %s, got %s", i, originalOrder[i], r.Worktree.Branch)
		}
	}
}

func TestAdvanceSort_PreservesSelectionAcrossSortNone(t *testing.T) {
	m := newSortableModel()
	m.rows[1].Selected = true // alpha

	// Sort then unsort
	m = m.advanceSort(nextSortColumn) // Branch
	m = m.advanceSort(nextSortColumn) // PRNum
	m = m.advanceSort(nextSortColumn) // State
	m = m.advanceSort(nextSortColumn) // None

	found := false
	for _, r := range m.rows {
		if r.Worktree.Branch == "alpha" && r.Selected {
			found = true
		}
	}
	if !found {
		t.Error("expected alpha to remain selected after cycling through sort columns")
	}
}

func TestAdvanceSort_PreservesCursorByPath(t *testing.T) {
	m := newSortableModel()
	m.cursor = 2 // bravo

	m = m.advanceSort(nextSortColumn)
	if m.rows[m.cursor].Worktree.Path != "/repo--bravo" {
		t.Errorf("expected cursor to track /repo--bravo, got %s", m.rows[m.cursor].Worktree.Path)
	}
}

// ---------- Sort keybindings in list phase ----------

func TestList_SortNextKey(t *testing.T) {
	m := newSortableModel()
	m.sortCol = SortNone

	updated, _ := m.Update(runeKey('>'))
	um := updated.(model)
	if um.sortCol != SortBranch {
		t.Errorf("expected SortBranch after '>', got %d", um.sortCol)
	}
}

func TestList_SortPrevKey(t *testing.T) {
	m := newSortableModel()
	m.sortCol = SortNone

	updated, _ := m.Update(runeKey('<'))
	um := updated.(model)
	if um.sortCol != SortState {
		t.Errorf("expected SortState after '<', got %d", um.sortCol)
	}
}

func TestList_SortToggleKey(t *testing.T) {
	m := newSortableModel()
	m.sortCol = SortBranch
	m.sortDir = SortAsc

	updated, _ := m.Update(runeKey('s'))
	um := updated.(model)
	if um.sortDir != SortDesc {
		t.Errorf("expected SortDesc after 's', got %d", um.sortDir)
	}
}
