package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/plinde/gwtui/internal/cli"
	"github.com/plinde/gwtui/internal/tui"
)

func main() {
	var repoPath string
	var noTUI bool

	rootCmd := &cobra.Command{
		Use:   "gwtui [path]",
		Short: "Git Worktree TUI Manager",
		Long:  "Interactive TUI for managing git worktrees with GitHub PR status enrichment.",
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				repoPath = args[0]
			}
			if repoPath == "" {
				p, err := gitRepoRoot()
				if err != nil {
					return fmt.Errorf("not in a git repository (use --repo or pass a path)")
				}
				repoPath = p
			}
			if noTUI {
				return cli.Print(repoPath)
			}
			return tui.Run(repoPath)
		},
	}

	rootCmd.Flags().StringVar(&repoPath, "repo", "", "path to git repository (default: current repo)")
	rootCmd.Flags().BoolVar(&noTUI, "no-tui", false, "print worktree status to stdout (non-interactive)")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func gitRepoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
