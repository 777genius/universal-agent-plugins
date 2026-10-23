# Project Windows directory identity correction

Author: core_clients, 2026-10-23. Corrective scope follows the existing approved
project-core plan. The hosted project-contracts Windows job failed both retained
namespace replacement assertions and exact embedded-schema digest validation.
Linux/macOS project jobs passed at that checkpoint.

## Production defect and correction

Go 1.26.10 `src/os/types_windows.go` stores a pathname in an `Lstat` FileInfo
and defers obtaining the volume/file ID until `SameFile` invokes `loadFileId`.
Consequently a retained namespace FileInfo can observe a replacement at its old
path on its first comparison. The original directory remains present under its
renamed path in both failing fixtures, so this was not file-ID recycling.

The guard now captures directory FileInfo using `os.Open` and `File.Stat`,
closing the handle before return. Its stored FileInfo contains the already
observed handle identity. Both selected-project and `.uap` observations use this
route. The existing pre-open real-directory check and SameFile consistency
check remain fail-closed, matching the established nativeconfig Cline lock
identity capture pattern. No long-lived handle, new lifecycle mechanism,
durable identity schema or relaxed replacement check was added.

Existing namespace and preview replacement tests retain their exact refusal
assertions. A focused selected-project replacement contract additionally proves
that root identity is pinned and a refused replacement creates no `.uap`.

The project-contracts checkout step now sets `core.autocrlf=false` through the
checkout process environment before files are written. This is the existing
exact-Git-byte pattern used by adjacent Windows lanes; expected embedded schema
digests and schema bytes remain unchanged.

## Local verification

Go 1.26.10, GOMAXPROCS=3, `-p=2`, exact project-focused regression selection:

- installer: PASS 0.277s;
- internal production CLI: PASS 0.014s;
- public CLI composition: PASS 0.184s;
- golangci-lint 2.13.2 changed-code gate: 0 issues;
- golangci-lint 2.13.2 whole-file size/architecture gate: 0 issues;
- `git diff --check`: PASS.

This task host is Linux. Genuine Windows validation requires rerunning the
unmodified hosted project-contracts lane; neither cross-compilation nor this
local run is a Windows runtime result. Independent corrective reviews remain
separate records.
