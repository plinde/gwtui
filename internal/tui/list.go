package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/plinde/gwtui/internal/git"
	gh "github.com/plinde/gwtui/internal/github"
)

// DisplayState represents the derived display state for a worktree row.
type DisplayState string

const (
	StateActive DisplayState = "open:ready"
	StateDraft  DisplayState = "open:draft"
	StateMerged DisplayState = "merged"
	StateClosed DisplayState = "closed"
	StateNoPR   DisplayState = "no-pr"
	StateMain   DisplayState = "main"
)

// WorktreeRow is the enriched view model for a single worktree.
type WorktreeRow struct {
	Worktree  git.Worktree
	PR        *gh.PR
	State     DisplayState
	Cleanable bool
	Selected  bool
}

// EnrichWorktrees combines worktree data with PR data into display rows.
func EnrichWorktrees(worktrees []git.Worktree, prs map[string]*gh.PR) []WorktreeRow {
	rows := make([]WorktreeRow, 0, len(worktrees))
	for _, wt := range worktrees {
		row := WorktreeRow{Worktree: wt}

		if wt.IsMain || wt.IsBare {
			row.State = StateMain
			row.Cleanable = false
		} else if pr, ok := prs[wt.Branch]; ok {
			row.PR = pr
			switch pr.State {
			case "MERGED":
				row.State = StateMerged
				row.Cleanable = true
			case "CLOSED":
				row.State = StateClosed
				row.Cleanable = true
			case "OPEN":
				if pr.IsDraft {
					row.State = StateDraft
				} else {
					row.State = StateActive
				}
				row.Cleanable = false
			}
		} else {
			row.State = StateNoPR
			row.Cleanable = false
		}

		rows = append(rows, row)
	}
	return rows
}

// CompressPath abbreviates a filesystem path for display.
func CompressPath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if strings.HasPrefix(p, home) {
		p = "~" + p[len(home):]
	}
	// Further compress: ~/workspace/github.com/org/repo--suffix → ~/...repo--suffix
	parts := strings.Split(p, string(filepath.Separator))
	if len(parts) > 4 {
		return "~/..." + parts[len(parts)-1]
	}
	return p
}

// RenderRow renders a single worktree row for the list view.
func RenderRow(row WorktreeRow, isCursor bool, maxBranch, maxStatus int) string {
	// Cursor indicator
	cursor := "  "
	if isCursor {
		cursor = cursorStyle.Render("▸ ")
	}

	// Checkbox
	var checkbox string
	switch {
	case !row.Cleanable:
		checkbox = checkboxReadOnlyStyle.Render("    ")
	case row.Selected:
		checkbox = checkboxCleanableStyle.Render("[x] ")
	default:
		checkbox = checkboxCleanableStyle.Render("[ ] ")
	}

	// Branch name
	branch := row.Worktree.Branch
	if branch == "" {
		branch = "(detached)"
	}
	branchCol := branchStyle.Render(padRight(branch, maxBranch))
	if row.State == StateMain {
		branchCol = stateMainStyle.Render(padRight(branch, maxBranch))
	}

	// Status column — pad using visible width since renderStatus returns styled text
	statusText := renderStatus(row)
	statusCol := padRightVisible(statusText, maxStatus)

	// Path column
	pathCol := pathStyle.Render(CompressPath(row.Worktree.Path))

	sep := dimStyle.Render(" │ ")
	return cursor + checkbox + branchCol + sep + statusCol + sep + pathCol
}

func renderStatus(row WorktreeRow) string {
	if row.PR != nil {
		label := fmt.Sprintf("#%d %s", row.PR.Number, strings.ToLower(row.PR.State))
		switch row.State {
		case StateMerged:
			return stateMergedStyle.Render(label)
		case StateClosed:
			return stateClosedStyle.Render(label)
		case StateActive:
			return stateOpenStyle.Render(label)
		case StateDraft:
			return stateDraftStyle.Render(label + " (draft)")
		}
	}
	switch row.State {
	case StateMain:
		return stateMainStyle.Render("main")
	case StateNoPR:
		return stateNoPRStyle.Render("no PR")
	}
	return dimStyle.Render("—")
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// padRightVisible pads a string that may contain ANSI escape codes,
// using visible (cell) width instead of byte length.
func padRightVisible(s string, width int) string {
	vis := lipgloss.Width(s)
	if vis >= width {
		return s
	}
	return s + strings.Repeat(" ", width-vis)
}

// ColumnWidths calculates maximum branch and status widths for alignment.
func ColumnWidths(rows []WorktreeRow) (maxBranch, maxStatus int) {
	for _, r := range rows {
		b := len(r.Worktree.Branch)
		if b == 0 {
			b = len("(detached)")
		}
		if b > maxBranch {
			maxBranch = b
		}
		// Approximate status width
		s := statusWidth(r)
		if s > maxStatus {
			maxStatus = s
		}
	}
	return
}

func statusWidth(row WorktreeRow) int {
	if row.PR != nil {
		w := len(fmt.Sprintf("#%d %s", row.PR.Number, strings.ToLower(row.PR.State)))
		if row.State == StateDraft {
			w += len(" (draft)")
		}
		return w
	}
	switch row.State {
	case StateMain:
		return 4
	case StateNoPR:
		return 5
	}
	return 1
}
