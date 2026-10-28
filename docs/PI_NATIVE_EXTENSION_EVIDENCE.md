# Pi implementation milestone evidence

This records the first provider/SDK milestone. It is not the final three-OS
desktop, sound or terminal-focus qualification for Agent Notifications.

## Implemented boundary

- Concrete `clients/pi` adapter with a strict, read-only namespace interpreter.
- Direct visible `extensions/<physicalArtifactID>` target and staging outside
  native discovery. Existing managed-package ownership and directory recovery
  remain the only mutation engine.
- Frozen physical-path bootstrap guard before selected source import, immutable
  package/data descriptor, read-only reconstruction and foreign-root refusal.
- Explicit owned-data, directory-verification and edited-package repair policy.
  The repair policy is necessary because the existing common repair engine
  otherwise intentionally restores a modified managed package. Only Pi opts in.
- Runtime-only typed SDK process carrier, strict bounded event union, generated
  registrar/registry/support entries and neutral context-aware callback.

## Verification completed

Toolchain: Go 1.27.2. Cross-repository local work uses only the explicit temporary
`/tmp/pi-integration.go.work`; no module replacement or workfile is committed.

From `install/integrationctl/agentplugins`:

```sh
go test ./domain ./planner ./clients/all ./clients/contracttest ./clients/pi -count=1
go test ./installer -run TestPiPublicDirectoryLifecycle -count=1
go build -o /tmp/pi-uap-driver ./clients/pi/testdata/driver
```

These pass. The facade test covers install, owned data allocation, direct native
target, update, missing repair, edited refusal and CLI-free removal, preserving
a foreign extension. Namespace/projection tests cover schema/path rejection,
reserved discovery overrides, bound reconstruction and edited bootstrap refusal.

From `sdk`, the complete suite passes with `GOWORK=off` and
`GOFLAGS=-buildvcs=false` outside the restrictive IPC sandbox. The fixture module
builds require their own module boundary; disabling VCS stamping avoids the
container's isolated fixture Git-status restriction. The sandbox itself denies
some existing socket/process descriptor tests. No existing observer fixtures or
implementation were changed to obtain this result.

Descriptor generation and CLI client-capability generation completed. Planner
goldens were regenerated; a structural comparison against the prior revision
confirms that removing the new Pi entries reproduces all prior clients exactly.
`git diff --check` passes.

The independent native E2E worker ran the public Engine driver with the official
SHA-verified Pi v1.4.0 Linux standalone. The actual visible leaf
`test-pi-native-loader-e30759e1053c` was discovered without `-e`. Selected source
top-level/factory/command each ran once. Visible retained copies, explicit hidden
copies and misbound/symlink paths did not activate source. Update, missing repair,
removal and foreign preservation passed. Its reproducible fixture is maintained
in the Agent Notifications change as `scripts/pi-uap-loader-e2e.py`.

## Remaining qualification

Independent code review and upstream CI are required before merge. Native macOS
and Windows Pi execution, release-equivalent Agent Notifications packaging,
receipt-backed product authority, old-running-observer revocation and actual
three-OS desktop/audio/click-to-focus evidence belong to the following stages.

A broad usecase/installer run under the temporary cross-repository environment
also exposed Codex environment-validation failures in existing sibling tests.
These are not marked passed; rerun the required suites under the normal CI
environment and classify any remaining failures before merge. Do not change
unrelated Claude/Codex or frozen VS Code behavior to hide an environment issue.
