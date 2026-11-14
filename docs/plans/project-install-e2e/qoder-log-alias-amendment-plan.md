# Qoder collector amendment: exact log alias metadata

Plan only, 2026-10-24. The genuine failed run40130660701 and artifact12575528479
are preserved with verified raw members and original false qualification fields
in `docs/evidence/project-scope-research/qoder-1.1.78-linux-amd64-40130660701`.
Both independent artifact interpretation and two fresh plan review rounds are
required before collector source changes; two code GOs precede publication.

## Observed blocker and narrow change

The four pinned Qoder1.1.78 help/version calls succeeded in the actually verified
networkless/nonroot namespace. Its logger created `home/.qoder/logs/latest` as a
symlink. The first bounded inventory correctly refused it before any list or
install. Only its lstat mode/type and 42-byte target length were observed; the
artifact supplies no target contents. No populated installation schema exists.

Amend the collector inventory for this one exact label/path only:
`home/.qoder/logs/latest`. Treat it as **link metadata**, never as a regular file
or directory and never as a package/discovery path. Native stage inventories may
opt into this exception through an explicit argument that defaults false. The
complete source-baseline inventory and every before/after source-pin check keep
the strict default. Only the two native before/after-stage inventory call sites
may opt in. Source labels remain source-user/source-A/source-B, never home. An input path or native-emitted string cannot add exceptions.

The current anchored held directory supplies `os.stat(...,follow_symlinks=False)`
and `os.readlink(...,dir_fd=fd)` for the exact entry. Before readlink, require
`st_nlink == 1` and `0 <= st_size <= 4096`, with sufficient remaining total
inventory byte budget. Pass a bytes name and preserve the returned literal
target bytes (or exact `os.fsencode` bytes), without Unicode replacement or
operational path decoding. After readlink, check the budget again, require the
actual byte count to equal the pre-read st_size and stay within4096/aggregate
bounds, and require matching full before/after lstat identity. Include those
bytes in the existing total inventory byte budget. Record kind=`symlink_metadata_only`, target byte count,
SHA256 and an exact hex representation; explicitly record `target_followed:false`.
Do not decode it into an operational path, resolve it, open it, stat the target,
scan it, execute it or use it to choose any later native command.

The literal may be relative, absolute, dangling, external or point at a special
file: only the bounded bytes are data. Neither target validity nor its contents
are asserted. A changed/oversized/multiply-linked entry refuses the stage with
partial metadata. Every other symlink, hardlink/special-file case and every
fixture symlink still refuses. All ordinary file/directory identity and byte/
entry/depth/path bounds remain unchanged. If the known alias is an ordinary
file/directory instead, its existing typed bounded inventory rules still apply;
it does not create a broad exemption from checking that path.

## Unchanged execution and qualification

Keep official acquisition pins, real hosted admission, actual gitHEAD check,
original/effective UID/GID/capability/network proof, finite19native-command table,
source full-tree equality before and after every probe, strict JSON/error
fail-stop, original HOME/profile authority, independent8minute collection/
5second cleanup+staging deadlines and separately admitted64MiB upload directory.
No new native operation or target-path read is added. Never skip the alias silently:
metadata must appear at each successful stage observation.

The failed artifact remains failed. A new exact-source hosted run is required.
Even a completed amended collection retains all eight qualification fields false;
actual populated rows need independent interpretation before a concrete adapter
plan, two plan rounds and product implementation. No production Qoder adapter is
included in this amendment.

## Inert contracts and review gates

Add focused no-vendor tests proving:

1. Exact allowed native home alias records literal metadata and hashes without
   opening its target, including a dangling target, foreign readable file and
   FIFO/directory target. Foreign bytes are never read and no call can block on
   a target. Include exact non-UTF8 bytes roundtrip and keep no-follow directory
   admission checks, including an aliased intermediate .qoder/logs directory.
2. The exception defaults off; identical aliases under each source fixture,
   differently named home entries and aliases in intermediate directories still
   refuse, with no target dereference.
3. Oversized readlink bytes, multiple links and lstat/readlink replacement races
   refuse while preserving partial metadata and existing deadline behavior.
4. Complete source snapshots still detect additions/removals/mode/hash changes
   before every native probe and after it. Existing inventory, capture, bounded
   cleanup/upload and real hosted-guard refusal suites continue to pass.

No Qoder binary is executed locally. Freeze the small collector/test delta,
obtain two independent code GOs, publish only those reviewed bytes, and verify/
retain the next actual artifact regardless of outcome. Unexpected further
native aliases require another concrete review; they do not widen this rule.


## Independent R1 artifact interpretation and plan decisions

Two independent reviewers, `skillshare_scope` and `claude_codex_scope`, verified
all15 preserved raw member lengths/SHA256 hashes and all7 stdout/stderr pairs.
The former also independently verified ZIP size/hash/CRC. Both confirmed the
same exact failure boundary, four successful help probes, absent populated
list/install evidence, unchanged false qualification fields and completed
cleanup. No client or acquisition ran during either interpretation.

Both gave **R1 GO** at `a665f6b81b4e5e072b6de6de245519c5de0c0370` for this
metadata-only amendment. Their concrete R2 precision is incorporated above:
pre-read nlink/size/budget admission, exact raw bytes and size equality, post-read
identity/budget checks, two opt-in native call sites only, intermediate alias
refusal and non-UTF8/race tests. No target interpretation or broader alias policy
was added. Fresh R2 review remains required before collector source changes.
