# gwtui — Specification

Git Worktree TUI Manager with GitHub PR status enrichment.

## Overview

`gwtui` is an interactive terminal UI for managing git worktrees. It fetches GitHub PR state for each worktree's branch, determines cleanability, and allows batch removal of stale worktrees (directory + branch).

## CLI Interface

```
gwtui [path] [--repo <path>]
```

### Path Resolution Priority

1. **Positional argument** — `gwtui ~/workspace/github.com/org/repo`
2. **`--repo` flag** — `gwtui --repo ~/workspace/github.com/org/repo`
3. **Current directory** — runs `git rev-parse --show-toplevel` to find repo root

If none resolve to a valid git repository, exits with error:
`not in a git repository (use --repo or pass a path)`

### Implementation

- Cobra with `MaximumNArgs(1)` — accepts zero or one positional args
- `SilenceUsage: true` — suppresses usage on runtime errors

## TUI Phases

```
Load → List → Confirm → Cleanup → Done
                                    ↕
                               Help (overlay)
```

| Phase | Description |
|-------|-------------|
| **Load** | Fetches worktrees and PR data concurrently. Shows spinner. |
| **List** | Main view. Browse worktrees, toggle selection, view status. |
| **Confirm** | Review selected worktrees before destructive cleanup. |
| **Cleanup** | Executes removal. Shows spinner. |
| **Done** | Summary of results (successes/failures). |
| **Help** | Overlay accessible from List phase. Returns to previous phase. |

### Concurrent Loading

Worktree list (`git`) and PR data (`gh`) are fetched in parallel via goroutines. PR errors are **non-fatal** — the TUI proceeds with empty PR data.

## Keybindings

### Navigation

| Key | Action |
|-----|--------|
| `↑` / `k` | Move cursor up |
| `↓` / `j` | Move cursor down |

### Selection (List phase)

| Key | Action |
|-----|--------|
| `space` | Toggle selection (cleanable rows only) |
| `a` | Select all cleanable worktrees |
| `n` | Deselect all |

### Actions

| Key | Action | Phase |
|-----|--------|-------|
| `tab` | Proceed to cleanup confirmation | List |
| `enter` | Confirm cleanup / quit | Confirm, Done |
| `backspace` / `delete` / `ctrl+h` | Go back | Confirm, Help |
| `?` | Toggle help overlay | List, Help |
| `q` / `ctrl+c` | Quit | All |

## Display States

| State | Style | Cleanable | Description |
|-------|-------|-----------|-------------|
| `open:ready` | Cyan | No | PR is open and ready for review |
| `open:draft` | Dim gray | No | PR is open but in draft |
| `merged` | Green | Yes | PR has been merged |
| `closed` | Red | Yes | PR has been closed |
| `no-pr` | Yellow | Yes | No associated PR found |
| `main` | Bright blue, bold | No | Main worktree or default branch |

### Cleanability Rules

- **Protected (never cleanable):** main worktree, bare repo entries, open PRs, draft PRs
- **Cleanable:** merged PRs, closed PRs, branches with no PR
- Main worktree is identified by: matching the first `worktree` entry path from porcelain output, OR matching the default branch name

### Status Column Format

- With PR: `#<number> <state>` (e.g., `#42 merged`, `#15 open (draft)`)
- Main worktree: `main`
- No PR: `no PR`

## Architecture

```
gwtui/
├── cmd/
│   └── main.go              # CLI entry point (Cobra)
├── internal/
│   ├── git/
│   │   ├── worktree.go      # Worktree listing, porcelain parsing
│   │   └── cleanup.go       # Worktree removal, branch deletion
│   ├── github/
│   │   └── pr.go            # PR fetching via gh CLI
│   └── tui/
│       ├── model.go         # BubbleTea model, phases, Update/View
│       ├── list.go          # WorktreeRow, EnrichWorktrees, RenderRow
│       ├── keymap.go        # Key bindings definition
│       ├── messages.go      # Tea commands and messages
│       └── styles.go        # Lipgloss style definitions
├── go.mod
├── go.sum
└── Makefile
```

### Package Responsibilities

| Package | Responsibility |
|---------|----------------|
| `cmd` | CLI argument parsing, repo path resolution |
| `internal/git` | Git worktree operations (list, remove, branch delete) |
| `internal/github` | GitHub PR data fetching via `gh` CLI |
| `internal/tui` | BubbleTea TUI: model, view, update, styling |

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/charmbracelet/bubbletea` | TUI framework (Elm architecture) |
| `github.com/charmbracelet/lipgloss` | Terminal styling |
| `github.com/charmbracelet/bubbles` | Reusable TUI components (spinner, key bindings) |
| `github.com/spf13/cobra` | CLI argument parsing |

### External Tools

| Tool | Usage |
|------|-------|
| `git` | `git worktree list --porcelain`, `git worktree remove --force`, `git branch -D`, `git symbolic-ref` |
| `gh` | `gh pr list --state all --limit 200 --json number,title,state,isDraft,headRefName` |

## Git Operations

### Worktree Listing

```
git worktree list --porcelain
```

Parsed fields: `worktree <path>`, `HEAD <sha>`, `branch refs/heads/<name>`, `bare`, `detached`

- SHA is truncated to 8 characters
- `refs/heads/` prefix is stripped from branch names
- Detached HEAD leaves branch empty

### Default Branch Detection

```
git symbolic-ref refs/remotes/origin/HEAD
```

Falls back to `"main"` if the command fails (e.g., no remote configured).

### Cleanup Operations

For each selected worktree:
1. `git worktree remove --force <path>` — removes worktree directory
2. `git branch -D <branch>` — force-deletes the branch (skipped if branch is empty)

Cleanup runs sequentially (not parallel) to avoid git lock contention.

## GitHub Integration

```
gh pr list --state all --limit 200 --json number,title,state,isDraft,headRefName
```

- Returns up to **200** PRs (open + closed + merged)
- Indexed into `map[string]*PR` keyed by `headRefName` for O(1) branch lookup
- PR states from GitHub: `"OPEN"`, `"CLOSED"`, `"MERGED"`

## UI Features

### Scrolling

List view implements viewport scrolling when rows exceed terminal height:
- Cursor-centered scrolling window
- `↑ more above` / `↓ more below` indicators
- Available height = terminal height - header (4 lines) - footer (3 lines)

### Path Compression

Paths are compressed for display:
- `$HOME` → `~`
- Paths with >4 segments: `~/workspace/github.com/org/repo--branch` → `~/...repo--branch`

### Status Bar

```
N selected / M cleanable / T total
```

Rendered with background color for visual distinction.

### Alt Screen

TUI runs in alt screen mode (`tea.WithAltScreen()`) — restores terminal on exit.

## Edge Cases

| Case | Behavior |
|------|----------|
| Detached HEAD | Branch shows as `(detached)`, state depends on PR lookup (likely `no-pr`) |
| Bare repository entry | Treated as main/protected, not cleanable |
| No remote configured | Default branch detection falls back to `"main"` |
| `gh` CLI not available | PR fetch fails silently, all non-main worktrees show `no-pr` |
| PR limit (200) | Only the 200 most recent PRs are fetched; older branches may show `no-pr` |
| Force removal | `--force` flag handles dirty worktrees (uncommitted changes are lost) |
| Empty branch name | Branch deletion step is skipped during cleanup |
| No cleanable worktrees | Tab key has no effect (confirm phase requires >0 selections) |
| Load error | Jumps directly to Done phase with error message |

## Build

```bash
make build    # → bin/gwtui
make install  # → ~/bin/gwtui
make clean    # removes bin/
```
