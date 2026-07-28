---
name: install
description: Build gwtui from source and install the binary to ~/.local/bin
triggers:
  - install
  - build and install
  - update binary
---

# Install gwtui

Builds the gwtui binary from source and installs it.

For a feature that affects the built binary, installation is a required local
validation step before PR/release delivery, not only when manual testing has
already been requested. Run these steps from the feature worktree containing
the exact change to test. In that case, being off `main` is expected; identify
the installed branch or commit in the handoff. Documentation-only changes that
cannot affect the binary are exempt.

## Steps

1. Ensure on latest main:

```bash
git fetch origin main
```

For normal installs, warn the user if the working tree has uncommitted changes
or if HEAD is not on main. For an explicitly requested feature-test install,
allow the feature worktree after confirming its identity and intended diff.

2. Run tests:

```bash
go test ./...
```

If tests fail, stop and report the failure. Do not install a broken build.

3. Build and install through the project target:

```bash
make install
```

4. Verify the installed binary runs:

```bash
~/.local/bin/gwtui --help
```

Report success with the installed path.
