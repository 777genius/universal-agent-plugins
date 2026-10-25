# VS Code display assets: retain exact kernel identities until cleanup

2026-10-25. Ubuntu 22.04 run `40187145475`, job `121150237501`, source
`1cb91d570cc94b5930d47aa0312a11f2bd1fdbad`, image `20261012.322.1`, failed the
inert replacement test before native execution. Immediate unlink/recreation can
recycle the inode and metadata time quantum; saved FileInfo alone is insufficient
for the intended lifetime identity contract. No native artifact or positive
workspace claim follows from that job.

After the existing exact PID/UID/type/single-link/socket readiness checks, open
only the two already captured display assets with `O_PATH|O_NOFOLLOW|O_CLOEXEC`.
Compare each held descriptor's `fstat` to its captured FileInfo and the named
no-follow identity before accepting either. Keep both descriptors open through
strict display teardown and the existing authorized cleanup decision. An open
descriptor prevents its original inode being recycled into a replacement path.
If opening/validating the second asset fails, close the first; on every execute
exit, close all retained descriptors after cleanup. No descriptor is inherited
by publisher processes, and no new executable/command is added.

Unlink still requires the existing strict tree-reaped success, captured exact
asset metadata, root-owned sticky parent and final named identity validation.
Readiness failure or unproved teardown grants no global unlink authority. Never
replace that gate with a successful close or a missing child handle. Add held
identity verification to the cleanup boundary: for a present named asset, a
missing/closed handle or changed held identity refuses unlink. Close every held
handle after each existing execute cleanup branch, joining close failures into
the original failure and explicit collection failure. No native argv, privilege,
trust, cgroup or policy change.

Keep the original wrong-PID/owner/type/replacement assertions. Make the
replacement test retain the original lock descriptor through unlink/recreation,
matching production's actual lifetime contract, and assert the replacement has
a distinct kernel identity. Add focused inert lock/socket descriptor admission,
partial-open failure/close and no-follow/replacement refusal checks. Existing
socket tests and strict teardown gates remain intact. Run the dedicated Go
tests/vet, then obtain independent frozen code reviews before publication and
the next hosted observation. This amendment does not admit product code.
