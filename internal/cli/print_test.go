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
