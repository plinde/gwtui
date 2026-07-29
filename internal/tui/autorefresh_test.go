package tui

import (
	"errors"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/plinde/gwtui/internal/git"
	gh "github.com/plinde/gwtui/internal/github"
)

// testModel returns a model in phaseList with a standard set of rows.
func testModel() model {
	rows := []WorktreeRow{
		{Worktree: git.Worktree{Path: "/repo", Branch: "main", IsMain: true}, State: StateMain, Cleanable: false},
		{Worktree: git.Worktree{Path: "/repo--a", Branch: "a"}, State: StateMerged, Cleanable: true},
		{Worktree: git.Worktree{Path: "/repo--b", Branch: "b"}, State: StateNoPR, Cleanable: false},
		{Worktree: git.Worktree{Path: "/repo--c", Branch: "c"}, State: StateActive, Cleanable: false},
	}
	return model{
		phase:     phaseList,
		repoPath:  "/repo",
		keys:      defaultKeyMap(),
		spinner:   spinner.New(),
		rows:      rows,
		cursor:    1,
		maxBranch: 4,
		maxStatus: 5,
		width:     80,
		height:    24,
	}
}

// ---------- scheduleAutoRefresh / doAutoRefresh ----------

func TestScheduleAutoRefresh_ReturnsNonNilCmd(t *testing.T) {
	cmd := scheduleAutoRefresh(1)
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd from scheduleAutoRefresh")
	}
}

func TestDoAutoRefresh_ReturnsNonNilCmd(t *testing.T) {
	cmd := doAutoRefresh("/tmp/fakerepo", "", 1)
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd from doAutoRefresh")
	}
}

// ---------- autoRefreshTickMsg ----------

func TestAutoRefreshTickMsg_InListPhase(t *testing.T) {
	m := testModel()

	updated, cmd := m.Update(autoRefreshTickMsg{})
	um := updated.(model)

	if um.phase != phaseList {
		t.Errorf("expected phase to stay phaseList, got %d", um.phase)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd (doAutoRefresh) when in phaseList")
	}
}

func TestAutoRefreshTickMsg_InOtherPhase(t *testing.T) {
	for _, ph := range []phase{phaseLoad, phaseConfirm, phaseConfirmOpenPR, phaseCleanup, phaseDone, phaseHelp} {
		m := testModel()
		m.phase = ph

		updated, cmd := m.Update(autoRefreshTickMsg{})
		um := updated.(model)

		if um.phase != ph {
			t.Errorf("phase %d: expected phase unchanged, got %d", ph, um.phase)
		}
		if cmd == nil {
			t.Errorf("phase %d: expected non-nil cmd (reschedule tick)", ph)
		}
	}
}

func TestAutoRefreshTickMsg_StaleGenerationIsIgnored(t *testing.T) {
	m := testModel()
	m.refreshGeneration = 2

	updated, cmd := m.Update(autoRefreshTickMsg{generation: 1})
	um := updated.(model)

	if cmd != nil {
		t.Fatal("stale timer must not start or reschedule a refresh")
	}
	if um.refreshInFlight {
		t.Fatal("stale timer marked refresh in flight")
	}
}

func TestAutoRefreshTickMsg_AllowsOnlyOneInFlightRefresh(t *testing.T) {
	m := testModel()
	m.refreshGeneration = 3
	tick := autoRefreshTickMsg{generation: 3}

	updated, cmd := m.Update(tick)
	um := updated.(model)
	if cmd == nil || !um.refreshInFlight {
		t.Fatal("current timer did not start refresh")
	}

	updated, duplicateCmd := um.Update(tick)
	um = updated.(model)
	if duplicateCmd != nil {
		t.Fatal("duplicate timer started a concurrent refresh")
	}
	if !um.refreshInFlight {
		t.Fatal("refresh unexpectedly stopped being in flight")
	}
}

// ---------- autoRefreshDoneMsg ----------

func TestAutoRefreshDoneMsg_PreservesSelections(t *testing.T) {
	m := testModel()
	m.rows[1].Selected = true // branch "a"
	m.rows[2].Selected = true // branch "b"

	msg := autoRefreshDoneMsg{
		worktrees: []git.Worktree{
			{Path: "/repo", Branch: "main", IsMain: true},
			{Path: "/repo--a", Branch: "a"},
			{Path: "/repo--b", Branch: "b"},
			{Path: "/repo--c", Branch: "c"},
		},
		prs: map[string]*gh.PR{},
	}

	updated, cmd := m.Update(msg)
	um := updated.(model)

	if um.phase != phaseList {
		t.Errorf("expected phaseList, got %d", um.phase)
	}
	for _, r := range um.rows {
		switch r.Worktree.Branch {
		case "a", "b":
			if !r.Selected {
				t.Errorf("branch %q should still be selected", r.Worktree.Branch)
			}
		default:
			if r.Selected {
				t.Errorf("branch %q should not be selected", r.Worktree.Branch)
			}
		}
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (reschedule)")
	}
}

func TestAutoRefreshDoneMsg_ClampsCursor(t *testing.T) {
	m := testModel()
	m.cursor = 3 // last row (index 3 of 4)

	msg := autoRefreshDoneMsg{
		worktrees: []git.Worktree{
			{Path: "/repo", Branch: "main", IsMain: true},
			{Path: "/repo--a", Branch: "a"},
		},
		prs: map[string]*gh.PR{},
	}

	updated, _ := m.Update(msg)
	um := updated.(model)

	if um.cursor >= len(um.rows) {
		t.Errorf("cursor %d out of bounds for %d rows", um.cursor, len(um.rows))
	}
	if um.cursor != 1 {
		t.Errorf("expected cursor clamped to 1, got %d", um.cursor)
	}
}

func TestAutoRefreshDoneMsg_WrongPhase(t *testing.T) {
	m := testModel()
	m.phase = phaseConfirm

	msg := autoRefreshDoneMsg{
		worktrees: []git.Worktree{
			{Path: "/repo", Branch: "main", IsMain: true},
		},
		prs: map[string]*gh.PR{},
	}

	updated, cmd := m.Update(msg)
	um := updated.(model)

	if um.phase != phaseConfirm {
		t.Errorf("expected phase unchanged (phaseConfirm), got %d", um.phase)
	}
	if len(um.rows) != 4 {
		t.Errorf("expected rows unchanged (4), got %d", len(um.rows))
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (reschedule)")
	}
}

func TestAutoRefreshDoneMsg_Error(t *testing.T) {
	m := testModel()

	msg := autoRefreshDoneMsg{err: errors.New("network error")}

	updated, cmd := m.Update(msg)
	um := updated.(model)

	if um.phase != phaseList {
		t.Errorf("expected phaseList on error, got %d", um.phase)
	}
	if len(um.rows) != 4 {
		t.Errorf("expected rows unchanged (4), got %d", len(um.rows))
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (reschedule)")
	}
}

func TestAutoRefreshDoneMsg_EmptyList(t *testing.T) {
	m := testModel()
	m.cursor = 2

	msg := autoRefreshDoneMsg{
		worktrees: []git.Worktree{},
		prs:       map[string]*gh.PR{},
	}

	updated, _ := m.Update(msg)
	um := updated.(model)

	if len(um.rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(um.rows))
	}
	if um.cursor != 0 {
		t.Errorf("expected cursor clamped to 0, got %d", um.cursor)
	}
}

func TestAutoRefreshDoneMsg_StaleGenerationDoesNotReplaceRows(t *testing.T) {
	m := testModel()
	m.refreshGeneration = 4
	originalRows := len(m.rows)

	updated, cmd := m.Update(autoRefreshDoneMsg{
		generation: 3,
		rows: []WorktreeRow{{
			Worktree: git.Worktree{Path: "/stale", Branch: "stale"},
		}},
	})
	um := updated.(model)

	if cmd != nil {
		t.Fatal("stale completion must not create another timer")
	}
	if len(um.rows) != originalRows {
		t.Fatalf("stale completion replaced rows: got %d, want %d", len(um.rows), originalRows)
	}
}

// ---------- handleLoadDone reschedules ----------

func TestHandleLoadDone_ReschedulesAutoRefresh(t *testing.T) {
	m := model{
		phase:    phaseLoad,
		repoPath: "/repo",
		keys:     defaultKeyMap(),
		spinner:  spinner.New(),
	}

	wts := []git.Worktree{
		{Path: "/repo", Branch: "main", IsMain: true},
	}
	msg := loadDoneMsg{worktrees: wts, prs: map[string]*gh.PR{}}

	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Error("expected non-nil cmd (scheduleAutoRefresh) after loadDone")
	}
}

func TestHandleLoadDone_Error_ReschedulesAutoRefresh(t *testing.T) {
	m := model{
		phase:    phaseLoad,
		repoPath: "/repo",
		keys:     defaultKeyMap(),
		spinner:  spinner.New(),
	}

	msg := loadDoneMsg{err: errors.New("fail")}

	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Error("expected non-nil cmd (scheduleAutoRefresh) even on error")
	}
}

// ---------- Init starts loading; load completion owns the refresh timer ----------

func TestInit_DoesNotScheduleCompetingAutoRefresh(t *testing.T) {
	m := model{
		repoPath: "/repo",
		spinner:  spinner.New(),
	}

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected non-nil cmd from Init()")
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg, got %T", msg)
	}
	if len(batch) != 2 {
		t.Fatalf("expected spinner + initial load only, got %d commands", len(batch))
	}
}

// ---------- Done-screen countdown ----------

func TestCleanupDone_StartsCountdown(t *testing.T) {
	m := testModel()
	m.phase = phaseCleanup

	results := []git.CleanupResult{
		{Worktree: git.Worktree{Branch: "a"}, Success: true},
	}
	msg := cleanupDoneMsg{results: results}

	updated, cmd := m.Update(msg)
	um := updated.(model)

	if um.phase != phaseDone {
		t.Errorf("expected phaseDone, got %d", um.phase)
	}
	if um.doneCountdown != doneCountdownSeconds {
		t.Errorf("expected doneCountdown=%d, got %d", doneCountdownSeconds, um.doneCountdown)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (scheduleDoneCountdown)")
	}
}

func TestCleanupDone_EmptyResults_NoCountdown(t *testing.T) {
	m := testModel()
	m.phase = phaseCleanup

	msg := cleanupDoneMsg{results: nil}

	updated, cmd := m.Update(msg)
	um := updated.(model)

	if um.doneCountdown != 0 {
		t.Errorf("expected no countdown with empty results, got %d", um.doneCountdown)
	}
	if cmd != nil {
		t.Error("expected nil cmd with no results")
	}
}

func TestDoneCountdownTick_Decrements(t *testing.T) {
	m := testModel()
	m.phase = phaseDone
	m.doneCountdown = 3

	updated, cmd := m.Update(doneCountdownTickMsg{})
	um := updated.(model)

	if um.doneCountdown != 2 {
		t.Errorf("expected countdown=2, got %d", um.doneCountdown)
	}
	if um.phase != phaseDone {
		t.Errorf("expected still in phaseDone, got %d", um.phase)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (next tick)")
	}
}

func TestDoneCountdownTick_ReachesZero_ReturnsToLoad(t *testing.T) {
	m := testModel()
	m.phase = phaseDone
	m.doneCountdown = 1
	m.results = []git.CleanupResult{{Worktree: git.Worktree{Branch: "a"}, Success: true}}

	updated, cmd := m.Update(doneCountdownTickMsg{})
	um := updated.(model)

	if um.phase != phaseLoad {
		t.Errorf("expected phaseLoad after countdown expires, got %d", um.phase)
	}
	if um.doneCountdown != 0 {
		t.Errorf("expected countdown=0, got %d", um.doneCountdown)
	}
	if um.results != nil {
		t.Error("expected results cleared")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (reload)")
	}
}

func TestDoneCountdownTick_WrongPhase_Ignored(t *testing.T) {
	m := testModel()
	m.phase = phaseList
	m.doneCountdown = 3

	updated, cmd := m.Update(doneCountdownTickMsg{})
	um := updated.(model)

	// Should not decrement or transition
	if um.doneCountdown != 3 {
		t.Errorf("expected countdown unchanged at 3, got %d", um.doneCountdown)
	}
	if cmd != nil {
		t.Errorf("expected nil cmd when not in phaseDone")
	}
}

func TestAutoRefreshDoneMsg_PreservesSortOrder(t *testing.T) {
	m := testModel()
	m.sortCol = SortState
	m.sortDir = SortDesc

	msg := autoRefreshDoneMsg{
		worktrees: []git.Worktree{
			{Path: "/repo", Branch: "main", IsMain: true},
			{Path: "/repo--open", Branch: "open"},
			{Path: "/repo--merged", Branch: "merged"},
			{Path: "/repo--closed", Branch: "closed"},
		},
		prs: map[string]*gh.PR{
			"open":   {Number: 1, State: "OPEN", HeadRef: "open"},
			"merged": {Number: 2, State: "MERGED", HeadRef: "merged"},
			"closed": {Number: 3, State: "CLOSED", HeadRef: "closed"},
		},
	}

	updated, _ := m.Update(msg)
	um := updated.(model)

	// With SortState desc: main (pinned) → closed → merged → open
	wantStates := []DisplayState{StateMain, StateClosed, StateMerged, StateActive}
	for i, want := range wantStates {
		if i >= len(um.rows) {
			t.Fatalf("only %d rows, expected at least %d", len(um.rows), i+1)
		}
		if um.rows[i].State != want {
			t.Errorf("position %d: expected %s, got %s", i, want, um.rows[i].State)
		}
	}
}

func TestAutoRefreshDoneMsg_NoSortWhenSortNone(t *testing.T) {
	m := testModel()
	m.sortCol = SortNone
	m.sortDir = SortAsc

	msg := autoRefreshDoneMsg{
		worktrees: []git.Worktree{
			{Path: "/repo", Branch: "main", IsMain: true},
			{Path: "/repo--c", Branch: "c"},
			{Path: "/repo--a", Branch: "a"},
		},
		prs: map[string]*gh.PR{},
	}

	updated, _ := m.Update(msg)
	um := updated.(model)

	// With SortNone, insertion order preserved: main, c, a
	want := []string{"main", "c", "a"}
	for i, w := range want {
		if um.rows[i].Worktree.Branch != w {
			t.Errorf("position %d: expected %s, got %s", i, w, um.rows[i].Worktree.Branch)
		}
	}
}

func TestDoneEnter_CancelsCountdown(t *testing.T) {
	m := testModel()
	m.phase = phaseDone
	m.doneCountdown = 4
	m.results = []git.CleanupResult{{Worktree: git.Worktree{Branch: "a"}, Success: true}}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um := updated.(model)

	if um.phase != phaseLoad {
		t.Errorf("expected phaseLoad after enter, got %d", um.phase)
	}
	if um.doneCountdown != 0 {
		t.Errorf("expected countdown reset to 0, got %d", um.doneCountdown)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (reload)")
	}
}
