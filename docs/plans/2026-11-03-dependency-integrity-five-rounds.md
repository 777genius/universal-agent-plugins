# Dependency integrity: five implementation rounds

Date: 2026-11-03. Research baseline: `2d2c4f331e8b3c674240f8b1b83fbd0b7c0f1aba`.
Implementation base also includes `b5221a9adb561bdc8e4a89499c4113e359641e9a` (#679).
Status: **All five rounds and revision 3 corrections are published in PR #690. The owner has now requested complete E2E qualification, further independent review and fixes, and merge if the resulting change is sound. Revision 4 review corrections are complete; exact final platform evidence and the merge outcome are recorded in the PR.**

## Owner request and delivery boundary

Deliver one new, large **draft** PR containing five substantial, coordinated
improvement rounds. Investigate Microsoft APM's actual issues and useful fixes;
implement only changes that fit UAP and solve a demonstrated problem. Write and
independently criticize this plan before implementation. Use parallel workers
where files and responsibilities are separable. Commit and immediately publish
each coherent increment; do not wait for the full remote CI matrix after every
commit. The owner requested the strongest available models and reasoning effort.

The final code-critic loop was initially deferred and was started after the owner's follow-up questioning the new limits. On 2026-11-03 the owner explicitly extended the task to complete E2E, independent review and corrections, and merge if qualification is successful. This supersedes the original draft-only boundary and authorizes the readiness transition needed by the existing CI. It does not authorize a release.
Do not change or disable existing CI gates. Size follows useful coverage, not an artificial line-count target.

## Revision 4: final qualification and merge plan

1. Read the current PR head, review threads and check results. Preserve unrelated work and use the exact published head for CI evidence. Diagnose existing lint failures before changing code.
2. Run the existing full portable package-manager workflow on Linux, Windows, macOS ARM and macOS Intel by marking the PR ready. Require actual complete case inventories, unchanged-source evidence and the real executable/scanner phase; a restricted local subset or cross-compilation is supplementary evidence only.
3. Have three independent critics examine graph/compatibility policy, filesystem/cache admission, and CLI/composition/architecture. Each must also assess whether the plan or implementation adds unnecessary restrictions or abstractions. Prefer demonstrated behavioral defects and minimal corrections; do not grow the feature scope or add CI policy.
4. Reproduce findings, make focused fixes, and have a different critic re-review each correction. Run the directly relevant regressions before publication and the existing required qualification on the final code. Triage GitHub review comments with the same evidence standard; do not dismiss or resolve an unresolved defect.
5. After source edits settle, independently review the complete diff and final evidence from behavior and maintainability perspectives. Record concrete outcomes and any remaining boundaries, keeping historical verification distinct from final-head results.
6. Merge only after the final head has passed the applicable existing gates, full four-platform package-manager E2E, and independent review with no unresolved material findings. Use the expected head SHA when merging and verify the resulting main-branch commit. Do not bypass protection, weaken assertions, add skip paths or publish a release.

The previously documented active-solver tradeoffs remain review inputs, not an invitation to increase all budgets or add another policy layer. Any newly identified regression needs a minimal, separately explained correction before it is treated as resolved.

### Revision 4 review corrections and local evidence

Four additional independent GPT-6 Astra/ultra critics reviewed graph semantics,
filesystem/cache admission, CLI/composition/architecture, and E2E evidence. All
approved the qualification plan. Their concrete source finding was that Go marks
unavailable Windows cloud placeholders as `ModeIrregular`, so shared type
admission could return `ErrUnsafeFile` before the Windows availability check and
therefore authorize store corruption handling. Availability attributes are now
checked before regular-file admission; no irregular or redirecting file becomes
admissible. The new regression uses Go-shaped synthetic `FileInfo`, not a live
cloud volume.

The initial remote lint failures were also corrected: explicit read-only handle
cleanup, snapshot fixture cleanup, inventory preallocation, import grouping,
integer range, and small extractions for tree indexing/rendering and Unicode
registry-name normalization. Operational snapshot cleanup still suppresses
corruption classification deliberately. No lint exemption, dependency, feature
flag or product admission rule was added.

The CLI critic independently approved the filesystem correction, and the
storage critic independently approved the CLI extractions. The evidence critic
confirmed the existing Windows jobs did not run the five bounded-reader tests;
the existing Windows held-reader E2E step now executes and validates those tests
alongside its original publication test, retaining the JSON log in the same
artifact. Workflow routing, platform matrix and draft/ready policy are unchanged.

Local verification after these corrections, using Go 1.26.10:

- Configured changed-line lint and full-file architecture/size lint passed for
  all three changed Go modules, without exclusions or baseline expansion.
- Full atomicfile, environmentstore, packagedigest and dependencymanifest
  packages passed (0.016s, 20.140s, 0.058s and 0.032s respectively).
- Focused CLI/production checks passed after final extraction (3.554s / 0.173s);
  race verification passed (2.073s / 1.526s). Cleanup-refusal subcases ran without
  skips. Independent tree/status and registry-association reruns passed.
- Independent environment tests passed (2.881s). A scratch-only differential
  oracle compared baseline/current resolution and old-pin replay on 2,000 seeded
  conflicting/shared/cyclic graphs with multiple targets; results, exact locks
  and failure text matched (1.780s).
- All 19 existing Python package-manager evidence/CI-policy tests passed.
  Windows atomicfile tests cross-compiled. Native execution remains distinct
  evidence supplied by the final remote Windows job.

Current main contains the installer/CLI refactors from #682/#683 at
`b68f83b5587e974995415ed936562f852e5ddf33`. Their paths do not overlap this PR,
and Git's combined merge tree is conflict-free. GitHub's PR jobs check the
synthetic merge commit; final evidence must identify its parents as the
published PR head and current base. Full E2E includes all 82 registered cases,
production installation/composition inventories and the real executable/scanner
phase. That phase is an imperative nonempty lifecycle; it does not separately
execute every new composed `deps check` scenario.

The PR description is the final exact-head CI and review ledger, allowing it to
record completed evidence without changing already qualified source files.

Two fresh final critics then independently identified the same remaining
composition issue: the agreement transfer reader rejected nonregular `FileInfo`
before reaching the shared reader, so an unavailable Windows placeholder could
still be labeled `manifest_invalid` or `lock_invalid`. The minimal correction
removes this duplicate pre-read type gate, preserving missing-file handling and
the outer post-read identity check. A wrapper regression verifies that strict
admission error identity survives into the command diagnostic. The platform
reader's synthetic placeholder test remains the attribute-ordering oracle; no
live cloud-volume test is claimed. Final source qualification follows this
correction and its independent re-review.
Both final critics independently approved the correction after focused tests.
The new wrapper regression failed for both manifest and lock before the fix.
A broader local transfer selector still encounters the documented protected Git
runner failure at private repository initialization in `TestDependencyTransferGraphRoundTrip`;
the unchanged real-Git path remains subject to supported-host E2E rather than a
local runner substitution or skipped qualification.

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
Merged [#679](https://github.com/777genius/universal-agent-plugins/pull/679) owns
project doctor/native readiness and is included in the implementation base. This PR excludes those features and avoids its
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
  validator. Existing `atomicfile.OpenReadFile` is an ordinary open, not a
  no-follow/nonblocking security primitive. Add a narrowly named bounded regular
  file reader in that adapter for authority files and metadata. Keep the rooted,
  contained-link package-sidecar reader a separate semantic route; no policy flags.
- Preserve documented valid historical lock behavior: empty version metadata may
  be hydrated, but a nonempty conflicting version must be rejected. For an
  authoritative lock, absent/null/empty executable lists all mean the empty set.
  Accept unsorted historical lists by comparing canonical copies; reject duplicates
  and unsafe paths, and do not rewrite a healthy lock solely to sort its list.
  Receipt-only repair is a separate incomplete input and keeps verified hydration.
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

Implement one unexported CLI comparison of locked immutable facts with the verified
package envelope: tree digest, manifest digest, executable inventory and nonempty
version. Use it in warm descriptions, cold descriptions/materialization, and
preflight paths so no-op shortcuts do not skip the invariant. Give mismatches a
typed/sentinel category for diagnostic callers, rather than matching error text.
The existing pure `environment` package owns lexical inventory validation; the
integrator wires it into `validatePin`. Reuse portable source-path rules where
their grammar matches. Do not move private installer filesystem validation into
the domain or edit the #682 installer implementation.

Lock-derived pins must match the actual verified inventory even when the recorded
list is empty. Receipt-only cold repair lacks inventory by design: acquire and
verify its exact recorded tree/manifest first, hydrate inventory before strict
comparison/publication, and keep that explicit repair route inaccessible to lock
replay. Nonempty recorded versions still cannot be hydrated over a disagreement.
Do not change requested selectors, source identity or directory trust policy.

Acceptance:

- A healthy executable-bearing package survives cold install, warm locked replay,
  repeated lock, and unchanged install with identical agreement/deployed bytes.
- Tampered missing/extra/duplicate/escaping executable entries fail on warm and
  cold paths before state, agreement or managed-file mutation.
- A wrong nonempty version fails on both paths; an omitted historical version
  follows the explicitly retained compatibility policy.
- The same executable-bearing package remains repairable from recorded receipts
  with and without a warm store. Repair hydration never admits an omitted
  executable inventory in an authoritative lock.
- Use actual CLI entrypoints and filesystem fixtures. At least one new
  regression must fail on the baseline before applying the production fix.

## Round 2 — bounded, strict immutable-store metadata

Owner files: `adapters/environmentstore` and, if justified, an existing shared
filesystem reader. Root authority reads may delegate to that common reader.

Read only stable regular metadata files with a documented ceiling before
allocating. Reject metadata leaf links, FIFOs, directories, oversized/changing
files, invalid UTF-8, duplicate (including case-folded) keys, unknown fields and
trailing values. Validate schema and inventory grammar/uniqueness before building
a snapshot; then compare the metadata inventory with the snapshot's canonical
actual executable members. Ghost, directory, link and duplicate claims must not
survive override-map deduplication. Source metadata is advisory: identical content
can legitimately share a store entry across origins, and the lock supplies origin.

Return existing `ErrMismatch` for corrupt/unsafe metadata and `ErrMissing` only
for actually absent entries. Preserve cancellation, permission and other I/O
errors as operational errors, because mismatch authorizes repair/quarantine.
Preserve ordinary repair
and quarantine behavior; never use an invalid metadata field to select a path.

Use the existing 64 MiB package-file envelope (`domain.DefaultMaxFileBytes`)
for serialized store metadata, applied before publishing and while reading.
Revision 3 replaces the original 4 MiB metadata cap after a legal old-writer
cache regression was reproduced. Validate a 10,000-member inventory with deep portable paths and JSON
escaping overhead; unusually larger serialized metadata is explicitly rejected
before publication, rather than creating an entry the reader cannot reopen.
Add exact-limit boundary coverage. Payload links keep their existing policy.

The strict reader must avoid blocking during opening if a regular file is swapped
for a FIFO: use a nonblocking/no-follow leaf open on Unix and a reparse-aware
file-handle open on Windows, preserve Windows delete sharing and long paths,
inspect the opened handle and recheck identity. Regular NTFS deduplicated files
remain readable; redirecting reparse points remain refused. Test the inspection-to-open swap using an explicit
local test seam/subprocess, not sleeps. This guarantee does not claim to prevent
all hostile concurrent modification of arbitrary parent directories.

Expose a narrowly scoped `Store.Snapshot(ctx, algorithm, digest)` that returns a
verified disposable `domain.PackageSnapshot`; the caller releases it with existing
snapshot cleanup. Share verification with `Get`; do not add another store engine.
Offline pin/declaration checks parse this very snapshot, not a mutable source tree
after its verification copy has been discarded. The integrator owns consumer wiring.

Acceptance:

- Hostile metadata fails promptly without network or writes and does not read a
  symlink target or block on a FIFO.
- Valid cold/warm store use, empty inventories and maximum supported metadata
  continue to work. A corrupt entry can still be explicitly repaired from a
  verified snapshot; failed repair preserves recoverable previous data.
- Metadata and lock agreeing on a ghost executable must still be rejected.
- Snapshot cleanup runs on success, failure and cancellation; metadata publication
  above the ceiling fails before replacing an existing healthy store entry.
- Focused adapter tests cover platform-specific file admission where supported;
  CLI tests prove corruption cannot become a successful unchanged install/check.

## Round 3 — one bounded dependency-manifest grammar for every entrypoint

Owner files: `adapters/dependencymanifest`, `deps_package_facts.go`,
`deps_document_io.go` constant wiring and dedicated regression tests.

Put the existing 4 MiB manifest limit in the shared decoder contract so root,
embedded package, authoring and in-memory callers agree. Bound the package reader
before allocation and verify its opened file is regular and stable. Preserve the
existing contained sidecar-symlink behavior and `os.Root` containment. Reuse the
same exported limit rather than copy a magic number into each layer. The decoder
remains pure; bounded package reading stays outside it. A contained-link rooted
open must also avoid blocking on a swapped FIFO on Unix; R2 owns the low-level
opened-handle helper, R3 owns the bounded rooted read and parser integration.

Acceptance:

- Exactly-at-limit input is admitted when valid; one byte above is rejected
  before TOML parse/source resolution. Raw SDK decoder callers get the same rule.
- Oversized dependency sidecars fail warm/cold and managed/imperative package
  admission before agreement publication or target writes.
- Existing valid manifests, source-location diagnostics, target semantics,
  legacy-extension refusal and contained-link behavior are preserved.

## Round 4 — cancellable and bounded graph resolution

Owner files: `environment/resolve_graph.go`, `resolve_expand.go`, a small pure
graph-limit module and tests. Budgets apply to fresh solving. Recorded-lock
validation uses indexed source lookup and adjacency target propagation without
inheriting those solver caps; the integrator bounds/cancels `deps tree` output.
Do not change source selection policy or introduce a solver.

Check context cancellation within fixed-point expansion, candidate enumeration
and backtracking. Bound distinct nodes, dependency demands and total search/work
using one request-local budget, allocated once for the entire `ResolveSelected`
operation and never reset by backtracking or a new candidate. Count work
deterministically; shared nodes and
duplicate edges must not create duplicate graph identity. A budget exhaustion is
a typed terminal `ErrResolutionLimit`, never a version conflict that triggers
more backtracking. Existing/imported locks retain schema, pin, edge and target
validation, with efficient indexed/adjacency traversal. They are not rejected
merely because they exceed the budgets for a new search. This preserves read,
import and locked replay of previously valid large agreements. Resolution-based
edits, including reductions, must still fit active-solver budgets; reduction to
an admitted graph is supported, not an unlimited removal fast path.

Use 1,024 distinct source nodes and 8,192 graph demands as active-solver
structural ceilings. Use 4,096 search states and 8,388,608 work events per resolution. One
work event is a node processed during expansion/search, an examined dependency
declaration (including duplicates), or an enumerated candidate (including rejected
candidates). Check candidate-list budget before sorting/callbacks. These are
documented safety ceilings, not minimum capacity guarantees for every topology.
No public configuration or mutable global policy is added.

Replace the repeated full fixed-point scan in expansion with a deterministic
worklist that re-enqueues a node only when its demands/targets change. This keeps
co-installation/cycle semantics while avoiding cubic ordinary-chain expansion;
the existing choice/backtracking policy remains. Before accepting the change,
measure deterministic work counts for 1,024 independent roots, a 256-node chain,
fanout64, dense shared DAGs and cycles. They must fit the default budget. If they
do not, revise the implementation/plan rather than silently lower acceptance.

Tree formatting uses bounded indentation (32 levels, with explicit depth after
that) and context checks per emitted node/edge. The existing 32 MiB lock
document bound applies to input; recorded graphs do not inherit solver counts. References and cycles remain explicit. Do not introduce topological
startup ordering or claim that exceeding a resource budget makes the graph invalid.

Acceptance:

- Cancellation is observed inside expansion and before further source callbacks.
- Very wide/deep and adversarial backtracking fixtures terminate with the typed
  limit error; CLI does not publish a partial lock or deploy target files.
- Existing/imported large locks remain inspectable and reducible without source
  callbacks. Efficient semantic validation preserves invalid-graph refusals;
  deeply nested admitted locks render bounded-indentation output.
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
version, selected scope, overall result and per-stage stable issue codes. Outcomes
are `valid`, `invalid`, `unavailable`, `error` and `canceled`; only `valid` exits
zero. Stages are `scope`, `manifest`, `lock`, `package` and `graph`. Missing exact
bytes are `unavailable`, not an invalid lock; permission/I/O is `error`, cancellation
is `canceled`. Keep codes compact and fixed by operation/typed errors, not string
matching. Report
all independently inspectable package failures without inventing downstream
success after an upstream invalid document. Nonzero means the requested local
integrity could not be established. A valid empty manifest plus matching empty
lock is healthy; absent agreement files are a distinct nonzero result. Unsafe or
unsupported scope fails before local validity is asserted.

The command must not call source resolution/acquisition, security-network or
directory refresh, native client detection, repair, publication or installer
mutation. Temporary verification snapshots are allowed only in disposable temp
roots and must be cleaned; persistent store, agreement, receipts and native
files retain contents, existence, mode, mtime, file identity and directory entries
where portable. No atime guarantee is made. Capture and recheck agreement inputs
before returning success. Parse package facts from R2's verified snapshots.
This is not project doctor or native
readiness. Keep `deps status`'s existing descriptive exit behavior and existing
`not_synced` fact; add precise missing/invalid/stale/unreadable lock diagnostics,
and distinguish missing/mismatched/unreadable store without triggering a full new
package scan in status. Reuse typed agreement/check primitives and link guidance
to check/manual agreement restoration, without advising deletion. Existing status
facts describe synchronization, and must not be presented as a full integrity proof.

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
lanes around the declared contracts. R1 owns the unexported locked-envelope
comparison and pure inventory validator. R2 owns strict filesystem helpers and
`Store.Snapshot`; R3 owns the shared manifest-size constant/decoder. The integrator
wires shared lock validation, snapshot consumers, root-file reader reuse and all
cross-lane calls after APIs exist. No worker edits another lane's owned files.
R4 can follow one completed lane or be handled by the integrator. R5
depends on the shared checks from R1–R3 and is integrated after their APIs settle.
Assign exclusive file ownership; only the integrator changes command registration,
the plan/review ledger, shared E2E catalog and workflow-facing evidence fixtures.
No worker commits another worker's uncommitted files. After the core APIs settled,
the integrator delegated exclusive ownership of the three E2E catalog files to
the R1 worker and a new status-regression file to the R3/R4 worker; publication
and plan ownership remained with the integrator.

Before changes, focused `environment` and `environmentstore` packages passed on
Go 1.26.10. During each round run the changed package and its relevant real CLI
regressions. New tests target critical integrity/refusal boundaries, not every
helper. At integration run the dependency suites and package-manager CLI cases,
format checks, existing architecture checks and appropriate race checks. R1–R4
refusal preserves agreement, existing package/state/managed bytes and does not
publish a partial lock; ordinary coordination lock files and explicitly permitted
temporary/acquisition effects are not claimed to be nonexistent. Preserve
the existing heavy multi-OS/native matrix for the later requested qualification.
Record exact commands and outcomes; a queued/skipped/unrun check is not a pass.

## Independent plan reviews and decisions

Revision 1 received three independent reviews:

1. Supply chain and immutable identity: exploitability, trust boundaries, legacy
   compatibility and avoiding false assurances.
2. Architecture and scope: dependency direction, shared helpers, overlap with
   active PRs, no unnecessary frameworks or feature expansion.
3. Lifecycle and verification: failure-before-mutation, offline side effects,
   cache repair, cancellation/limits, realistic cross-platform regression tests.

All three requested revisions before implementation. Revision 2 dispositions:

| Finding | Decision |
| --- | --- |
| Nil/empty inventory unspecified; receipt-only cold repair would break | Fixed authoritative empty-set rule, canonical order comparison, separate explicit repair hydration, warm/cold repair acceptance. |
| Envelope alone does not prove metadata inventory | Verify grammar/uniqueness and compare to the resealed snapshot's actual inventory, including ghost/dir/link cases. |
| Existing open can block on FIFO replacement | Add platform-aware nonblocking/no-follow authority open and separate contained-root sidecar route, with controlled swap tests. |
| Verification copy discarded before offline parsing | Add one verified snapshot accessor, explicit cleanup and agreement recheck; keep source metadata advisory. |
| Metadata read cap could reject the writer's own output | Use the same 4 MiB cap on read and prepublication, with supported large-inventory and boundary tests. |
| Corruption conflated with I/O; check outcome unspecified | Preserve operational errors; define five outcomes/stages, unavailable vs invalid, descriptive status compatibility. |
| Resolver-only bounds miss imported locks and tree output | Share structural admission, bounded indentation and cancellation across locked graph paths. |
| Initial budgets reject benign graphs due repeated expansion | Replace fixed-point rescans with deterministic worklist; specify counts and tested 1,024-root/256-chain envelope; single lifetime budget. |
| Shared ownership/race claims too broad | Assign helpers and integration ownership, preserve domain purity/contained links, specify allowed effects and exclude atime guarantees. |

All three critics re-reviewed revision 2 and approved it without remaining
planning blockers. The lifecycle critic additionally required unsorted historical
inventory acceptance in both comparison and operational SDK copies (without lock
rewrites), and retained the 1,024-root/256-chain budget gates. These are included
in implementation acceptance. These are the historical revision 2 decisions.
Revision 3 below supersedes the metadata cap and recorded-graph admission choices
after concrete code-review counterexamples.

## Postimplementation code review — revision 3

The owner's follow-up asked whether the limits could make behavior worse.
Three new independent code critics reviewed graph/resource behavior, input and
cache compatibility, and actual CLI diagnostics/cleanup. The initial code was
not approved unchanged. The following corrections are part of this PR:

| Confirmed finding | Correction |
| --- | --- |
| A legal 8,000-executable package with only 80,000 payload bytes produced 4,736,215 bytes of old-writer metadata; the new 4 MiB reader rejected it as corrupt. | Derive the symmetric metadata ceiling from the existing 64 MiB package-file limit, and retain an old-writer/new-reader regression. The 4 MiB dependency-manifest limit is separate and unchanged. |
| A valid 1,025-node lock (600,691 bytes) and a 129-node/8,256-edge lock (606,041 bytes) became unreadable and could not be reduced to empty. | Remove fresh-solver size admission from semantic lock validation. Index source lookup and propagate targets through adjacency queues; keep solver budgets local to active solving. |
| Raw Windows `CreateFile` lost Go's long-path normalization and rejected regular NTFS DEDUP files. | Preserve extended drive/UNC paths and delete sharing; admit the regular DEDUP tag while refusing redirected/irregular/unavailable files. Native Windows execution remains a separate gate. |
| Missing temporary storage returned `ENOENT`, which could be mislabeled as corrupt cache and authorize quarantine. | Mark temporary-output failures at their source in the snapshot builder; preserve operational error identity even when temporary storage is nested under the input tree. |
| Offline/recall-dependent Windows files were labeled unsafe and therefore repairable corruption. | Preserve a Windows filesystem operational error without reading or hydrating unavailable bytes. |
| A missing required `plugin.json` in verified bytes was mislabeled as a storage-access error. | Classify content absence after snapshot verification as invalid package content, keeping pre-snapshot I/O operational. |
| Snapshot cleanup failure could still return a successful offline verdict and leave temporary files behind. | Propagate cleanup failures as operational errors; an integrity check cannot report success after failed cleanup. |
| Short registry-name mapping scanned all pins for every root, and cancellation during decode could be overwritten by another verdict. | Index registry names and aliases once; observe cancellation after admission before reporting its result. |
| The actual project CLI could block on a FIFO in unrelated installed state before checking the agreement, even after cancellation. | Reuse scope/namespace identity and path guards while separating integrity checking from installed-state parsing. Existing status and mutation preflight remain intact. |

The remaining solver ceilings are deliberately not described as harmless to
every valid input. A conflict-free graph of 1 root, 8 shared hubs and 1,000 leaves
(1,009 nodes, 8,008 edges) exhausted the work budget at state 892 after about
29 seconds and 2.60 GB cumulative allocations in a zero-I/O fixture. Raising all
budgets would increase resource exposure. The correction protects historical
agreement usability while keeping an explicit resource ceiling on fresh search.
The 1,024-node chain and flat graphs fit the current budget; these observations
are examples, not a capacity guarantee for every topology.

A second reviewer also reproduced partial reduction from 1,026 roots to 1,025
roots hitting the solver ceiling. This remains an explicit scope boundary:
manifest edits invoke resolution, so only reductions whose resulting search fits
the budget are supported. Reading, inspection and import do not inherit that
ceiling. No blanket claim is made that every removal is limit-free.

Fixes were re-reviewed by a different critic from their author. The second
round found the nested-temporary-directory error-classification case and the
actual production project-state FIFO blockage; both were corrected and checked
again. All three reviewers now approve their reviewed scope, with no remaining
reproduced code blocker. The partial-reduction limit and unexecuted native
platform qualification remain explicit limitations, not completed checks.

| Final revision 3 verification | Result |
| --- | --- |
| Full environment suite and historical graph regressions | Passed; independent rerun also passed. |
| Full packagedigest and environmentstore suites | Passed; store run 19.432s includes exact 64 MiB boundaries, real old-writer cache, nested/outside missing temp output and missing input. |
| Independent registry-canonicalizer comparison | 10,000 deterministic old/new parity cases passed. |
| Focused public-command E2E catalog | 14/14 selected cases passed in 4.831s; existing validator verified 82 test/subtest pass events with no skips. Catalog now contains 82 cases. |
| Production project integrity commands | Invalid unrelated state and FIFO state passed; namespace and agreement unchanged; ordinary status retains state preflight. |
| Final CLI and production command race check | Passed (2.166s and 1.600s). |
| Architecture gates | All six existing gates passed again. |
| Windows strict-reader tests | Final test binary cross-compiled; native long-path, share-delete, DEDUP and offline filesystem execution remain unperformed. |
| Formatting and whitespace | Passed. |

Published review corrections: `7563132ddd3525f8617a44800ae4f4e70c942638`
(recorded graph compatibility) and `a2ec8fa994fb831a4ee6225df5b17bc52c3db573`
(cache/Windows/temporary-storage/CLI/scope fixes). The following documentation
commit records the dispositions. No merge, release or broad platform
qualification is implied by this review phase.

## Initial implementation and verification ledger (before revision 3)

Draft: [#690](https://github.com/777genius/universal-agent-plugins/pull/690).
Implementation head: `8269559d70b0d5c482e54ce8312cba85e204b1d4`.
That head records the initial implementation before revision 3. The PR
description records the latest published branch head and subsequent fixes. Workers used GPT-6 Astra with ultra reasoning,
as requested. The three independent critics reviewed and approved the plan before
production edits; this is separate from a final review of the implemented code.

Published implementation increments:

| Commit | Content |
| --- | --- |
| `9d581b539978107532eb5b477a721bc6609c3173` | R1–R3 together, including their shared strict reader and verified-snapshot contracts. |
| `f170b2f6265557a86a5670d8e9aa5ca6de894061` | R4 resolver budget, strict/compatible lock admission and iterative cancellable tree rendering. |
| `8269559d70b0d5c482e54ce8312cba85e204b1d4` | R5 offline check, precise descriptive status, maintainer documentation and composed catalog cases. |

The missing executable-inventory warm-lock regression failed on the baseline
before its fix. Oversized embedded-manifest and direct-decoder regressions also
failed before their production changes. These were reproduced UAP defects;
the upstream issue reports remain motivation rather than UAP exploit evidence.

| Verification | Observed result |
| --- | --- |
| R1 immutable replay | 20 warm/cold × lock/install tamper cases passed; unsorted historical inventories preserve agreement bytes and mtimes; old absent versions and explicit receipt repair remain supported. |
| R2 atomic-file/store suites | Linux focused and race suites passed; strict JSON, FIFO substitution, real executable membership, exact cap boundaries, 10,000-member inventory, snapshot cleanup and repair preservation covered. |
| R2 cross-compilation | Windows amd64 and Darwin arm64 test binaries for both atomicfile and environmentstore compiled successfully; they were not executed on those platforms. |
| R3 manifest admission | Shared decoder boundary tests and CLI cold/warm lock/install/imperative refusal passed, including contained links and existing nonstandard-format controls. |
| R4 environment regression | Complete environment package passed; candidate enumeration, backtracking lifetime budgets, terminal exhaustion and callback cancellation covered. |
| R4 public-command boundaries | Tree accepts 1,024 roots and a 256-node chain; tree/check/import/lock reject 1,025 nodes or 8,193 recorded edges. Fresh oversized resolution performs no source acquisition. |
| R5 offline check | Empty/nonempty agreement, user/project isolation, unsafe namespace refusal, corrupted/absent metadata or payload, pin/graph mismatch, concurrent edits, cancellation and snapshot cleanup passed with forbidden source/native/security effects. |
| R5 status | Existing and new status tests passed; missing/malformed/conflicted/stale lock diagnostics preserve successful descriptive exit and `not_synced`; missing/corrupt/unreadable/canceled store remain distinct. |
| Integrated race check | Environment and CLI focused resolution/integrity/status/tree suites passed with `-race` (32.272s and 7.005s respectively). |
| Existing archive compatibility | 59/59 archive-safe CLI catalog scenarios passed (65.778s), with their required run/pass events verified by the existing Python validator. |
| Existing production composition | 10/10 installation catalog scenarios (11.618s) and 3/3 ZIP lifecycle/scope restore journeys (7.517s) passed. |
| New composed E2E catalog | 12/12 new cases passed (3.970s); evidence validator confirmed 79 test/subtest passes, no skipped cases. Catalog now contains 80 cases in the existing six groups. |
| Architecture and evidence tooling | All six existing dependency-direction/import-boundary gates and all 13 Python evidence-validator tests passed. Formatting and diff whitespace checks passed. |

Deterministic positive resolver measurements:

| Graph | Search states | Work events |
| --- | ---: | ---: |
| 1,024 independent roots | 1,025 | 1,576,448 |
| 256-node chain | 257 | 99,455 |
| One root with 64 children | 66 | 10,661 |
| Dense shared 64-node DAG | 65 | 93,665 |
| 64-node cycle | 65 | 6,434 |

Representative reproducible commands, with Go 1.26.10:

```sh
go test ./install/integrationctl/adapters/atomicfile \
  ./install/integrationctl/agentplugins/adapters/environmentstore \
  ./install/integrationctl/agentplugins/adapters/dependencymanifest \
  ./install/integrationctl/agentplugins/environment -count=1

go test -race ./install/integrationctl/agentplugins/environment \
  ./cli/internal/agentpluginscli \
  -run 'Test(Resolution|Dependency(Check|Status|Tree|Pin|RecordedRepair|PackageManifest))' -count=1

go test -tags=uappm_e2e ./cli/internal/agentpluginscli \
  -run '^TestPackageManagerE2E$/(agreement|registry)/(pin-envelope-warm-cold-refusal|pin-inventory-order-noop|pin-legacy-version-hydration|recorded-repair-inventory-hydration|embedded-manifest-size-refusal|graph-large-recorded-tree|graph-resource-limit-refusal|offline-check-valid-noop|offline-check-damaged-state|offline-check-declared-graph|offline-check-concurrent-edit|offline-check-cancellation)$' \
  -count=1 -json

python3 scripts/test_package_manager_e2e.py
```

Environment-specific verification limits are recorded explicitly. The host
provided duplicate case variants of proxy environment variables; test child
processes normalized those keys while retaining the remaining environment and
`HOME`. The protected production Git runner still fails during private `git init`
in this container, before the changed admission logic. New local-Git regression
fixtures use the repository's existing `directGitTestRunner` with real disposable
repositories; production process containment is unchanged. This does not qualify
the production Git transport. Archive and production ZIP compatibility passed.

Full `make lint` was not run because `golangci-lint` is unavailable. The complete
remote multi-OS CI/native-client matrix, unchanged-binary production-scanner
qualification were not run. The code-critic/fix loop is recorded separately in
the revision 3 section above. No
workflow was changed or disabled, no release was made, and the PR remains draft.

## Delivery checklist

- [x] Plan reviewed by three independent critics and revised/re-reviewed; all approved revision 2.
- [x] Round 1 implemented and focused regressions pass.
- [x] Round 2 implemented and focused regressions pass.
- [x] Round 3 implemented and focused regressions pass.
- [x] Round 4 implemented and focused regressions pass.
- [x] Round 5 implemented and composed CLI scenarios pass.
- [x] All intended changes published in one draft PR; branch head recorded in its description.
- [x] Remaining CI/platform work described accurately.
- [x] Revision 3 code-review findings fixed, tested and independently re-reviewed within the documented scope.
