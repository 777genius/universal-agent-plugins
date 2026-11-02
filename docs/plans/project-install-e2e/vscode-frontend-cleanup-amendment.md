# VS Code frontend cleanup: final ownership-only amendment

2026-10-25. The genuine failed workspace run `40182639794` separately found
frontend cleanup could not unlink `mkall.sh` from its own Go module cache's
read-only directories. This amendment changes only final cleanup of the exact
frontend-owned input tree. It does not close the failed workspace observation
gate, change native arguments, enable a sandbox helper or authorize a retry.

Add a workspace-only fixed `cleanup_frontend` command using the existing bounded
`run_probe`: `/usr/bin/python3 -I`, trusted cwd `/`, finite environment, live
output caps and five-second deadline. Keep the inherited help/version cleanup
implementation unchanged. Root path/expected device/inode/owner are captured by
the trusted frontend before acquisition, never read from a native child result.
Accept only the exact ordinary `/tmp/vscode-workspace-input-*` root. Open `/tmp`
and its root leaf with directory/no-follow descriptors; require the opened root
to match captured device/inode/original UID and the current nonroot cleanup UID.

At final cleanup only, each opened ordinary same-owner/same-device directory
must match its no-follow named identity before `fchmod(fd,(mode&0777)|0700)`.
This strips privileged directory bits and restores owner permissions needed to
remove read-only Go cache entries. Never chmod leaves, symlinks or foreign-owned
directories. Leaf removal is no-follow; outside symlink targets retain modes and
bytes. Refuse foreign owners, different-device directory trees and changed
identities. Recheck each named child and root device/inode/owner immediately
before removal so replacement is not accepted. Report cleanup failure separately
and preserve the original collection/probe error and false qualification flags.

Inert actual-child regressions run under an ordinary UID, using a fixed
`setpriv` drop into an exclusively created/chowned fixture only when the test
executor is root. Prove removal of a 0555 module directory and 0444 `mkall.sh`,
preservation of outside symlink target bytes/modes, and refusal of replaced roots
and foreign-owned directories. No vendor executable, hosted-guard spoof or
native sandbox change is involved.

ROOT and CI/SDK independently gave R1 concept GO. This final revision incorporates
their trusted-frontend identity origin, opened-directory ownership/identity,
privileged-bit stripping and original-failure precedence clarifications. Final
R2 plan and frozen code reviews are recorded separately.
