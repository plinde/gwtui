package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/plinde/gwtui/internal/tui"
)

// Print loads worktree and PR data, scopes it to repository when provided,
// then writes a plain-text table to stdout.
// Errors are printed to stderr.
func Print(repoPath, repository string) error {
	rows, warnings, err := tui.LoadRows(repoPath)
	if err != nil {
		return err
	}
	for _, warning := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s: could not fetch PR data: %v\n", warning.Repo.Name, warning.Err)
	}
	if repository != "" {
		scoped := make([]tui.WorktreeRow, 0, len(rows))
		for _, row := range rows {
			if strings.EqualFold(row.Worktree.RepoName, repository) {
				scoped = append(scoped, row)
			}
		}
		rows = scoped
	}
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
