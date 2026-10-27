# Qoder exact original log link: preserve metadata without following aliases

## Observed current refusal

Published PR523 head `c14b4974ddd42d16233ad654f3dbc50bcbcabcf0`, tree
`9197585febf142cc586712c04590cd06a967c17a`, has genuine runtime run
`40455028917`, native job `122026305662`. Public add-a succeeds (exit0,
error=false/deadline=false, 569 ms), then the first original-profile inventory
refuses before native catalog/startup with:

`Qoder profile ordinary single-link file required: entry="logs/latest" mode=Lrwxrwxrwx nlink=1 size=42`

The owner check preceding that refusal passed. This proves an original-user-owned
symlink at this exact relative entry. It does not capture the target bytes,
target kind/ownership or containment, nor establish which earlier command
created it. No source or runtime assumption may fill those gaps.

Native artifact `12711428594` outer SHA256
`eee6372ce39902fe9269f652632792bb5ab24cb16e8f1cf63e30ac50ac9681a6`
and frontend `12711149158` outer SHA256
`9ac977e326a940f96a3e1fa96e4e67edd91438aa9aa47750e7aca0c43cd033bb`
were checked as 13/25 bounded canonical unique ordinary flat ZIP members with
valid CRCs, exact source/tree, empty source-clean streams and matching frontend
command stream hashes. All qualifications remain false.

Local `51accde02931ff08158f47b94c2a195b5881fdbe` has exactly the current
published tree. Its accepted bounded metadata diagnostic remains preserved.
The preceding worker branch is retained; this slice has its own branch/plan.

## Decision and retained guarantee

A symlink can be inventoried without granting it filesystem authority if its
own held identity and exact link bytes are preserved, it is never followed,
and its lexical target and every target ancestor are confined to ordinary owned
entries already covered by the same complete profile inventory. Implement this
only for the observed exact `logs/latest` entry. All other symlinks and every
hardlink remain refused. Do not exclude logs, caches, new directories, changed
modes, bytes or link targets from the complete pre/post catalog comparison.

This does not assume the catalog is read-only in practice. If it rotates the
link, writes a log or otherwise changes the frozen profile, the existing fatal
comparison must still fail; no normal-logging exception is authorized here.

## Finite implementation after two independent plan GOs

Add one private Linux helper for that exact entry. Keep the current root/parent,
owner, entry/depth and ordinary-file checks and their order; call the helper
only after the existing Lstat/owner check and only when rel is exactly
`logs/latest` and the leaf is a symlink. Require its observed symlink mode,
original UID/GID, nlink1 and a positive size at most4096. Include link bytes in
the existing256MiB inventory budget; no new unbounded read is introduced.

Hold the ordinary original profile and logs directories with O_DIRECTORY,
O_NOFOLLOW/O_CLOEXEC descriptors and compare held/linked inode identity. Read
only `latest` with bounded Readlinkat on that held logs descriptor; do not use
EvalSymlinks, open or read through the symlink. Compare the leaf's held
AT_SYMLINK_NOFOLLOW stat before/after the read with the already observed Lstat,
and recheck both linked ancestor identities. Require exact target byte length
and stable inode/device/mode/owner/link-count/size/timestamps. Close every held
descriptor with checked errors. A race, alias or close failure refuses.

Interpret those bounded bytes only as a canonical nonempty relative path:
valid UTF8, no NUL/control, absolute/volume spelling, backslash or parent escape;
clean path equality and bounded component depth are mandatory. Resolve only
lexically beneath original `.qoder/logs`, then inspect each target component
relative to held descriptors without following symlinks. All target ancestors
must be ordinary original-user-owned directories; final target must be an
ordinary owned directory or single-link regular file. Self-links, dangling
links, absolute/outside targets, target/ancestor aliases, nonregular types and
multiply linked files refuse. Do not open or read target contents in this helper.

The ordinary target and complete descendant closure must also appear in the
unchanged normal profile walk; validate that requirement before admitting the
inventory. Store only an unambiguous symlink record containing its mode/link
metadata, target byte count and SHA256 of exact target bytes. The target text
stays private. Exact before/after map equality freezes the link along with every
other entry; do not adopt a native-written baseline or retarget/delete the link.

Record only finite no-follow/contained-target type, target byte count and target
SHA256 as a small original-log-link observation in existing runtime-attempt.json
after complete initial inventory succeeds. Unknown/unsafe target shapes keep
the original failure, adding only a fixed refusal category and bounded held
metadata; never serialize raw link text, absolute paths, log contents or errors.
If the actual42-byte target fails any rule, retain the capture and require a
separate source/evidence-backed plan; do not add a fallback or broaden acceptance.

## Scope, meaningful checks and publication

Scope is the existing shared CLI attempt file, one new Linux Qoder log-link
helper/contract file and this plan/validation record. Preserve all earlier
diagnostics, other-client paths, opaque catalog grammar, exact argv/cwd and
budgets, source/pins, original HOME/profile authority, installer/receipt/security,
native trust/authentication/permission, supervision/cleanup and all qualification
flags. No production adapter, user profile relocation, auth workaround, blanket
symlink allowance, ignored subtree, new process, new PR, release or VS Code work.

Actual inert temp-file contracts must cover the sole contained ordinary link
and its unchanged complete target closure, plus other-entry links, hardlinks,
absolute/outside/unclean/self/dangling targets, aliased ancestor/target, FIFO,
oversized target text and changed link/target/empty directory/mode/content.
Verify refusal is before unsafe content reads, metadata records contain no
target text or private sentinel, link/target identity/bytes survive unchanged,
and changed target bytes or any new log entry cannot compare equal. Keep all
existing Qoder full-profile mutation and finite command/opaque JSON contracts.
Use only the root-granted exclusive Go/lint slot for focused contracts and both
affected lint gates; inspect exact scope, obtain independent code review, and
let root publish to existing PR523 using a fresh non-forced ref lease. Only a
fresh genuine source-bound run can establish whether catalog/startup proceeds.
