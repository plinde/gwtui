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
Load → List → Confirm ─────────────────────→ Cleanup → Done
                 └─ selected open PR → Open-PR Confirm ┘
                                                        ↕
                                                   Help (overlay)
```

| Phase | Description |
|-------|-------------|
| **Load** | Fetches worktrees and PR data concurrently. Shows spinner. |
| **List** | Main view. Browse worktrees, toggle selection, view status. |
| **Confirm** | Review selected worktrees before destructive cleanup. |
| **Open-PR Confirm** | Required second warning when any selected worktree has an open or draft PR. |
| **Cleanup** | Executes removal. Shows spinner. |
| **Done** | Summary of results (successes/failures). |
| **Help** | Overlay accessible from List phase. Returns to previous phase. |

### Concurrent Loading

Worktree list (`git`) and PR data (`gh`) are fetched in parallel via goroutines. PR errors are **non-fatal** — the TUI proceeds with empty PR data.

## Launch Scope and Filtering

gwtui distinguishes immutable launch scope from an editable user filter:

- Launching at `.../github.com/<org>` loads all direct repository checkouts and
  displays the org-root path.
- Launching inside a direct child repository or one of its linked worktrees
  loads org data internally but scopes visible rows to the owning repository.
- `--org <org-or-root> --repo <name>` establishes the same repository scope
  explicitly.
- Repository-scoped mode displays the repository path and a separate
  `scope: repo:<name>` indicator. Scope is never stored in filter input.
- `/` edits a user filter within the current scope. `enter` or `tab` applies it.
- `esc` clears a user filter but never removes repository launch scope or
  reveals sibling repositories. With no user filter, `esc` is a no-op.
- Org-wide mode remains available by launching at the org root or with `--org`.

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
| `ctrl+a` | Toggle all visible merged/closed worktrees; preserve every other selection |
| `a` | Select all cleanable worktrees |
| `n` | Deselect all |

### Actions

| Key | Action | Phase |
|-----|--------|-------|
| `tab` | Proceed to cleanup confirmation | List |
| `enter` | Confirm cleanup / confirm open-PR cleanup / quit | Confirm, Open-PR Confirm, Done |
| `/` | Edit user filter within launch scope | List |
| `esc` | Clear user filter; retain repository scope | List, Filter |
| `backspace` / `delete` / `ctrl+h` | Go back | Confirm, Open-PR Confirm, Help |
| `?` | Toggle help overlay | List, Help |
| `q` / `ctrl+c` | Quit | All |

## Display States

| State | Style | Cleanable | Description |
|-------|-------|-----------|-------------|
| `open:ready` | Cyan | No | PR is open and ready for review |
| `open:draft` | Dim gray | No | PR is open but in draft |
| `merged` | Green | Yes | PR has been merged |
| `closed` | Red | Yes | PR has been closed |
| `no-pr` | Yellow | No | No associated PR found; protected from cleanup |
| `main` | Bright blue, bold | No | Main worktree or default branch |

### Cleanability Rules

- **Protected (never cleanable):** main worktree, bare repo entries, open PRs, draft PRs
- **Cleanable:** merged PRs and closed PRs
- Main worktree is identified by: matching the first `worktree` entry path from porcelain output, OR matching the default branch name

Ctrl+A deliberately uses the current display state rather than the broader
cleanability flag: only visible `merged` and `closed` rows participate in its
toggle. Selections on every other visible or hidden row are unchanged.

### Cleanup Confirmation Safety

- Every selected cleanup passes through the normal confirmation screen.
- If any selected row has `PR.State == OPEN`, Enter transitions to a dedicated
  open-PR warning instead of starting cleanup. Draft PRs are included.
- The warning lists all affected open PR branches, paths, PR numbers, and draft
  state. A second Enter starts cleanup; Backspace returns to the first
  confirmation without changing selection.
- The open-PR check is independent of cleanability so it also protects stale
  selection retained across refresh/filter state.

### Cursor Highlight

The active row keeps the leading cursor glyph and adds a background-color 237
band across the usable terminal width after the two-space list indent. The
highlight reopens after inner ANSI resets so checkbox, branch, state, separator,
and path foreground colors remain visible, then ends with a reset to prevent
style bleed. Non-cursor rows are neither highlighted nor width-padded.

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
