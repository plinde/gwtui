package tui

import (
	"testing"

	"github.com/plinde/gwtui/internal/git"
	gh "github.com/plinde/gwtui/internal/github"
)

func TestEnrichWorktrees_MergedPR(t *testing.T) {
	wts := []git.Worktree{{Path: "/repo--feat", Branch: "feat"}}
	prs := map[string]*gh.PR{
		"feat": {Number: 10, State: "MERGED", HeadRef: "feat"},
	}
	rows := EnrichWorktrees(wts, prs)

	if rows[0].State != StateMerged {
		t.Errorf("expected StateMerged, got %s", rows[0].State)
	}
	if !rows[0].Cleanable {
		t.Error("expected Cleanable=true for merged PR")
	}
	if rows[0].PR == nil || rows[0].PR.Number != 10 {
		t.Error("expected PR #10 to be attached")
	}
}

func TestEnrichWorktrees_OpenPR(t *testing.T) {
	wts := []git.Worktree{{Path: "/repo--feat", Branch: "feat"}}
	prs := map[string]*gh.PR{
		"feat": {Number: 20, State: "OPEN", HeadRef: "feat"},
	}
	rows := EnrichWorktrees(wts, prs)

	if rows[0].State != StateActive {
		t.Errorf("expected StateActive, got %s", rows[0].State)
	}
	if rows[0].Cleanable {
		t.Error("expected Cleanable=false for open PR")
	}
}

func TestEnrichWorktrees_DraftPR(t *testing.T) {
	wts := []git.Worktree{{Path: "/repo--feat", Branch: "feat"}}
	prs := map[string]*gh.PR{
		"feat": {Number: 30, State: "OPEN", IsDraft: true, HeadRef: "feat"},
	}
	rows := EnrichWorktrees(wts, prs)

	if rows[0].State != StateDraft {
		t.Errorf("expected StateDraft, got %s", rows[0].State)
	}
	if rows[0].Cleanable {
		t.Error("expected Cleanable=false for draft PR")
	}
}

func TestEnrichWorktrees_ClosedPR(t *testing.T) {
	wts := []git.Worktree{{Path: "/repo--feat", Branch: "feat"}}
	prs := map[string]*gh.PR{
		"feat": {Number: 40, State: "CLOSED", HeadRef: "feat"},
	}
	rows := EnrichWorktrees(wts, prs)

	if rows[0].State != StateClosed {
		t.Errorf("expected StateClosed, got %s", rows[0].State)
	}
	if !rows[0].Cleanable {
		t.Error("expected Cleanable=true for closed PR")
	}
}

func TestEnrichWorktrees_NoPR(t *testing.T) {
	wts := []git.Worktree{{Path: "/repo--feat", Branch: "feat"}}
	prs := map[string]*gh.PR{}
	rows := EnrichWorktrees(wts, prs)

	if rows[0].State != StateNoPR {
		t.Errorf("expected StateNoPR, got %s", rows[0].State)
	}
	if !rows[0].Cleanable {
		t.Error("expected Cleanable=true for no PR")
	}
	if rows[0].PR != nil {
		t.Error("expected PR to be nil")
	}
}

func TestEnrichWorktrees_MainWorktree(t *testing.T) {
	wts := []git.Worktree{{Path: "/repo", Branch: "main", IsMain: true}}
	prs := map[string]*gh.PR{}
	rows := EnrichWorktrees(wts, prs)

	if rows[0].State != StateMain {
		t.Errorf("expected StateMain, got %s", rows[0].State)
	}
	if rows[0].Cleanable {
		t.Error("expected Cleanable=false for main worktree")
	}
}

func TestEnrichWorktrees_BareWorktree(t *testing.T) {
	wts := []git.Worktree{{Path: "/repo.git", IsBare: true}}
	prs := map[string]*gh.PR{}
	rows := EnrichWorktrees(wts, prs)

	if rows[0].State != StateMain {
		t.Errorf("expected StateMain for bare worktree, got %s", rows[0].State)
	}
	if rows[0].Cleanable {
		t.Error("expected Cleanable=false for bare worktree")
	}
}

func TestEnrichWorktrees_MultipleRows(t *testing.T) {
	wts := []git.Worktree{
		{Path: "/repo", Branch: "main", IsMain: true},
		{Path: "/repo--a", Branch: "a"},
		{Path: "/repo--b", Branch: "b"},
	}
	prs := map[string]*gh.PR{
		"a": {Number: 1, State: "MERGED", HeadRef: "a"},
	}
	rows := EnrichWorktrees(wts, prs)

	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	if rows[0].State != StateMain {
		t.Errorf("row 0: expected StateMain, got %s", rows[0].State)
	}
	if rows[1].State != StateMerged {
		t.Errorf("row 1: expected StateMerged, got %s", rows[1].State)
	}
	if rows[2].State != StateNoPR {
		t.Errorf("row 2: expected StateNoPR, got %s", rows[2].State)
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		input    string
		width    int
		expected string
	}{
		{"abc", 6, "abc   "},
		{"abcdef", 3, "abcdef"},
		{"abc", 3, "abc"},
		{"", 4, "    "},
	}
	for _, tt := range tests {
		got := padRight(tt.input, tt.width)
		if got != tt.expected {
			t.Errorf("padRight(%q, %d) = %q, want %q", tt.input, tt.width, got, tt.expected)
		}
	}
}

func TestColumnWidths(t *testing.T) {
	rows := []WorktreeRow{
		{Worktree: git.Worktree{Branch: "main"}, State: StateMain},
		{Worktree: git.Worktree{Branch: "feature/long-name"}, State: StateNoPR},
		{Worktree: git.Worktree{Branch: ""}, State: StateNoPR}, // detached
	}
	maxBranch, maxStatus := ColumnWidths(rows)

	// "feature/long-name" = 17 chars
	if maxBranch != 17 {
		t.Errorf("expected maxBranch=17, got %d", maxBranch)
	}
	// "no PR" = 5, "main" = 4 → max is 5
	if maxStatus != 5 {
		t.Errorf("expected maxStatus=5, got %d", maxStatus)
	}
}

func TestColumnWidths_WithPR(t *testing.T) {
	rows := []WorktreeRow{
		{
			Worktree: git.Worktree{Branch: "feat"},
			PR:       &gh.PR{Number: 123, State: "MERGED"},
			State:    StateMerged,
		},
	}
	_, maxStatus := ColumnWidths(rows)

	// "#123 merged" = 11 chars
	if maxStatus != 11 {
		t.Errorf("expected maxStatus=11, got %d", maxStatus)
	}
}

func TestColumnWidths_DraftPR(t *testing.T) {
	rows := []WorktreeRow{
		{
			Worktree: git.Worktree{Branch: "feat"},
			PR:       &gh.PR{Number: 5, State: "OPEN", IsDraft: true},
			State:    StateDraft,
		},
	}
	_, maxStatus := ColumnWidths(rows)

	// "#5 open (draft)" = 15 chars
	if maxStatus != 15 {
		t.Errorf("expected maxStatus=15, got %d", maxStatus)
	}
}

func TestColumnWidths_DetachedBranch(t *testing.T) {
	rows := []WorktreeRow{
		{Worktree: git.Worktree{Branch: ""}, State: StateNoPR},
	}
	maxBranch, _ := ColumnWidths(rows)

	// "(detached)" = 10 chars
	if maxBranch != 10 {
		t.Errorf("expected maxBranch=10 for detached, got %d", maxBranch)
	}
}
