package main

import (
	"fmt"
	"os"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/plinde/gwtui/internal/cache"
	"github.com/plinde/gwtui/internal/cli"
	"github.com/plinde/gwtui/internal/tui"
)

var version = "dev"

func main() {
	var orgArg string
	var repoArg string
	var noTUI bool
	var cacheTTL time.Duration
	var noCache bool

	rootCmd := &cobra.Command{
		Use:          "gwtui [path]",
		Short:        "Git Worktree TUI Manager",
		Long:         "Interactive TUI for managing git worktrees with GitHub PR status enrichment.\n\nFrom a github.com/<org> root, gwtui loads direct child checkouts. From inside one of those repositories, it stays scoped to that repository while retaining org-aware data internally.",
		Version:      version,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			var positional string
			if len(args) > 0 {
				positional = args[0]
			}
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			scope, err := resolveLaunch(cwd, positional, orgArg, repoArg)
			if err != nil {
				return err
			}
			cacheOpts := cache.Options{TTL: cacheTTL, Disabled: noCache}
			if noTUI || !isatty.IsTerminal(os.Stdout.Fd()) {
				return cli.Print(scope.targetPath, scope.repository, cacheOpts)
			}
			jumpPath, err := tui.Run(scope.targetPath, scope.displayPath, scope.repository, cacheOpts)
			if err != nil {
				return err
			}
			if jumpPath != "" {
				fmt.Println(jumpPath)
			}
			return nil
		},
	}

	rootCmd.Flags().StringVar(&orgArg, "org", "", "GitHub org name or path to an org root")
	rootCmd.Flags().StringVar(&repoArg, "repo", "", "repository scope with --org; otherwise repository/org-root path (legacy)")
	rootCmd.Flags().BoolVar(&noTUI, "no-tui", false, "print worktree status to stdout (non-interactive)")
	rootCmd.Flags().DurationVar(&cacheTTL, "cache-ttl", cache.DefaultTTL, "reuse cached gh PR data younger than this (avoids GitHub rate limits); 0 forces a live call but still writes through")
	rootCmd.Flags().BoolVar(&noCache, "no-cache", false, "bypass the PR response cache entirely (no read, no write, no stale fallback)")

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
