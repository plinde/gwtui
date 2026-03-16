package tui

import (
	"cmp"
	"slices"
	"strings"
)

// SortColumn identifies which column to sort by.
type SortColumn int

const (
	SortNone   SortColumn = iota
	SortBranch            // alphabetical by branch name
	SortPRNum             // by PR number
	SortState             // by display state rank
)

// SortDirection indicates ascending or descending sort.
type SortDirection int

const (
	SortAsc  SortDirection = iota
	SortDesc
)

// stateRank maps DisplayState to a sort rank.
// Asc order: open(0) → draft(1) → no-pr(2) → merged(3) → closed(4).
// Main/bare rows are pinned and never sorted.
var stateRank = map[DisplayState]int{
	StateActive: 0,
	StateDraft:  1,
	StateNoPR:   2,
	StateMerged: 3,
	StateClosed: 4,
}

// sortRows partitions pinned (main/bare) rows to the top, then sorts
// the remaining rows by the given column and direction. Returns a new slice.
func sortRows(rows []WorktreeRow, col SortColumn, dir SortDirection) []WorktreeRow {
	if col == SortNone {
		return rows
	}

	pinned := make([]WorktreeRow, 0)
	sortable := make([]WorktreeRow, 0, len(rows))
	for _, r := range rows {
		if r.State == StateMain {
			pinned = append(pinned, r)
		} else {
			sortable = append(sortable, r)
		}
	}

	cmpFn := comparator(col)
	slices.SortStableFunc(sortable, func(a, b WorktreeRow) int {
		// For PR# sort, nil-PR rows always go to the end regardless of direction
		if col == SortPRNum {
			aHas := a.PR != nil
			bHas := b.PR != nil
			if !aHas && bHas {
				return 1
			}
			if aHas && !bHas {
				return -1
			}
			if !aHas && !bHas {
				return 0
			}
		}
		v := cmpFn(a, b)
		if dir == SortDesc {
			v = -v
		}
		return v
	})

	result := make([]WorktreeRow, 0, len(rows))
	result = append(result, pinned...)
	result = append(result, sortable...)
	return result
}

func comparator(col SortColumn) func(a, b WorktreeRow) int {
	switch col {
	case SortBranch:
		return cmpBranch
	case SortPRNum:
		return cmpPRNum
	case SortState:
		return cmpState
	default:
		return func(_, _ WorktreeRow) int { return 0 }
	}
}

func cmpBranch(a, b WorktreeRow) int {
	return cmp.Compare(strings.ToLower(a.Worktree.Branch), strings.ToLower(b.Worktree.Branch))
}

// cmpPRNum compares by PR number. Callers must handle nil-PR cases before calling.
func cmpPRNum(a, b WorktreeRow) int {
	return cmp.Compare(a.PR.Number, b.PR.Number)
}

func cmpState(a, b WorktreeRow) int {
	return cmp.Compare(stateRank[a.State], stateRank[b.State])
}

// nextSortColumn advances to the next column (wraps around).
func nextSortColumn(col SortColumn) SortColumn {
	switch col {
	case SortNone:
		return SortBranch
	case SortBranch:
		return SortPRNum
	case SortPRNum:
		return SortState
	case SortState:
		return SortNone
	}
	return SortNone
}

// prevSortColumn goes to the previous column (wraps around).
func prevSortColumn(col SortColumn) SortColumn {
	switch col {
	case SortNone:
		return SortState
	case SortState:
		return SortPRNum
	case SortPRNum:
		return SortBranch
	case SortBranch:
		return SortNone
	}
	return SortNone
}

// sortColumnLabel returns the display name for a sort column.
func sortColumnLabel(col SortColumn) string {
	switch col {
	case SortBranch:
		return "BRANCH"
	case SortPRNum:
		return "PR#"
	case SortState:
		return "STATE"
	}
	return ""
}

// renderHeader renders the column header line with a sort indicator on the active column.
// It mirrors the structural pattern of RenderRow so ANSI wrapping is identical,
// producing aligned columns in the terminal.
func renderHeader(col SortColumn, dir SortDirection, maxBranch, maxStatus int) string {
	indicator := ""
	if col != SortNone {
		if dir == SortAsc {
			indicator = " ▲"
		} else {
			indicator = " ▼"
		}
	}

	branchLabel := "BRANCH"
	statusLabel := "STATUS"
	pathLabel := "PATH"

	switch col {
	case SortBranch:
		branchLabel += indicator
	case SortPRNum, SortState:
		statusLabel += indicator
	}

	// Mirror RenderRow structure exactly:
	// cursor (2 vis) + checkbox (4 vis) + branch + sep + status + sep + path
	prefix := dimStyle.Render("  ") + dimStyle.Render("    ")
	branchCol := dimStyle.Render(padRight(branchLabel, maxBranch))
	statusCol := dimStyle.Render(padRight(statusLabel, maxStatus))
	pathCol := dimStyle.Render(pathLabel)
	sep := dimStyle.Render(" │ ")

	return prefix + branchCol + sep + statusCol + sep + pathCol
}
