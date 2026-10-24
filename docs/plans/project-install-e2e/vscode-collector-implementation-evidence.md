# VS Code publisher collector: fresh implementation checkpoint

Implements the twice-reviewed restored collector plan in this branch. This is
help/version research only; no workspace adapter or production client behavior
is added. The existing Cline finite command/environment contracts are unchanged.

The collector calls the real unchanged hosted guard before output, acquisition
or native execution. It streams the exact pinned publisher archive to disk,
verifies full size/SHA256 before parsing, reads fixed TAR headers before bounded
metadata, refuses sparse/hardlink/special/duplicate/case-colliding paths, omits
all symlinks and materializes ordinary files descriptor-relative with no-follow
and exclusive leaf creation. Gzip CRC/end and zero TAR trailing data must close
before either finite native command is scheduled.

Only the shipped ELF plus shipped cli.js receives --version or --help. Each
process uses a fresh profile/cwd and the reviewed sudo/unshare/setpriv/env-i
chain. The same process verifies network namespace, original nonroot uid/gid,
empty groups, zero capabilities, no-new-privileges and loopback-only interfaces
before execve. File hashes/identities are checked before and after each probe.
Cleanup is a separate bounded inert Python child, anchored to the held task-root
identity and using no-follow descriptor-relative traversal. Failed cleanup keeps
collection incomplete. Flat research artifacts have an actual32MiB aggregate cap.

The shared live-output helper and its six inert tests are byte-for-byte the
already reviewed versions from origin/feat/project-qoder-e2e. Its independent
stdout/stderr caps do not limit unrelated native cache files. No shared helper
semantics were changed here.

## Actual local checks

- Dedicated VS Code synthetic archive/inert child tests: 11 PASS, 0.203s.
- All affected *_native_research suites: 34 PASS, 1.319s.
- Reused shared live-capture suite: 6 PASS, 2.197s.
- Native safety suites including that capture suite: 63 PASS, 2.861s.
- Python syntax and git diff --check passed.

No publisher/vendor/native executable, archive acquisition, trust/account/model,
extension host, MCP process or guard spoofing occurred. Tests used synthetic
archives and inert Python children. Two fresh independent implementation reviews
are required before publishing this source to trigger the hosted native workflow.
Successful help/version capture will still require independent interpretation
and a new reviewed workspace-evidence/adapter plan. It never sets
native_project_qualified true.
