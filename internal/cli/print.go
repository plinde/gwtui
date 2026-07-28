package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/plinde/gwtui/internal/tui"
)

// Print loads worktree and PR data, then writes a plain-text table to stdout.
// Errors are printed to stderr.
func Print(repoPath, filter string) error {
	rows, warnings, err := tui.LoadRows(repoPath)
	if err != nil {
		return err
	}
	for _, warning := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s: could not fetch PR data: %v\n", warning.Repo.Name, warning.Err)
	}
	rows = tui.FilterRows(rows, filter)
	maxBranch, maxStatus := tui.ColumnWidths(rows)

	header := fmt.Sprintf("%-*s  %-*s  %s", maxBranch, "BRANCH", maxStatus, "STATUS", "PATH")
	fmt.Println(header)

	for _, row := range rows {
		branch := tui.BranchLabel(row)
		status := plainStatus(row)
		path := tui.CompressPath(row.Worktree.Path)

		fmt.Printf("%-*s  %-*s  %s\n", maxBranch, branch, maxStatus, status, path)
	}

	return nil
}

func plainStatus(row tui.WorktreeRow) string {
	if row.PR != nil {
		label := fmt.Sprintf("#%d %s", row.PR.Number, strings.ToLower(row.PR.State))
		if row.State == tui.StateDraft {
			label += " (draft)"
		}
		return label
	}
	switch row.State {
	case tui.StateMain:
		return "main"
	case tui.StateNoPR:
		return "no PR"
	}
	return "-"
}
