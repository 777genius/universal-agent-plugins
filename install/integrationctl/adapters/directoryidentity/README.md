# Directory identity primitive (P1, unwired)

`Capture(ctx, explicitAbsoluteRoot)` resolves Unix aliases once, then records
version 1 and every directory from the namespace root through that existing
canonical directory. `Revalidate(ctx, expected)` walks the frozen spelling with
component no-follow handles and compares the complete authority. Neither call
creates directories, discovers HOME, reads settings or grants consent.
A missing selection returns `ErrBootstrapRequired` (`profile_bootstrap_required`).
Permission, malformed metadata and other acquisition errors fail closed.

`Authority` and the stdlib-only `domain.ProfileAuthority` have private immutable
encodings. Their constructors validate structure only; a constructed or decoded
value is not filesystem proof. Facts returns fresh slices, zero cannot encode,
and JSON rejects ambiguous/unknown fields, trailing values and input over 2 MiB.
Version 1 has at most 256 complete ordered entries with unique physical ID
tuples, paths at most 4096 UTF-8
bytes without controls, and encoded IDs at most 128 bytes. The thin
`agentplugins/adapters/profileauthority` bridge only converts and calls this API.
No optional port, planner, client, installer, state, journal or AN consumer is
wired here. Native settings, URI, SDK loader and CLI identities stay separate.

Each component has an owned held directory handle. Before return, held metadata
and parent-child edges are repeated, followed by another complete namespace walk.
Every descriptor is closed on success, unsupported results, cancellation and
errors. Cancellation is checked between bounded operations, not promised to
interrupt an in-flight kernel syscall. Filesystem transitions are allowed and
individually queried; packageview's single-volume inventory restriction is absent.
Observed concurrent drift denies. This is not atomic exclusion of arbitrary
path writers after the last check; cooperating effect fencing belongs to P2.

Linux amd64/arm64 uses the v6.10 source floor's `FS_IOC_GETFSUUID` request
`0x80111500`, a 17-byte length/UUID buffer and a separate fstat inode. An O_PATH
pin is reopened at `.` relative to itself as O_RDONLY/DIRECTORY/NOFOLLOW/CLOEXEC;
the opened device/inode and directory kind must match before ioctl. Length 1–16
is retained in lowercase hex, including leading zero bytes; meaningful UUID
bytes must be nonzero. Only ENOTTY maps this ioctl to `ErrUnsupported`. No
procfs, block device, boot, PID, device-number or network fallback exists.
Read-only GETFLAGS refuses case-folding directory tuples whose spelling cannot
be resolved from Linux held handles without procfs. Other query errors fail.
The source floor establishes the ABI, not driver/filesystem qualification.
Actual namespace/filesystem tuples need independent native evidence before a
future opted-in consumer claims physical qualification. The implementation
queries each held descriptor; success on one filesystem says nothing about others.

Darwin arm64 queries local APFS using held-fd fgetattrlist, bitmap count 5,
ATTR_CMN_RETURNED_ATTRS and ATTR_VOL_INFO|ATTR_VOL_UUID. The bounded packed
returned length and UUID bit are checked before decoding 16 nonzero UUID bytes.
A separate fstat supplies the directory inode because a volume query substitutes
its volume-root vnode. Missing UUID bit is unsupported; syscall errors including
permission/EINVAL are failures. Held-fd F_GETPATH checks native spelling and
refuses unresolved APFS case/Unicode aliases. The pinned XNU source establishes no actual
macOS/APFS version floor; native APFS arm64 qualification remains pending.

Windows amd64/arm64 uses a fixed-drive root and NTFS checks on every held
handle, relative NtCreateFile metadata probes, HANDLE-relative list-access
upgrades and no delete sharing while anchors live. Reparse/unsafe attributes
are rejected before traversal. Volume serial and file-index high/low use the
existing decimal scheme. UNC, device paths, ADS and unresolved case aliases
are refused. Native NTFS identity, mount transitions and reparse qualification
require Windows CI; cross-compilation is not that evidence.

Other OS/ABIs return unsupported for the new primitive only. Legacy dirswap
calls the extracted `LegacyIdentity` conversion and keeps exactly its Unix
`dev:inode` and Windows `serial:high:low` encodings, without requiring UUIDs.
No Go SDK platform gate or legacy journal schema changes.

UUID cloning, inode/file-ID reuse and privileged namespace/volume remapping can
escape this evidence. Normal reboot/login stability is an intended qualified
filesystem property, not a reboot test performed by P1. No security engine,
store, lease, framework, inventory or discovery extraction is introduced.

Tests use fresh TEST directories only. Independent ABI/decoder vectors run on
Linux; genuine fd metadata and fresh-process supported/unsupported outcomes
are recorded. Replacement and ancestor-substitution tests report NOT_RUN when
the actual complete namespace lacks UUID support, rather than claiming a
replacement PASS. Native Darwin/Windows and supported Linux durability evidence
remain separate future requirements. No physical consumer, release or full
programme qualification follows from these primitives.
