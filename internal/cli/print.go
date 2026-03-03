package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/plinde/gwtui/internal/git"
	gh "github.com/plinde/gwtui/internal/github"
	"github.com/plinde/gwtui/internal/tui"
)

// Print loads worktree and PR data, then writes a plain-text table to stdout.
// Errors are printed to stderr.
func Print(repoPath string) error {
	type wtResult struct {
		wts []git.Worktree
		err error
	}
	type prResult struct {
		prs map[string]*gh.PR
		err error
	}

	wtCh := make(chan wtResult, 1)
	prCh := make(chan prResult, 1)

	go func() {
		wts, err := git.List(repoPath)
		wtCh <- wtResult{wts, err}
	}()
	go func() {
		prs, err := gh.PRsByBranch(repoPath)
		prCh <- prResult{prs, err}
	}()

	wt := <-wtCh
	pr := <-prCh

	if wt.err != nil {
		return fmt.Errorf("failed to list worktrees: %w", wt.err)
	}
	if pr.err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not fetch PR data: %v\n", pr.err)
	}

	prs := pr.prs
	if prs == nil {
		prs = make(map[string]*gh.PR)
	}

	rows := tui.EnrichWorktrees(wt.wts, prs)
	maxBranch, maxStatus := tui.ColumnWidths(rows)

	header := fmt.Sprintf("%-*s  %-*s  %s", maxBranch, "BRANCH", maxStatus, "STATUS", "PATH")
	fmt.Println(header)

	for _, row := range rows {
		branch := row.Worktree.Branch
		if branch == "" {
			branch = "(detached)"
		}
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
