# Local dependency integrity

`agentplugins deps check` is a strict, offline check of the selected `uap.toml`,
`uap.lock`, and the exact package bytes already present in UAP's immutable store.
It succeeds only when that local agreement and every recorded package can be
verified. It does not establish whether a native client can use the installation.

This document describes the source implementation in
[PR #690](https://github.com/777genius/universal-agent-plugins/pull/690).
It does not imply that a published CLI release contains the command. The
[five-round implementation plan](plans/2026-11-03-dependency-integrity-five-rounds.md)
records the evidence, compatibility decisions, and acceptance scenarios.

## Commands and scope

```sh
# User environment: the default scope for deps commands.
agentplugins deps check
agentplugins deps check --scope user --format json

# Project environment: select it explicitly, including in automation.
agentplugins deps check --scope project --project-root /path/to/project
agentplugins deps check --scope project --project-root /path/to/project --format json
```

`--scope` accepts `user` or `project`. For project scope, `--project-root` must
select an existing project directory; omitting it selects the current directory.
Passing `--project-root` with user scope is an error. Unlike the source-free
`install` command, `deps check` does not infer project scope from a local manifest.
The report identifies the selected scope and agreement paths.

Targets come from `uap.toml`; inherited `--target` is rejected. The command checks
the registry's scope policy for those declared targets without detecting installed
native clients. Project scope retains namespace identity and path-containment
checks. Integrity inspection does not parse unrelated installed-state files;
`deps status` and mutating commands retain their installed-state preflight. A target unsupported in the
selected scope produces `scope_targets_unsupported`. Manually prepared targets
that cannot be saved dependency targets remain excluded. `--format` accepts
`human` (default) or `json`. A dry-run flag is unnecessary: checking already has no
persistent mutation path.

## What the check establishes

The command admits the selected agreement as bounded, stable regular files,
validates its schema and manifest/lock identity, and verifies each pinned package
against a disposable snapshot of the cached bytes. Package manifest and embedded
dependency declarations are parsed from that same verified snapshot. A mutable
source tree is not substituted after digest verification. Recorded graph edges
must agree with those exact package declarations. Before returning success, the
command rereads the agreement and checks that it remained unchanged.

The check does not resolve sources, download missing packages, refresh a directory
catalog, contact security services, invoke native clients, install packages,
repair the store, or rewrite agreement, ownership, receipt, or deployed files.
Verification can create temporary snapshots, which are cleaned up. Ordinary reads
may update filesystem access times; preserving access times is not a guarantee.
The result describes the bytes observed during this check, not future mutations
or complete isolation from a hostile concurrent writer.

An explicit empty manifest with a matching empty lock can be valid. Missing files
or missing cached packages cannot establish validity. A successful local check
does not certify publisher trust, current revocation or vulnerability information,
native activation, authentication, or the integrity of deployed client files.
Use the separate [project readiness diagnostics](PROJECT_READINESS_DOCTOR.md)
and appropriate native verification for those questions.

## Exit and JSON contract

Only `outcome: "valid"` is a successful integrity result. Every other outcome
returns a nonzero command exit. Output failures and invalid command arguments can
also fail the command; argument validation can fail before a report is produced.

JSON reports contain `schema_version: 1`, `scope`, `outcome`, and a `checks` array.
`manifest_path` and `lock_path` are included when the scope is available. Each
check contains `stage` and `outcome`, with optional `code`, `alias`, `path`, and
`message`. Stages are `scope`, `manifest`, `lock`, `package`, and `graph`.
Consumers should use outcomes and codes instead of matching explanatory text.

| Outcome | Meaning | Example codes |
| --- | --- | --- |
| `valid` | The reported stage was verified; overall validity requires every necessary stage. | Successful entries omit `code`. |
| `invalid` | Observed input violates schema, immutable identity, or declaration rules. | `manifest_invalid`, `lock_invalid`, `lock_stale`, `store_mismatch`, `package_invalid`, `graph_declarations_mismatch` |
| `unavailable` | Required agreement files or exact cached package bytes are absent. | `manifest_missing`, `lock_missing`, `package_unavailable` |
| `error` | Operational failures, unsupported scope or concurrent changes prevent inspection. | `scope_unavailable`, `scope_targets_unsupported`, `manifest_unreadable`, `lock_unreadable`, `store_unreadable`, `snapshot_cleanup_failed`, `agreement_changed` |
| `canceled` | Cancellation or a deadline interrupted inspection. | `check_canceled` |

Independent package failures can appear together. A stage whose prerequisites
failed is not fabricated as successful. Overall severity is ordered `canceled`,
`error`, `invalid`, `unavailable`, then `valid`; inspect individual checks to see
all reported failures. A missing package, for example, can produce:

```json
{
  "stage": "package",
  "outcome": "unavailable",
  "code": "package_unavailable",
  "alias": "example",
  "message": "Exact package bytes are not cached. This offline check did not fetch them and cannot establish their integrity."
}
```

This is one `checks` entry, not a complete report. Reports can include absolute
agreement paths; account for that when publishing CI artifacts.

## Five maintained invariants

| Round | Contract |
| --- | --- |
| 1. Identical immutable admission | Warm cache replay and cold acquisition compare the tree digest, manifest digest, actual executable inventory, and any nonempty recorded version. Cache availability must not change which immutable facts are accepted. |
| 2. Strict store metadata | Metadata is bounded on both read and publication by the existing package-file ceiling, strict JSON, and validated against the actual sealed snapshot. Corrupt metadata is distinct from absence, permission failures, and cancellation. Store `Source` metadata is advisory; the lock supplies source identity because identical content may be shared across origins. |
| 3. One manifest boundary | Root, embedded package, and direct in-memory decoder callers share the same dependency-manifest byte limit and parser. Package sidecars retain root-contained link support; authority files and store metadata reject leaf links. |
| 4. Bounded graph work | Resolution has structural and request-wide work budgets, including failed search branches, and checks cancellation during expansion and search. Limit exhaustion is terminal, not a dependency conflict eligible for more backtracking. Recorded and imported locks retain semantic validation without inheriting fresh-solver size caps; indexed lookup and adjacency traversal keep that validation efficient. |
| 5. Truthful local diagnostics | `deps check` gates local integrity without acquisition or persistent mutation. Missing bytes, invalid bytes, operational errors, and cancellation remain distinct outcomes. |

For authoritative locks, absent, `null`, and empty executable inventories all
mean the empty set. An executable-bearing package cannot satisfy such a pin.
Unsorted historical lists are compared using canonical copies without rewriting
the healthy lock; unsafe paths and duplicate or case-fold-colliding entries are
rejected. Omitted historical versions may be hydrated, but a nonempty conflicting
version cannot be replaced silently. Receipt-only repair is a separate incomplete
input: it may hydrate missing facts from verified exact bytes, without relaxing
the lock contract.

Current ceilings are 4 MiB for dependency manifests, 32 MiB for `uap.lock`, and
64 MiB for store metadata, including serialized JSON overhead. The metadata ceiling
uses the existing `domain.DefaultMaxFileBytes` package-file bound, so a legal
large inventory written by an older version is not refused by a new 4 MiB cap.
Store executable
inventories remain within the package limit of 10,000 files. Resolution admits
at most 1,024 source nodes and 8,192 dependency demands, with 4,096 search states
and 8,388,608 work events per fresh resolution. These budgets protect active
search; they are not limits on the validity of an already recorded graph. Tree,
check and import of older large locks retain the existing document byte limits
and all pin/edge/target validation. Manifest edits still invoke resolution, even
when removing a dependency: reducing 1,026 roots to 1,025 still exceeds the new
solver ceiling. Reducing an old graph to an admitted graph (including empty or
one retained package) is supported. No claim is made that every removal bypasses
search budgets. A graph below the solver
node/edge ceilings can still exhaust its work budget; there is no promise that
every topology below those counts can be freshly resolved.

On Unix, strict leaf opens use no-follow and nonblocking admission so a FIFO swap
cannot stall the open; opened handles and final file identity are checked.
Windows admission preserves long paths and delete sharing, rejects redirecting
or irregular files, and accepts NTFS deduplicated files that Go treats as regular.
Offline or recall-dependent bytes produce an operational error without hydration
and without authorizing cache replacement.
These mechanisms do not claim complete protection against arbitrary concurrent
parent-directory changes. Exact-commit platform qualification is recorded in
[PR #690](https://github.com/777genius/universal-agent-plugins/pull/690).
The existing Windows E2E step executes real long-path and held-reader publication
tests alongside the attribute-admission regressions. DEDUP and cloud-placeholder
cases validate attribute handling with fixtures; they do not provision an NTFS
deduplication or cloud-provider volume. Cross-compilation alone does not establish
native Windows or Darwin runtime behavior.

## Status and explicit recovery

`deps status` remains descriptive. It reports installation/environment facts and
retains `not_synced`, with precise lock diagnostics such as missing, invalid,
stale, or unreadable. Those descriptive facts do not become a strict nonzero
integrity gate, and status does not perform the complete cached-package check.
Use `deps check` when automation needs that gate.

For a missing agreement, restore the intended files. For a malformed or conflicted
agreement, preserve the evidence and resolve or restore it explicitly. For
`lock_stale`, review the changed dependency intent before explicitly running
`deps lock`. For access errors, fix access rather than deleting the agreement or
quarantining the cache. For `agreement_changed`, retry after the writer finishes.

If exact package bytes are missing, acquiring or installing them is a separate
action. A reviewed `agentplugins install --locked` uses the existing agreement
and may acquire exact sources and change installed state; it is not an offline
check. Store recovery and receipt-based repair likewise require explicit action
and verified source bytes. Keep the same `--scope` and `--project-root` throughout
diagnosis and recovery. Do not regenerate a lock merely to silence an integrity
failure or treat cache corruption as permission to adopt foreign deployed files.

## Upstream motivation

The following primary-source reports informed the boundaries above. Issue
descriptions are upstream reports, not failures reproduced in UAP. Statuses here
were checked on 2026-11-03:

- [APM #3388](https://github.com/microsoft/apm/issues/3388) (open) reports a warm
  cache identity losing its immutable SHA; proposed
  [#3407](https://github.com/microsoft/apm/pull/3407) was unmerged. The relevant
  invariant is identical warm/cold identity admission.
- [APM #3382](https://github.com/microsoft/apm/issues/3382) (open) reports frozen
  replay deploying a version different from the lock. Keeping lock bytes intact
  is insufficient without checking the acquired package's immutable facts.
- [APM #3374](https://github.com/microsoft/apm/issues/3374) (open) reports a cold
  audit reconstructing an SSH source through HTTPS. This check instead makes
  missing local bytes explicit and performs no acquisition.
- [APM #3028](https://github.com/microsoft/apm/pull/3028) (merged) improves
  conflicted-lock diagnostics while preserving the original bytes. UAP follows
  the same evidence-preserving diagnostic principle, with explicit recovery.
