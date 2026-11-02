# Project Windows namespace identity correction — independent R2

Reviewer: codex_claude, 2026-10-23. Frozen correction reviewed:
`c49a50542da071223e9a858096ca9f3508f2ac53` in
`work/uap-core-publication`. Independent of the correction's author and R1.

## Decision: GO for the narrow corrective implementation

No blocker found. The correction pins the physical identity actually obtained
from the open directory handle, rather than retaining Windows' deferred
pathname-only `Lstat` identity across later namespace replacement. The existing
project-root and `.uap` refusal contracts are preserved. Genuine Windows CI is
still the platform runtime verification gate; this review does not replace it.

## Independent source trace

Inspected the installed, pinned Go 1.26.10 source directly:

- `src/os/stat_windows.go`: `File.Stat` calls `statHandle` using the open handle;
  ordinary directory handles reach `newFileStatFromGetFileInformationByHandle`.
- `src/os/types_windows.go`: that constructor immediately fills the volume
  serial and high/low file indexes, leaving `fileStat.path` empty specifically
  to prevent a later `SameFile` from fetching identity again by pathname.
- In contrast, ordinary `Lstat` can take the fast attribute path, call
  `saveInfoFromPath`, and retain only the old pathname. `sameFile` calls
  `loadFileId`, which reopens that stored pathname when it is nonempty. A renamed
  original plus a replacement at the old path can therefore make an earlier,
  still-lazy observation refer to the replacement on its first comparison.

`projectDirectoryIdentity` now returns the eager handle-derived FileInfo for
both retained guard observations. The opened handle is closed before return;
Stat and Close failures are joined and returned. The pre-open real-directory
and no-symlink checks, opened-directory check and `SameFile` consistency check
remain. Its shape matches the established Cline lock-directory identity helper
in `adapters/nativeconfig/lock.go`; it adds no durable identity format,
long-lived handle, lifecycle engine or relaxed replacement check.

## Scope and fixture checks

- The guard still requires exact selected `<project>/.uap` state authority,
  preserves containment checks for all mutable/temporary paths, and shares its
  pinned observations across preview engine views.
- A not-yet-created `.uap` remains allowed. Once a namespace exists and is
  pinned, disappearance or a different physical directory refuses with the
  retained `ErrPlanChanged` contract.
- The test diff is strictly additive. The original namespace replacement and
  preview-view assertions are unchanged; no skip, weakened expectation,
  deletion, workaround platform branch or regenerated expected digest was added.
- The new selected-project replacement fixture retains the original under a
  renamed path, creates a replacement, requires `ErrPlanChanged`, and checks
  that refusal did not create `.uap` in the replacement project. It therefore
  does not depend on file-ID recycling behavior.
- The project-contracts checkout sets `core.autocrlf=false` in Git's process
  configuration before checkout writes the working tree. This matches adjacent
  Windows job configuration. Schema source and pinned embedded-schema digest
  constants are unchanged; the fix preserves their exact-Git-byte contract.
- `git diff --check c49a5054^ c49a5054`: PASS. Worktree was clean on inspection.

Root requested static review while coordinating scarce build-cache headroom;
this R2 did not launch another compile or claim a new local runtime result.
Author/R1 results remain attributed to their separate records. Re-run the
unchanged hosted project-contracts Windows lane on the published corrective
commit before claiming Windows runtime success.
