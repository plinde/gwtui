package cli

import (
	"testing"

	"github.com/plinde/gwtui/internal/git"
	gh "github.com/plinde/gwtui/internal/github"
	"github.com/plinde/gwtui/internal/tui"
)

func TestPlainStatus_MergedPR(t *testing.T) {
	row := tui.WorktreeRow{
		Worktree: git.Worktree{Branch: "feat"},
		PR:       &gh.PR{Number: 42, State: "MERGED"},
		State:    tui.StateMerged,
	}
	got := plainStatus(row)
	if got != "#42 merged" {
		t.Errorf("expected '#42 merged', got %q", got)
	}
}

func TestPlainStatus_OpenPR(t *testing.T) {
	row := tui.WorktreeRow{
		Worktree: git.Worktree{Branch: "feat"},
		PR:       &gh.PR{Number: 10, State: "OPEN"},
		State:    tui.StateActive,
	}
	got := plainStatus(row)
	if got != "#10 open" {
		t.Errorf("expected '#10 open', got %q", got)
	}
}

func TestPlainStatus_DraftPR(t *testing.T) {
	row := tui.WorktreeRow{
		Worktree: git.Worktree{Branch: "feat"},
		PR:       &gh.PR{Number: 5, State: "OPEN", IsDraft: true},
		State:    tui.StateDraft,
	}
	got := plainStatus(row)
	if got != "#5 open (draft)" {
		t.Errorf("expected '#5 open (draft)', got %q", got)
	}
}

func TestPlainStatus_ClosedPR(t *testing.T) {
	row := tui.WorktreeRow{
		Worktree: git.Worktree{Branch: "feat"},
		PR:       &gh.PR{Number: 99, State: "CLOSED"},
		State:    tui.StateClosed,
	}
	got := plainStatus(row)
	if got != "#99 closed" {
		t.Errorf("expected '#99 closed', got %q", got)
	}
}

func TestPlainStatus_Main(t *testing.T) {
	row := tui.WorktreeRow{
		Worktree: git.Worktree{Branch: "main", IsMain: true},
		State:    tui.StateMain,
	}
	got := plainStatus(row)
	if got != "main" {
		t.Errorf("expected 'main', got %q", got)
	}
}

func TestPlainStatus_NoPR(t *testing.T) {
	row := tui.WorktreeRow{
		Worktree: git.Worktree{Branch: "feat"},
		State:    tui.StateNoPR,
	}
	got := plainStatus(row)
	if got != "no PR" {
		t.Errorf("expected 'no PR', got %q", got)
	}
}

func TestPlainStatus_Fallback(t *testing.T) {
	row := tui.WorktreeRow{
		Worktree: git.Worktree{Branch: "feat"},
		State:    "unknown",
	}
	got := plainStatus(row)
	if got != "-" {
		t.Errorf("expected '-', got %q", got)
	}
}

func TestPlainStatus_LargePRNumber(t *testing.T) {
	row := tui.WorktreeRow{
		Worktree: git.Worktree{Branch: "feat"},
		PR:       &gh.PR{Number: 99999, State: "OPEN"},
		State:    tui.StateActive,
	}
	got := plainStatus(row)
	if got != "#99999 open" {
		t.Errorf("expected '#99999 open', got %q", got)
	}
}

func TestPlainStatus_MainWithBareWorktree(t *testing.T) {
	row := tui.WorktreeRow{
		Worktree: git.Worktree{Branch: "main", IsMain: true, IsBare: true},
		State:    tui.StateMain,
	}
	got := plainStatus(row)
	if got != "main" {
		t.Errorf("expected 'main', got %q", got)
	}
}

func TestPlainStatus_AllDisplayStates(t *testing.T) {
	// Verify every known DisplayState value produces a non-empty result.
	states := []struct {
		state tui.DisplayState
		pr    *gh.PR
	}{
		{tui.StateActive, &gh.PR{Number: 1, State: "OPEN"}},
		{tui.StateDraft, &gh.PR{Number: 2, State: "OPEN", IsDraft: true}},
		{tui.StateMerged, &gh.PR{Number: 3, State: "MERGED"}},
		{tui.StateClosed, &gh.PR{Number: 4, State: "CLOSED"}},
		{tui.StateNoPR, nil},
		{tui.StateMain, nil},
	}
	for _, tc := range states {
		row := tui.WorktreeRow{
			Worktree: git.Worktree{Branch: "b"},
			PR:       tc.pr,
			State:    tc.state,
		}
		got := plainStatus(row)
		if got == "" {
			t.Errorf("plainStatus returned empty string for state %q", tc.state)
		}
	}
}
