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

## Steps

1. Ensure on latest main:

```bash
git fetch origin main
```

Warn the user if the working tree has uncommitted changes or if HEAD is not on main.

2. Run tests:

```bash
go test ./...
```

If tests fail, stop and report the failure. Do not install a broken build.

3. Build and install:

```bash
go build -o ~/.local/bin/gwtui ./cmd/
```

4. Verify the installed binary runs:

```bash
gwtui -h
```

Report success with the installed path.
