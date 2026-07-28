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

## Installed-binary validation

For every code change that affects the built binary, local feature validation is
not complete until the installed binary has been exercised. After automated
tests pass, and before entering PR/release delivery or asking the user to test:

1. Run the required automated tests for the change.
2. Run `make install` from the exact checkout or worktree containing the change.
3. Verify the installed binary starts with `~/.local/bin/gwtui --help`.
4. Tell the user which branch or commit was installed.

Do not deliver or ask the user to test a code change that has not passed this
installed-binary check. Do not make the user run the install step.
Documentation-only changes that cannot affect the binary are exempt.
