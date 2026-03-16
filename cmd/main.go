package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/plinde/gwtui/internal/cli"
	"github.com/plinde/gwtui/internal/tui"
)

var version = "dev"

func main() {
	var repoPath string
	var noTUI bool

	rootCmd := &cobra.Command{
		Use:     "gwtui [path]",
		Short:   "Git Worktree TUI Manager",
		Long:    "Interactive TUI for managing git worktrees with GitHub PR status enrichment.",
		Version: version,
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
			if noTUI || !isatty.IsTerminal(os.Stdout.Fd()) {
				return cli.Print(repoPath)
			}
			jumpPath, err := tui.Run(repoPath)
			if err != nil {
				return err
			}
			if jumpPath != "" {
				fmt.Println(jumpPath)
			}
			return nil
		},
	}

	rootCmd.Flags().StringVar(&repoPath, "repo", "", "path to git repository (default: current repo)")
	rootCmd.Flags().BoolVar(&noTUI, "no-tui", false, "print worktree status to stdout (non-interactive)")

	initCmd := &cobra.Command{
		Use:       "init [shell]",
		Short:     "Generate shell integration (add to .zshrc: eval \"$(gwtui init zsh)\")",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"zsh", "bash"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return printShellInit(args[0])
		},
	}
	rootCmd.AddCommand(initCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func printShellInit(shell string) error {
	switch shell {
	case "zsh", "bash":
		fmt.Print(shellInitScript)
		return nil
	default:
		return fmt.Errorf("unsupported shell: %s (supported: zsh, bash)", shell)
	}
}

const shellInitScript = `# gwtui shell integration
gw() {
  local dir
  dir=$(command gwtui "$@")
  if [[ -n "$dir" && -d "$dir" ]]; then
    cd "$dir" || return 1
  fi
}
`

func gitRepoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
