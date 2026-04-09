---
name: release
description: Cut a new GitHub release for gwtui with auto-generated changelog
triggers:
  - release
  - cut a release
  - new release
  - ship it
---

# Release gwtui

Creates a new GitHub release with a changelog derived from commits since the last tag.

## Arguments

- Version bump level: `patch` (default), `minor`, or `major`
- Can also accept an explicit version like `v1.4.0`

## Steps

1. Ensure on latest main and all tests pass:

```bash
git fetch origin main --tags
go test ./...
```

Stop if tests fail.

2. Determine the next version:

```bash
LATEST=$(git describe --tags --abbrev=0)
```

If an explicit version was provided as an argument, use that. Otherwise, bump the patch version (e.g., `v1.3.0` -> `v1.3.1`). For `minor`: `v1.3.0` -> `v1.4.0`. For `major`: `v1.3.0` -> `v2.0.0`.

3. Generate changelog from commits since last tag:

```bash
git log ${LATEST}..HEAD --oneline
```

Group commits by type prefix (`feat:`, `fix:`, `chore:`, etc.) into sections:
- **Features** for `feat:`
- **Bug Fixes** for `fix:`
- **Other** for everything else

4. Create the release using `~/.claude/bin/gh`:

```bash
~/.claude/bin/gh release create ${VERSION} --repo plinde/gwtui --title "${VERSION}" --notes "${NOTES}"
```

The notes should follow this format:

```
## What's Changed

### Features
- **Description** — details. (#PR)

### Bug Fixes
- **Description** — details. (#PR)

**Full Changelog**: https://github.com/plinde/gwtui/compare/${LATEST}...${VERSION}
```

5. After release, run the `install` skill to update the local binary.
