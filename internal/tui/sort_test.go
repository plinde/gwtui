package tui

import (
	"strings"
	"testing"

	"github.com/plinde/gwtui/internal/git"
	gh "github.com/plinde/gwtui/internal/github"
)

func makeRow(branch string, state DisplayState, pr *gh.PR) WorktreeRow {
	return WorktreeRow{
		Worktree: git.Worktree{Path: "/wt--" + branch, Branch: branch},
		PR:       pr,
		State:    state,
	}
}

func makeMainRow() WorktreeRow {
	return WorktreeRow{
		Worktree: git.Worktree{Path: "/repo", Branch: "main", IsMain: true},
		State:    StateMain,
	}
}

func TestSortRows_PinnedAtTop(t *testing.T) {
	rows := []WorktreeRow{
		makeRow("zebra", StateNoPR, nil),
		makeMainRow(),
		makeRow("alpha", StateNoPR, nil),
	}
	sorted := sortRows(rows, SortBranch, SortAsc)

	if sorted[0].State != StateMain {
		t.Errorf("expected main row pinned at top, got %s", sorted[0].State)
	}
	if sorted[1].Worktree.Branch != "alpha" {
		t.Errorf("expected alpha second, got %s", sorted[1].Worktree.Branch)
	}
	if sorted[2].Worktree.Branch != "zebra" {
		t.Errorf("expected zebra third, got %s", sorted[2].Worktree.Branch)
	}
}

func TestSortRows_BranchAsc(t *testing.T) {
	rows := []WorktreeRow{
		makeRow("charlie", StateNoPR, nil),
		makeRow("Alpha", StateNoPR, nil),
		makeRow("bravo", StateNoPR, nil),
	}
	sorted := sortRows(rows, SortBranch, SortAsc)

	want := []string{"Alpha", "bravo", "charlie"}
	for i, w := range want {
		if sorted[i].Worktree.Branch != w {
			t.Errorf("position %d: expected %s, got %s", i, w, sorted[i].Worktree.Branch)
		}
	}
}

func TestSortRows_BranchDesc(t *testing.T) {
	rows := []WorktreeRow{
		makeRow("alpha", StateNoPR, nil),
		makeRow("charlie", StateNoPR, nil),
		makeRow("bravo", StateNoPR, nil),
	}
	sorted := sortRows(rows, SortBranch, SortDesc)

	want := []string{"charlie", "bravo", "alpha"}
	for i, w := range want {
		if sorted[i].Worktree.Branch != w {
			t.Errorf("position %d: expected %s, got %s", i, w, sorted[i].Worktree.Branch)
		}
	}
}

func TestSortRows_PRNumAsc(t *testing.T) {
	rows := []WorktreeRow{
		makeRow("c", StateMerged, &gh.PR{Number: 50}),
		makeRow("a", StateNoPR, nil),
		makeRow("b", StateActive, &gh.PR{Number: 10}),
	}
	sorted := sortRows(rows, SortPRNum, SortAsc)

	if sorted[0].PR.Number != 10 {
		t.Errorf("expected PR#10 first, got %d", sorted[0].PR.Number)
	}
	if sorted[1].PR.Number != 50 {
		t.Errorf("expected PR#50 second, got %d", sorted[1].PR.Number)
	}
	if sorted[2].PR != nil {
		t.Error("expected nil PR last")
	}
}

func TestSortRows_PRNumDesc(t *testing.T) {
	rows := []WorktreeRow{
		makeRow("a", StateActive, &gh.PR{Number: 10}),
		makeRow("b", StateNoPR, nil),
		makeRow("c", StateMerged, &gh.PR{Number: 50}),
	}
	sorted := sortRows(rows, SortPRNum, SortDesc)

	// Nil-PR rows stay at end regardless of direction
	if sorted[0].PR == nil {
		t.Error("nil-PR row should be at the end even in desc order")
	}
	if sorted[0].PR.Number != 50 {
		t.Errorf("expected PR#50 first in desc, got %d", sorted[0].PR.Number)
	}
	if sorted[2].PR != nil {
		t.Error("expected nil-PR row last")
	}
}

func TestSortRows_StateAsc(t *testing.T) {
	rows := []WorktreeRow{
		makeRow("a", StateClosed, &gh.PR{Number: 1, State: "CLOSED"}),
		makeRow("b", StateActive, &gh.PR{Number: 2, State: "OPEN"}),
		makeRow("c", StateNoPR, nil),
		makeRow("d", StateMerged, &gh.PR{Number: 3, State: "MERGED"}),
		makeRow("e", StateDraft, &gh.PR{Number: 4, State: "OPEN", IsDraft: true}),
	}
	sorted := sortRows(rows, SortState, SortAsc)

	wantOrder := []DisplayState{StateActive, StateDraft, StateNoPR, StateMerged, StateClosed}
	for i, w := range wantOrder {
		if sorted[i].State != w {
			t.Errorf("position %d: expected %s, got %s", i, w, sorted[i].State)
		}
	}
}

func TestSortRows_StateDesc(t *testing.T) {
	rows := []WorktreeRow{
		makeRow("a", StateActive, &gh.PR{Number: 1, State: "OPEN"}),
		makeRow("b", StateClosed, &gh.PR{Number: 2, State: "CLOSED"}),
		makeRow("c", StateMerged, &gh.PR{Number: 3, State: "MERGED"}),
	}
	sorted := sortRows(rows, SortState, SortDesc)

	wantOrder := []DisplayState{StateClosed, StateMerged, StateActive}
	for i, w := range wantOrder {
		if sorted[i].State != w {
			t.Errorf("position %d: expected %s, got %s", i, w, sorted[i].State)
		}
	}
}

func TestSortRows_NonePreservesOrder(t *testing.T) {
	rows := []WorktreeRow{
		makeRow("c", StateNoPR, nil),
		makeRow("a", StateNoPR, nil),
		makeRow("b", StateNoPR, nil),
	}
	sorted := sortRows(rows, SortNone, SortAsc)

	for i, r := range rows {
		if sorted[i].Worktree.Branch != r.Worktree.Branch {
			t.Errorf("position %d: expected %s, got %s", i, r.Worktree.Branch, sorted[i].Worktree.Branch)
		}
	}
}

func TestSortRows_StableSort(t *testing.T) {
	rows := []WorktreeRow{
		makeRow("b", StateNoPR, nil),
		makeRow("a", StateNoPR, nil),
		makeRow("c", StateNoPR, nil),
	}
	sorted := sortRows(rows, SortState, SortAsc)

	// All same state, stable sort preserves original order
	want := []string{"b", "a", "c"}
	for i, w := range want {
		if sorted[i].Worktree.Branch != w {
			t.Errorf("position %d: expected %s, got %s (stable sort broken)", i, w, sorted[i].Worktree.Branch)
		}
	}
}

func TestRenderHeader_NoSort(t *testing.T) {
	h := renderHeader(SortNone, SortAsc, 10, 10)
	if strings.Contains(h, "▲") || strings.Contains(h, "▼") {
		t.Error("expected no sort indicator when SortNone")
	}
	if !strings.Contains(h, "BRANCH") {
		t.Error("expected BRANCH label")
	}
}

func TestRenderHeader_BranchAsc(t *testing.T) {
	h := renderHeader(SortBranch, SortAsc, 10, 10)
	if !strings.Contains(h, "BRANCH ▲") {
		t.Errorf("expected 'BRANCH ▲' in header, got: %s", h)
	}
	if strings.Contains(h, "STATUS ▲") || strings.Contains(h, "STATUS ▼") {
		t.Error("indicator should only be on BRANCH")
	}
}

func TestRenderHeader_PipeDelimiters(t *testing.T) {
	h := renderHeader(SortNone, SortAsc, 10, 10)
	if !strings.Contains(h, "│") {
		t.Error("expected pipe delimiters in header")
	}
}

func TestRenderHeader_StateDesc(t *testing.T) {
	h := renderHeader(SortState, SortDesc, 10, 10)
	if !strings.Contains(h, "STATUS ▼") {
		t.Errorf("expected 'STATUS ▼' in header, got: %s", h)
	}
}

func TestRenderHeader_PRNumAsc(t *testing.T) {
	h := renderHeader(SortPRNum, SortAsc, 10, 10)
	if !strings.Contains(h, "STATUS ▲") {
		t.Errorf("expected 'STATUS ▲' for PR# sort, got: %s", h)
	}
}

func TestNextSortColumn(t *testing.T) {
	tests := []struct {
		from SortColumn
		want SortColumn
	}{
		{SortNone, SortBranch},
		{SortBranch, SortPRNum},
		{SortPRNum, SortState},
		{SortState, SortNone},
	}
	for _, tt := range tests {
		got := nextSortColumn(tt.from)
		if got != tt.want {
			t.Errorf("nextSortColumn(%d) = %d, want %d", tt.from, got, tt.want)
		}
	}
}

func TestPrevSortColumn(t *testing.T) {
	tests := []struct {
		from SortColumn
		want SortColumn
	}{
		{SortNone, SortState},
		{SortState, SortPRNum},
		{SortPRNum, SortBranch},
		{SortBranch, SortNone},
	}
	for _, tt := range tests {
		got := prevSortColumn(tt.from)
		if got != tt.want {
			t.Errorf("prevSortColumn(%d) = %d, want %d", tt.from, got, tt.want)
		}
	}
}

func TestSortColumnLabel(t *testing.T) {
	tests := []struct {
		col  SortColumn
		want string
	}{
		{SortBranch, "BRANCH"},
		{SortPRNum, "PR#"},
		{SortState, "STATE"},
		{SortNone, ""},
	}
	for _, tt := range tests {
		got := sortColumnLabel(tt.col)
		if got != tt.want {
			t.Errorf("sortColumnLabel(%d) = %q, want %q", tt.col, got, tt.want)
		}
	}
}
