# Dependency integrity: five implementation rounds

Date: 2026-11-03. Baseline: `2d2c4f331e8b3c674240f8b1b83fbd0b7c0f1aba`.
Status: **Plan revision 1 — independent criticism pending; implementation has not started.**

## Owner request and delivery boundary

Deliver one new, large **draft** PR containing five substantial, coordinated
improvement rounds. Investigate Microsoft APM's actual issues and useful fixes;
implement only changes that fit UAP and solve a demonstrated problem. Write and
independently criticize this plan before implementation. Use parallel workers
where files and responsibilities are separable. Commit and immediately publish
each coherent increment; do not wait for the full remote CI matrix after every
commit. The owner requested the strongest available models and reasoning effort.

The requested final code-critic loop is a later, separately requested phase.
This delivery stays draft: no merge, release, readiness transition, or claim of
complete cross-platform/native qualification. Focused regression and integration
checks are still part of implementation. Do not change or disable existing CI
gates. Size follows useful coverage, not an artificial line-count target.

## Evidence and overlap inventory

Upstream issue descriptions are reports, not reproductions performed by this PR.
Statuses were checked on 2026-11-03; historical merged fixes are not described as
current APM defects.

| Evidence | Current status | Lesson applied here |
| --- | --- | --- |
| [APM #3388](https://github.com/microsoft/apm/issues/3388), [proposed #3407](https://github.com/microsoft/apm/pull/3407) | Open; proposed fix unmerged | Cache hits must preserve the same immutable identity as cold replay. |
| [APM #3382](https://github.com/microsoft/apm/issues/3382) | Open | Exact replay must compare actual acquired bytes and recorded metadata, not merely keep lock bytes unchanged. |
| [APM #3374](https://github.com/microsoft/apm/issues/3374) | Open | A diagnostic must state whether it needs acquisition; an offline check must never silently fetch. |
| [APM #3028](https://github.com/microsoft/apm/pull/3028) | Merged | Malformed/conflicted agreement files need actionable diagnostics without rewriting the user's evidence. |
| [APM #3373](https://github.com/microsoft/apm/issues/3373) | Open | Validate the complete candidate before persistent effects; do not imply rollback when work is partial. |
| [APM #3433](https://github.com/microsoft/apm/issues/3433) | Open | Repeated successful validation and no-op install must preserve agreement bytes. |
| [APM #3392](https://github.com/microsoft/apm/issues/3392), [#3401](https://github.com/microsoft/apm/issues/3401), [#3426](https://github.com/microsoft/apm/issues/3426) | Open | Already covered by UAP's pinned declaration replay, graph reconciliation and exact commit validation; do not implement them again. |

Current UAP already contains the relevant work from
[#667](https://github.com/777genius/universal-agent-plugins/pull/667),
[#675](https://github.com/777genius/universal-agent-plugins/pull/675),
[#678](https://github.com/777genius/universal-agent-plugins/pull/678),
[#681](https://github.com/777genius/universal-agent-plugins/pull/681),
[#685](https://github.com/777genius/universal-agent-plugins/pull/685), and
[#688](https://github.com/777genius/universal-agent-plugins/pull/688).
Open [#682](https://github.com/777genius/universal-agent-plugins/pull/682) and
[#683](https://github.com/777genius/universal-agent-plugins/pull/683) decompose
installer/CLI code; their declared scope excludes dependency orchestration.
Open [#679](https://github.com/777genius/universal-agent-plugins/pull/679) owns
project doctor/native readiness. This PR excludes those features and avoids its
existing `dependencymanifest/decode_test.go` by using a separate regression file.

The new work concentrates on the remaining **agreement → cached package → graph
→ truthful diagnostic** boundaries. UAP's existing TOML, Go installer authority,
Agent Plugins 1.0 and Agent Skills 1.0 contracts remain the product foundation.

## Confirmed baseline gaps

1. `environment.validatePin` does not validate `ExecutableFiles`.
   `describePinnedDependency` and `loadStoredDependency` trust the store's list
   without comparing the lock's list. A tampered inventory can therefore pass a
   warm unchanged operation while cold publication rejects it. This is an exact
   agreement inconsistency, not evidence of arbitrary execution: the installer
   already reseals newly materialized packages.
2. Cold `readReplayDependency` overwrites recorded version metadata with the
   manifest version; warm `describePinnedDependency` rejects a nonempty mismatch.
   Reconciliation must not quietly repair a tampered nonempty immutable fact.
3. `environmentstore.inspect` uses unbounded `os.ReadFile(metadata.json)` and
   permissive `json.Unmarshal`. It can follow a metadata symlink or block on a
   FIFO before package digest validation. The payload tree is already verified;
   the missing protection is metadata admission and stable, bounded reading.
4. `dependencymanifest.Decode` has no byte limit, and `packageDependencyFacts`
   reads the complete embedded sidecar before decoding. The root agreement has
   a 4 MiB limit, but package and SDK callers bypass it. Package archive limits
   still apply; this is not an unlimited remote package-file claim.
5. Graph expansion and backtracking lack work limits, and fixed-point expansion
   does not check cancellation internally. A bounded input file can describe a
   much more expensive graph/search. Existing cycle and shared-pin semantics
   must remain intact.
6. `deps status` collapses missing, malformed and mismatched lock states into
   `not_synced` and returns success. It is a descriptive command, not a strict
   integrity gate. There is no explicit, offline, non-mutating agreement check.

`deps init` already uses create-only `O_EXCL`; its suspected overwrite race was
disproved and is excluded. State persistence already rejects duplicate ownership
tuples; no duplicate ownership framework is needed.

## Architecture rules

- Keep source semantics and graph policy in the existing `environment` package;
  keep file/JSON admission in existing adapter packages; keep Cobra rendering and
  command wiring in `agentpluginscli`. Do not make adapters depend on CLI code.
- Prefer a small helper with actual multiple callers to a new framework or a
  second parser. Reuse `atomicfile` secure opens and the existing strict JSON key
  validator. A shared bounded regular-file reader may be extracted into the
  existing filesystem adapter if both authority-file and store reads need it.
- Preserve documented valid historical lock behavior: empty version metadata may
  be hydrated, but a nonempty conflicting version must be rejected. Decide and
  test nil/empty executable-list semantics before changing admission.
- Keep source pin verification independent of directory eligibility/security
  network checks. A local integrity check does not certify publisher trust,
  latest revocation state, native activation, authentication, or vulnerabilities.
- No new feature flags, dependencies, daemon, generic policy framework, second
  package engine, APM schema import, profile model, or standalone audit subsystem.
- VS Code implementation and its dedicated fixtures/tests remain frozen.
- Preserve all useful legacy authoring code. Do not edit landing, locale trees,
  release machinery, SDK API generation, or unrelated package-manager UX.

## Round 1 — identical immutable pin admission for warm and cold replay

Owner files: new dependency identity helper/tests; `deps_candidates.go`,
`deps_replay.go`, `deps_sync.go`, and the existing pure lock validator.

Implement a single comparison of locked immutable facts with the verified
package envelope: tree digest, manifest digest, executable inventory and nonempty
version. Use it in warm descriptions, cold descriptions/materialization, and
preflight paths so no-op shortcuts do not skip the invariant. The lock validator
rejects malformed executable paths and duplicate entries using existing portable
path/inventory rules. Do not change requested selectors, source identity,
directory trust policy, or allowed empty historical version hydration.

Acceptance:

- A healthy executable-bearing package survives cold install, warm locked replay,
  repeated lock, and unchanged install with identical agreement/deployed bytes.
- Tampered missing/extra/duplicate/escaping executable entries fail on warm and
  cold paths before state, agreement or managed-file mutation.
- A wrong nonempty version fails on both paths; an omitted historical version
  follows the explicitly retained compatibility policy.
- Use actual CLI entrypoints and filesystem fixtures. At least one new
  regression must fail on the baseline before applying the production fix.

## Round 2 — bounded, strict immutable-store metadata

Owner files: `adapters/environmentstore` and, if justified, an existing shared
filesystem reader. Root authority reads may delegate to that common reader.

Read only stable regular metadata files with a documented ceiling before
allocating. Reject metadata leaf links, FIFOs, directories, oversized/changing
files, invalid UTF-8, duplicate (including case-folded) keys, unknown fields and
trailing values. Validate schema and immutable fields before constructing a
snapshot. Return the existing `ErrMismatch` category for corrupt metadata and
preserve `ErrMissing` only for actually absent entries. Preserve ordinary repair
and quarantine behavior; never use an invalid metadata field to select a path.

Select a ceiling that admits supported executable inventories and add exact-limit
boundary coverage. Symlink policy for store metadata must not be confused with
contained links intentionally supported inside package payloads.

Acceptance:

- Hostile metadata fails promptly without network or writes and does not read a
  symlink target or block on a FIFO.
- Valid cold/warm store use, empty inventories and maximum supported metadata
  continue to work. A corrupt entry can still be explicitly repaired from a
  verified snapshot; failed repair preserves recoverable previous data.
- Focused adapter tests cover platform-specific file admission where supported;
  CLI tests prove corruption cannot become a successful unchanged install/check.

## Round 3 — one bounded dependency-manifest grammar for every entrypoint

Owner files: `adapters/dependencymanifest`, `deps_package_facts.go`,
`deps_document_io.go` constant wiring and dedicated regression tests.

Put the existing 4 MiB manifest limit in the shared decoder contract so root,
embedded package, authoring and in-memory callers agree. Bound the package reader
before allocation and verify its opened file is regular and stable. Preserve the
existing contained sidecar-symlink behavior and `os.Root` containment. Reuse the
same exported limit rather than copy a magic number into each layer.

Acceptance:

- Exactly-at-limit input is admitted when valid; one byte above is rejected
  before TOML parse/source resolution. Raw SDK decoder callers get the same rule.
- Oversized dependency sidecars fail warm/cold and managed/imperative package
  admission before agreement publication or target writes.
- Existing valid manifests, source-location diagnostics, target semantics,
  legacy-extension refusal and contained-link behavior are preserved.

## Round 4 — cancellable and bounded graph resolution

Owner files: `environment/resolve_graph.go`, `resolve_expand.go` and new focused
graph-limit tests. Do not change source selection policy or introduce a solver.

Check context cancellation within fixed-point expansion, candidate enumeration
and backtracking. Bound distinct nodes, dependency demands and total search/work
using one request-local budget. Count work deterministically; shared nodes and
duplicate edges must not create duplicate graph identity. A budget exhaustion is
a typed terminal error, not a version conflict that triggers more backtracking.

Initial values for critic review: 2,048 nodes, 16,384 distinct demands, 8,192
search decisions and 1,048,576 expansion visits. These are ceilings, not promises
that every graph below each independent ceiling is inexpensive. Critics must
check the limits against current fixtures and whether fewer limits suffice.
Document exact behavior and keep it configurable only if a real caller requires
that; no speculative flags or global mutable settings.

Acceptance:

- Cancellation is observed inside expansion and before further source callbacks.
- Very wide/deep and adversarial backtracking fixtures terminate with the typed
  limit error; CLI does not publish a partial lock or deploy target files.
- Existing cycles, diamond/shared graphs, high fan-out, selected updates,
  compatible old transitive pins, exact pins and deterministic no-op graphs pass.
- Avoid timing-only tests. Use known work counts and controlled cancellation.

## Round 5 — strict offline dependency check and actionable status

Owner files: new `deps_check*.go`, command registration, small status additions,
maintainer documentation and existing package-manager E2E catalog integration.

Add `agentplugins deps check` for **local agreement and cached-package integrity**.
It reads the selected scope's manifest/lock, verifies their schema and identity,
validates every already-cached package against its pin and checks declared graph
edges. Missing cached evidence is reported as unavailable and yields a nonzero
result. Never call the replay helper that auto-acquires on a missing cache; use
the store-only verified description plus the shared pure edge checker.

Return a deterministic human report or `--format json` report with a schema
version, selected scope, overall result and per-stage stable issue codes. Report
all independently inspectable package failures without inventing downstream
success after an upstream invalid document. Nonzero means the requested local
integrity could not be established; an empty valid environment is healthy.

The command must not call source resolution/acquisition, security-network or
directory refresh, native client detection, repair, publication or installer
mutation. Temporary verification snapshots are allowed only in disposable temp
roots and must be cleaned; persistent store, agreement, receipts and native
files remain byte/metadata-identical. This is not project doctor or native
readiness. Keep `deps status`'s existing descriptive exit behavior and existing
`not_synced` fact; add precise missing/invalid/stale lock diagnostics and link its
guidance to check/manual agreement restoration, without advising deletion.

Acceptance:

- Healthy empty/nonempty environments, offline warm graph, missing cache,
  corrupt metadata/payload, pin mismatch, missing/spurious edge, malformed and
  conflicted lock, malformed manifest, and project/user selection.
- Assert zero network/source/client calls, deterministic JSON, appropriate exit
  status, retained original bytes and no persistent state/target writes.
- Compose all rounds: hostile metadata/sidecar/inventory is reported truthfully,
  then restoring the original bytes permits a successful check and locked no-op.

## Parallel execution and verification

After plan approval by independent critics, R1, R2 and R3 are independent worker
lanes. R4 can follow one completed lane or be handled by the integrator. R5
depends on the shared checks from R1–R3 and is integrated after their APIs settle.
Assign exclusive file ownership; only the integrator changes command registration,
the plan/review ledger, shared E2E catalog and workflow-facing evidence fixtures.
No worker commits another worker's uncommitted files.

Before changes, focused `environment` and `environmentstore` packages passed on
Go 1.26.10. During each round run the changed package and its relevant real CLI
regressions. New tests target critical integrity/refusal boundaries, not every
helper. At integration run the dependency suites and package-manager CLI cases,
format checks, existing architecture checks and appropriate race checks. Preserve
the existing heavy multi-OS/native matrix for the later requested qualification.
Record exact commands and outcomes; a queued/skipped/unrun check is not a pass.

## Independent plan reviews and decisions

Revision 1 will receive three independent reviews:

1. Supply chain and immutable identity: exploitability, trust boundaries, legacy
   compatibility and avoiding false assurances.
2. Architecture and scope: dependency direction, shared helpers, overlap with
   active PRs, no unnecessary frameworks or feature expansion.
3. Lifecycle and verification: failure-before-mutation, offline side effects,
   cache repair, cancellation/limits, realistic cross-platform regression tests.

Record each actionable finding, disposition and revision here. Re-review the
revised plan before implementation. This is plan review, not the deferred final
code-critic loop.

## Delivery checklist

- [ ] Plan reviewed by multiple independent critics and revised/re-reviewed.
- [ ] Round 1 implemented and focused regressions pass.
- [ ] Round 2 implemented and focused regressions pass.
- [ ] Round 3 implemented and focused regressions pass.
- [ ] Round 4 implemented and focused regressions pass.
- [ ] Round 5 implemented and composed CLI scenarios pass.
- [ ] All intended changes published in one draft PR; exact final head recorded.
- [ ] Remaining CI/platform/final-review work described accurately.
