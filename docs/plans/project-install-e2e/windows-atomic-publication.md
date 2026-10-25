# Windows atomic publication with concurrent readers

## Observed failure

The exact integrated candidate is `6a1d53f4b69f6761dd62039bd36404dad7119a04`,
tree `48a00b68a4f935aee2eddc6c7b761fa787da3bc1`. Terminal run `40201578627`,
Windows job `121193687175`, failed its `concurrent-add` resilience case. The
four public add commands produced no successful result. The preserved artifact
is `12601824719`, 32,106,740 bytes, SHA-256
`83e4c68487ccbb72886079ff3361d16f2d71501a648b115ca2b23283a829323f`.

The artifact's `resilience-evidence/concurrent-add/add-N.stderr` and `.stdout`
retain all four independent results. Commands 0 and 1 refused the held mutation
lock. Command 2 failed replacing an assessment cache entry with access denied.
Command 3 obtained the mutation lock but failed replacing an operation journal,
also with access denied, and reported `managed_rolled_back`. The later broad
Windows CLI contracts passed. Linux and macOS Terminal jobs also passed.

The failed cache publication is `securityscan/cache.go`: on Windows it deletes
the destination before renaming a synced temporary file. Its reader uses
`os.ReadFile`. The failed operations-v2 journal uses `dirswap.Manager.Load`
(`dirswap.go`) with `os.ReadFile`, and `Manager.save` with `atomicfile.Write`.
This is the new directory-swap journal, distinct from the legacy FileJournal.

No handle owner or native handle flags were captured in the failing run. The
two access-denied results establish publication failures; the specific
concurrent reader overlap remains an inference to exercise deterministically.
Do not assign either error to Linux process supervision or relax the four-add
case's required successful winner.

## Primary mechanism and bounded scope

The pinned Go 1.26.10 implementation has two relevant API paths:

- Ordinary `os.Open`/`os.ReadFile` uses Windows sharing without delete sharing;
  ordinary `os.Rename` uses `MoveFileEx` replacement.
- `os.Root.Open`/`Root.ReadFile` uses the rooted Windows open with read, write
  and delete sharing. `Root.Rename` uses a held parent directory and native
  POSIX replacement. The pinned implementation makes one traditional native
  rename attempt after a failed POSIX attempt; neither path removes the target
  first or contains a delay/retry loop.

References: pinned Go source `src/os/root.go`, `src/os/root_windows.go`,
`src/os/root_openat.go`, `src/internal/syscall/windows/at_windows.go` and
`src/syscall/syscall_windows.go`; [Microsoft rename flags](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntifs/ns-ntifs-_file_rename_information),
[rooted rename structure](https://learn.microsoft.com/en-us/windows/win32/api/winbase/ns-winbase-file_rename_info)
and [sharing semantics](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilew).
[Git's independently reviewed implementation](https://code.googlesource.com/git/+/391bceae4350136a05d977573caeaa07059f2136)
also requires destination readers to share deletion for POSIX replacement.
Changing only the replacement primitive does not repair an ordinary open
reader, which remains a required refusal control.

Use the pinned standard library instead of custom unsafe Windows ABI buffers.
Root methods handle native alignment, UTF-16 byte lengths, Unicode names and
long paths. A root is opened on the cleaned parent directory; every operation
receives only the generated temporary or target basename. Root follows the
parent directory as the existing path APIs do, but prevents a leaf traversal
from escaping that parent. Absolute or outside leaf symlinks therefore fail
closed. No broader filesystem or ownership API is changed.

The existing `atomicfile.Write` call sites include filesystem-port writes,
safe mutation and directory-swap journals; state-v2; native configuration and
shared client JSON; selected OpenCode, Kimi and ChatGPT project metadata;
migration backups; staged environment metadata; and retained-data markers.
The writer's same-directory atomic contract remains common to those callers.
Only the two positively implicated reader sites, directory-swap journals and
assessment cache entries, opt into the new read helper. Legacy filesystem-port
reads, state-v2 reads, client configuration reads and every other reader remain
unchanged. This slice does not qualify all other possible reader/writer races.

## Frozen implementation boundaries

Production files are limited to `atomicfile/write.go`, new platform-specific
`atomicfile/read_{windows,other}.go` and `replace_{windows,other}.go`,
`dirswap/dirswap.go`, and `agentplugins/adapters/securityscan/cache.go`.
Tests are new Windows-only files in atomicfile and securityscan; the separate
focused workflow and this plan complete the slice. No state-v2 or legacy
filesystem reader is edited.

1. Add a small `atomicfile.ReadFile` helper with platform files. Non-Windows
   behavior remains `os.ReadFile`. A private Windows `openReadFile` opens
   `os.Root` on `filepath.Dir` and calls `Root.Open` on `filepath.Base`.
   Root closure is checked before returning the file; if it fails after opening
   the file, close the file as well and retain both failures. ReadFile uses
   `io.ReadAll` and a checked file close, retaining errors with `errors.Join`.
   This small private decomposition allows a test to hold the exact production
   read handle without a global hook or additional exported handle API.
   Keep existing decoding, ownership checks,
   cache size/strict-JSON checks and failure behavior in the callers.
2. Change only `dirswap.Manager.Load` and `securityscan.FileCache.Load` to use
   that helper. A cache read failure remains a miss; journal read failures remain
   errors. Do not change the generic filesystem port or unrelated readers.
3. Add `atomicfile.Replace(source, target)` for an already synced and closed
   temporary file. It creates no directory, changes no mode and removes neither
   name on failure. Route `atomicfile.Write`'s existing final publication through
   it. Non-Windows uses the original `os.Rename`. Windows first
   requires cleaned absolute source and destination to have the same parent, opens that
   parent once, and calls `Root.Rename` with their single-component basenames.
   Preserve the existing target symlink/nonregular refusal, temporary mode,
   complete write, sync, checked close, cleanup and parent sync. Check root
   closure and retain native failures. No ACL-ignore or readonly-ignore flags,
   custom native calls, removal fallback, sleeps or application retries.
4. Keep every existing FileCache key validation, parent creation with mode
   0700, JSON encoding, temporary creation/mode 0600, write, sync, checked close
   and deferred cleanup. Replace only its final Windows remove plus rename
   with `atomicfile.Replace(temporaryName, target)`, retaining the cache commit
   error boundary. The common primitive does not create a missing parent, so a
   removed cache root cannot be recreated with a wider mode. Do not treat a
   failed cache write as successful or suppress its error.

## Deterministic contracts and hosted observation

Use private local temporary files, no native client, scanner, account or
network. Native Windows contracts must distinguish these cases:

- Hold an ordinary `os.Open` reader. The legacy replacement control fails;
  corrected publication must still refuse it, retain the old named bytes and
  clean its temporary file. This establishes the required sharing boundary.
- Hold the exact private production `openReadFile` reader. Establish that
  legacy `os.Rename` cannot replace
  the open destination, then require corrected `atomicfile.Write` to succeed.
  The original handle must still read the complete old bytes, while a fresh
  production read returns the complete new bytes. No intermediate deletion or
  partial file is accepted.
- Exercise actual FileCache Store/Load on an existing assessment entry with a
  held delete-sharing reader; require the updated valid assessment and exact
  old bytes through the retained reader. The deterministic held-reader contract
  exercises the former remove-before-rename path directly; no stress loop or
  additional concurrent-store experiment is added.
- Readonly targets, directory targets, unsafe target symlinks where available,
  missing paths and permanent errors continue to fail appropriately. Verify
  old target bytes and absence of leaked temporary files on refusal. Include
  Unicode and long-parent-path names. Existing ordinary write/read and strict
  journal/cache validation contracts remain required.

Run only necessary affected-package contracts, vet and configured lint with
the actual CI merge base plus the unfiltered size/architecture gate. Native
Windows behavior is not qualified by local Linux compilation. Coordinate all
builds because local disk capacity is limited.

The exact new names are `TestWindowsAtomicPublicationHeldReaders`,
`TestWindowsAtomicPublicationRefusals`,
`TestWindowsAtomicPublicationUnicodeLongPath`, and
`TestWindowsAtomicPublicationSecurityCache`. The first three live in
`adapters/atomicfile`; the fourth exercises `agentplugins/adapters/securityscan`.
The positive atomicfile test holds `openReadFile`; the cache test holds the
same pinned Root.Open API and exercises the real cache Store/Load. Existing
atomicfile package tests include a Unix-only mode assertion and a Windows
symlink skip; do not claim those are a portable native Windows qualification.
All four new required tests must execute and pass without skips.

After both independent plan rounds and frozen source reviews, add one finite
`.github/workflows/project-windows-atomic-diagnostic.yml` job triggered only by
push to `fix/project-windows-atomic-20261025` or explicit workflow dispatch.
Use `windows-2025`, checkout pin
`3d3c42e5aac5ba805825da76410c181273ba90b1`, setup-go pin
`b7ad1dad31e06c5925ef5d2fc7ad053ef454303e`, Go `1.26.10`, cache disabled,
read-only repository permissions, a ten-minute job deadline and one `go test`
invocation across exactly the two affected package paths. Use `-count=1`,
`-timeout=2m`, `-v` and an anchored union of the four exact test names.
Preserve raw output even on failure, exact source/tree and measured Go version;
record the actual runner image from GitHub's Set up job log. No retries or
additional environment/containment authority are added. The ordinary-reader
refusal and legacy delete-sharing failure controls execute in the same
observation as the corrected success cases. Do not rerun unchanged failing
source for green. A finite log check requires all four exact top-level PASS
records and rejects missing tests or any skip, before reporting qualification.
Final integration still requires the unchanged concurrent-add resilience
assertions and all normal Required/Core/Coverage/native gates.
