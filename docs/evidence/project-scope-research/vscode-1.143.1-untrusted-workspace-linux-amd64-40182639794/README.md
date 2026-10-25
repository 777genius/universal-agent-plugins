# VS Code 1.143.1 untrusted workspace: blocked before metadata

Genuine hosted observation attempted on 2026-10-25 at source
`37fc30df2cb0b8499c6d32520e8b32d82d7fa9da`, pinned publisher commit
`e646868227c3e3a9e3f24277c8c7220be42c0be7`, in
[run 40182639794](https://github.com/777genius/universal-agent-plugins/actions/runs/40182639794),
[job 121136740348](https://github.com/777genius/universal-agent-plugins/actions/runs/40182639794/job/121136740348).
The inert contracts passed. The actual collector exited 2; **no workspace
component contract or product implementation is admitted by this artifact**.

Provider artifact `vscode-1.143.1-untrusted-workspace-linux-amd64`, ID
`12596073531`, was 5,185 bytes, SHA256
`21bad1e1d409fe33476cc2b9ec96d64aaafc38c0609d7262aec8617eb897d977`.
Its sole member is `research.zip`; all 20 inner text members are preserved
byte-for-byte here. `provenance.json` records both ZIP identities and every
raw member length/hash. The worker verified the outer digest, unique flat
non-symlink member closure, 14 separate command streams, two merged native
diagnostic streams and exact frontend/controller source agreement. Binary ZIPs
are not committed, and original failed summaries are unchanged.

## Concrete native blocker

`A.merged.log` records the publisher's Chromium fatal error: its SUID sandbox
helper is present but is not configured as a root-owned mode-4755 executable.
The publisher aborts rather than run without sandboxing. The approved collector
intentionally permits execution only of the ordinary nonprivileged editor and
ripgrep ELF files; it does not enable the privileged sandbox helper. The reviewed
plan explicitly forbids granting setuid privileges, disabling the sandbox or
changing kernel/AppArmor policy as a fallback. No such fallback was attempted.

The A verifier emitted real UID/EUID/GID/EGID 1001, empty groups, zero capability
sets, no-new-privileges and a network namespace distinct from its recorded
parent, with only loopback. Xvfb also emitted its own isolation proof. These
pre-exec facts do not prove successful workbench startup or successful teardown.
Both child records report `tree_reaped:false`, `planned_stop:false`, and failed
strict cleanup; display asset cleanup remains unproved. The controller's
`observations` is `null`: there is no A metadata result and the B/restart/removal
table was not completed. All qualification flags and collection-complete values
remain false.

## Separate cleanup defect

The root-owned sealed-tree remover returned successfully, but global display
assets were deliberately retained because their process-tree cleanup was not
proved. Separately, frontend cleanup of its own input tree failed with
`PermissionError` on `mkall.sh` under the Go build/module cache. That cache can
contain ordinary owner-controlled read-only directories; the current held-root
remover does not restore owner write permission before descending. A narrow
final-cleanup correction requires its own source review and inert regression;
it cannot repair or qualify the failed native workspace observation.

The artifact is negative native evidence. A newly reviewed safe execution
design and fresh positive component observations remain necessary before the
conditional VS Code adapter plan permits product code. Help/version success,
static schema support, VM disposal or local filesystem fixtures cannot replace
that gate. No native client was rerun while interpreting this artifact.
