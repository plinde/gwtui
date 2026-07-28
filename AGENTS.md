# gwtui project instructions

## Build and validation

Use the repository Makefile:

```bash
make test
make build
make install
```

`make install` builds and copies `gwtui` to `~/.local/bin/gwtui`, then restores
its ad-hoc signature on macOS so the copied binary is not killed on launch.

## Manual testing handoff

Before asking the user to test a code change:

1. Run the required automated tests for the change.
2. Run `make install` from the exact checkout or worktree containing the change.
3. Verify the installed binary starts with `~/.local/bin/gwtui --help`.
4. Tell the user which branch or commit was installed.

Do not ask the user to test a change that has not been installed. Do not make the
user run the install step.
