---
name: feature
description: End-to-end feature workflow — create issue, implement via worktree, PR, merge, release
aliases:
  - implement
  - enhancement
triggers:
  - feature
  - implement
  - enhancement
  - work on feature
  - implement feature
  - new feature
---

# Feature Workflow

End-to-end skill for planning and shipping features in gwtui. Operates in two modes depending on arguments.

## Mode 1: Plan (no issue number)

When invoked without a GitHub issue number (e.g., `/feature add vim keybindings`), create a new GitHub issue.

### Steps

1. Gather feature details from the user's prompt. Distill into:
   - A short title prefixed with `feat:` (e.g., `feat: add vim keybindings`)
   - A body with these sections:

```markdown
## Summary
<1-3 sentences describing the feature and motivation>

## Acceptance criteria
- [ ] <concrete, testable criterion>
- [ ] <tests covering the new behavior>
- [ ] All existing tests pass
```

2. Create the issue:

```bash
~/.claude/bin/gh issue create --repo plinde/gwtui --title "${TITLE}" --body "${BODY}"
```

3. Report the issue URL to the user. Ask if they want to proceed to implementation now.

## Mode 2: Implement (issue number provided)

When invoked with a GitHub issue number (e.g., `/feature #20`, `implement #20`, `work on /feature #20`), execute the full implementation lifecycle.

### Phase 1: Understand

1. Fetch the issue details:

```bash
~/.claude/bin/gh issue view ${NUMBER} --repo plinde/gwtui --json title,body
```

2. Read the issue title and body. Understand the acceptance criteria. Read relevant source files to plan the implementation. Identify which files need changes and what tests are needed.

### Phase 2: Worktree

3. Create a worktree branching from latest main:

```bash
git fetch origin main
DESCRIPTION=$(echo "${ISSUE_TITLE}" | sed 's/^feat: //' | tr ' ' '-' | tr '[:upper:]' '[:lower:]' | tr -cd 'a-z0-9-')
git worktree add ~/workspace/github.com/plinde/gwtui--${DESCRIPTION} -b issue-${NUMBER}/${DESCRIPTION} origin/main
```

The worktree path uses `gwtui--<description>` and the branch uses `issue-N/<description>`.

### Phase 3: Implement

4. Make all code changes **inside the worktree directory**. This is critical — never edit files in the main checkout.

5. Write thorough tests:
   - Unit tests for new functions
   - Integration tests for new behavior interacting with existing features
   - Edge cases and error paths
   - Update existing tests if behavior changed

6. Run the full test suite:

```bash
go test ./...
```

Fix any failures before proceeding. Do not skip this step.

7. If a `Makefile` was changed, lint it:

```bash
checkmake Makefile
```

### Phase 4: PR

8. Stage only the relevant files (no `git add .`):

```bash
git add <specific files>
```

9. Commit with a semantic message:

```bash
git commit -m "feat: <description>

<optional body explaining the change>

Closes #${NUMBER}

Co-Authored-By: Claude <noreply@anthropic.com>"
```

10. Push and create a PR:

```bash
git push -u origin issue-${NUMBER}/${DESCRIPTION}
~/.claude/bin/gh pr create --repo plinde/gwtui --title "feat: <description>" --body "${PR_BODY}"
```

PR body format:

```markdown
## Summary
<bullet points summarizing changes>

Closes #${NUMBER}

## Test plan
- [x] <what's tested>
- [ ] <manual verification steps>

🤖 Generated with [Claude Code](https://claude.ai/code)
```

### Phase 5: Merge

11. Squash merge the PR:

```bash
~/.claude/bin/gh pr merge ${PR_NUMBER} --squash --delete-branch --repo plinde/gwtui
```

12. Update local main and clean up the worktree:

```bash
git fetch origin main
git reset --hard origin/main
git worktree remove ~/workspace/github.com/plinde/gwtui--${DESCRIPTION}
```

### Phase 6: Release

13. Determine the next minor version (features bump minor):

```bash
git fetch --tags
LATEST=$(git describe --tags --abbrev=0)
```

Bump minor version: `v1.3.0` -> `v1.4.0`.

14. Generate changelog and create release:

```bash
~/.claude/bin/gh release create ${VERSION} --repo plinde/gwtui --title "${VERSION}" --notes "${NOTES}"
```

Notes format:

```markdown
## What's Changed

### Features
- **Feature title** — description. (#PR)

**Full Changelog**: https://github.com/plinde/gwtui/compare/${LATEST}...${VERSION}
```

15. Build and install the new binary:

```bash
go build -o ~/.local/bin/gwtui ./cmd/
```

16. Report final summary: issue URL, PR URL, release URL, installed version.
