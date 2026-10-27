# Qoder original-profile refusal: bounded metadata diagnostic

## Observed boundary and exact current source

Published PR523 head `0d1bfa7225937f4f581958022ef103200ee14301`, tree
`74242f382c0c738b4a13a95547b2d8102b461566`, has genuine runtime run
`40408766184`. Native artifact `12699200113` and matching frontend
`12698695352` retain successful public project installation followed by
`Qoder profile ordinary single-link file required`. The failure happens during
the first complete original `HOME/.qoder` inventory, before the selected native
catalog or startup command. The closed capture does not identify the offending
entry, file type, link count or size; no particular cause is established.
All five native qualification flags remain false.

Local baseline `4106c6f9a942f3fadd26b29e109f4ceb4dbe68a5` has exactly that
published tree. It preserves the original worker branch and mirrors only the
published runtime-workflow overlay, immutable blob
`66fb12161a3d1cfe6992cfeada8fbf44ad54fda6`. No other baseline bytes changed.

## Narrow diagnostic change

After two independent plan GOs, change only the existing ordinary/single-link
refusal in `projectNativeCLIQoderProfile` to retain its exact reason prefix and
append four metadata fields from the already obtained `os.Lstat` result:
the relative entry, `info.Mode()` as a finite mode string, `identity.Nlink`,
and `info.Size()`. Keep the current refusal predicate and its position before
the existing byte accounting or bounded body reader unchanged.

Use a small private formatter for that error. The relative entry is limited to
256 input bytes with an explicit truncation marker, and `%q` quotes/escapes it
so names containing controls cannot produce extra log lines. The complete
diagnostic is finite (at most 2 KiB), contains no absolute HOME/profile root,
and includes no file contents, link target, credentials or directory dump.
Do not follow the entry, call Readlink, reopen it or add filesystem reads.
Keep all original ancestor/owner, directory, count/depth, byte, ordinary-file,
single-link and stable bounded-read checks and their order. Do not admit,
remove, move, relabel or repair the refused entry. The error still terminates
the native fixture before catalog/startup; artifact closure and qualification
flags remain unchanged.

## Meaningful inert contracts

Exercise the actual profile inventory with real temporary ordinary parent
directories and separate refused symlink and hardlink entries. Compare the
diagnostic metadata against the actual entry's `os.Lstat` mode, size and link
count; ensure the relative entry is reported and the absolute root, foreign
target path and sentinel target contents are absent. Verify both entries and
their outside target remain unchanged after refusal. An actual nested symlink
whose relative name exceeds 256 bytes and contains quoted/control characters
checks truncation, the finite diagnostic bound and absence of raw line breaks.
No native binary, installer, model, network or subprocess runs.

Preserve the existing full-profile mutation contracts (bytes, modes, new empty
directory, new cache, alias and hardlink), opaque strict whole-JSON catalog
contract and finite command/argument/client/cwd/deadline controls. Add no skip,
fake filesystem metadata, synthetic native response or acceptance relaxation.

## Scope, checks and publication

Scope is exactly the two existing Linux Go test files
`repotests/agentplugins_project_native_cli_attempt_test.go` and
`repotests/agentplugins_project_native_cli_contract_test.go`, plus this plan and
validation record. No production/client adapter, installer, source/pins, user or
project authority, profile baseline, permissions/trust/authentication, original
deadline, process supervision, cleanup or qualification policy changes.
Useful legacy source remains preserved. VS Code is excluded by the owner's
latest instruction and is not part of this work.

After the shared exclusive Go/lint slot is granted, run the affected existing
and new inert Qoder inventory/catalog/command contracts, then actual affected
all-enabled changed-line lint and the whole-file size/architecture gate. Run
gofmt/whitespace checks and obtain independent code review before root-owned
publication to existing PR523; no new PR or release. Fresh genuine hosted raw
evidence must identify the refused entry before a separate behavioral plan can
be considered. Passing inert diagnostics does not qualify native operation.
