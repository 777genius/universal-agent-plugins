# Dependency graph efficiency and safe reduction

Date: 2026-11-03. Research baseline: `07ab910b75e397c9f223078e3c7d9f701f2f36a6`.
Status: **A–E implemented and locally verified in draft PR #697. First combined-code findings corrected; three final review lanes and native qualification remain before merge.**
Delivery: one draft PR, publish this plan immediately, then publish each coherent implementation increment. Complete E2E, independent review/fixes and the authorized merge. No release is included.

## 1. Goal and scope

Make large dependency graphs substantially cheaper to resolve and allow an existing valid environment to become smaller without forcing it through fresh-search size ceilings. Preserve immutable package admission, deterministic choices, source/target policy, cancellation, and the existing installation/publication authority.

The owner requested substantial improvements with a detailed plan, researchers and multiple critics before implementation, immediate publication of commits, and end-to-end completion in one PR. A large line count is not an acceptance criterion. Include work because it fixes a demonstrated problem, not to approach 10,000 lines.

This is a coordinated follow-up to [PR #690](https://github.com/777genius/universal-agent-plugins/pull/690), not a reopening of its completed qualification. The work is bounded to the existing dependency graph engine, its CLI consumers, directly relevant regression/benchmark fixtures, and documentation.

### Included

1. Remove repeated whole-graph reconstruction and avoidable allocation from active resolution while preserving the existing search policy.
2. Admit and reuse validated immutable dependency facts within one resolution; preserve strict source/selector validation and dynamic package aliases.
3. Preserve rollback of graph/choice state during failed candidate search, and preserve lifetime resource accounting.
4. Fix mutation of provider-owned candidate arrays during sorting.
5. Resolve a compatible, closed set of previously locked pins without fresh candidate search, allowing root/target contraction even when the remaining graph exceeds fresh-search counts.
6. Remove repeated full-lock edge scans from declaration verification used by exact replay, import and offline check.
7. Exercise the complete public-command lifecycle, including strict install, user/project isolation, warm/cold retained packages, failure preservation and actual native platforms through the existing E2E.

### Excluded

- Raising or making configurable the existing node, demand, state or work limits; new flags, feature flags, global caches, telemetry, dependencies, or a replacement solver.
- New lock/manifest schemas, version-selection policy, hidden moving-ref refresh, broader source/target eligibility, or automatic acceptance of changed immutable package facts.
- Reimplementing installer transactions, receipts, ownership, drift, recovery, security policy, Directory trust or source transports.
- Landing, registry publication, SDK expansion, legacy authoring/YAML retirement, VS Code implementation/configuration/tests, and product releases.
- New mandatory CI matrices, performance timing thresholds, general lint rules, or exemptions for failing tests.
- Real DEDUP/Cloud Files infrastructure in this graph PR. Existing Windows native reader tests remain in the normal qualification; no live DEDUP/OneDrive claim is added.

## 2. Research and reproduced baseline

Three independent researchers examined solver performance, large-graph lifecycle semantics, and native Windows qualification/coverage. Root inspected the engine, coordinator and CLI routing independently. All production files remain unchanged at this stage. The full baseline environment package passes on Go 1.26.10 (3.012 s).

The performance researcher used scratch-only Go overlays plus CPU and allocation profiles. These are single-run observations from the same host, not timing guarantees or proposed CI thresholds.

| Graph | Baseline result | Elapsed | Cumulative allocated bytes | Allocations |
| --- | --- | ---: | ---: | ---: |
| 1 root, 8 hubs, 1,000 shared leaves: 1,009 nodes / 8,008 edges | Work exhausted at state 892; empty result | 27.853 s | 2,602,135,792 | 45,287,071 |
| 1,024 independent roots | Success | 4.119 s | 945,663,792 | 5,332,003 |
| 1,024-node chain | Success | 2.583 s | 558,716,272 | 6,369,796 |
| 1,024-node fanout | Success | 4.944 s | 1,051,976,624 | 8,482,723 |
| 1,024-node cycle | Success | 2.699 s | 558,717,000 | 6,369,817 |
| 128-node / 8,128-edge dense DAG | Success | 2.621 s | 253,218,944 | 4,396,360 |
| 1 root, 1,000 parents, 1 shared child | Success | 6.636 s | 1,151,451,912 | 10,142,866 |

The shared-hub failure consumed 8,388,531 work events under the unchanged 8,388,608 ceiling. Cumulative allocation is not peak RSS. CPU samples attribute 84.52% cumulative CPU to dependency expansion and 61.46% to requirement validation inside it; these percentages overlap. Allocation samples attribute about 49.88% flat bytes to demand merging, 18.34% to source-key construction, 9.68% to version-matching splits, 4.40% to target union, and 3.98% to choices-map copying.

### Demonstrated defects and gaps

- `graphSolver.search` reconstructs the graph from roots for every candidate state; `try` copies all choices. Shared declarations are repeatedly validated, sorted and merged.
- `candidates` sorts the slice returned by `GraphResolver.Candidates` in place. A probe proves that provider-owned ascending candidates are reordered; the current fixture masks this by returning a copy.
- A valid 1,025-root graph reduced to 1,024 succeeds with no candidate/acquisition callbacks; 1,026 to 1,025 fails at the fresh 1,024-node limit. Removing only one target from a 1,025-root agreement also fails.
- Actual saved `remove`, edited-manifest `deps lock`, and unlocked changed-intent `deps sync` encounter that limit. Their baseline refusals preserve manifest, lock, cache, state, managed files and operation files.
- Ordinary `install` with any existing lock is already strict, like `install --locked`. An edited manifest produces a digest mismatch. This is a required behavior to preserve, not an opportunity to silently re-resolve on install.
- `verifyDeclaredEdges` scans all recorded packages/edges for each parent, producing an avoidable cross-product during exact replay/check/import of large graphs.
- Root inspection found that cold exact replay discards snapshot cleanup errors in `readReplayDependency`, while warm `describePinnedDependency` preserves them. Reproduce this directly and align the cold path with the existing joined-error contract as part of retained-package verification; do not build a new cleanup abstraction.
- Existing large-history tests mostly drop everything or retain one independent pin. They do not cover a contraction that still exceeds fresh-search counts.
- `BenchmarkVersionSelectionSharedGraph` is currently a small star. Keep its useful historical cases, but add real shared-hub and high-fan-in shapes.

## 3. Architectural contracts

The pure `environment` package owns graph/search policy and the existing-pin proof. It performs no filesystem/network operations; it calls the existing resolver ports. CLI remains responsible for acquiring/verifying package facts, selected scope, snapshot lifetime, agreement publication and installer composition. Reuse current types and private helpers; avoid a generic graph framework or a second source of truth.

The following contracts must remain:

- Lexicographic source selection; existing candidate sorting/tie-breaking, old-pin reuse, exact-release preference, selected-update rules and downgrade protection.
- Stable ownership aliases supplied by recorded pins, roots or acquired package manifests. Declaration-local names do not redefine environment ownership.
- Current immutable pin checks, executable inventories, manifest limits and snapshot cleanup/error identity.
- Fresh-search ceilings: 1,024 sources, 8,192 dependency demands, 4,096 states, 8,388,608 work events. Count actual fresh-search work; do not reset its lifetime counters on rollback or hide candidate enumeration. Verified historical closure is a cancellation-aware traversal, not fresh search, and does not inherit any of these four ceilings.
- Terminal resource exhaustion, cancellation and operational failures. Only actual dependency conflicts are eligible for candidate backtracking.
- Current `Eligible` callback behavior and compatible-pin exemption. Eligibility can surface Directory/scope/security/immutable-content errors and must not become an unreviewed policy cache.
- Existing environment locks, input/preimage checks and publication/application ordering. No promise of universal rollback after publication; existing partial-result/retry behavior remains truthful.
- No inspection or reacquisition of removed unreachable packages merely to authorize their removal. Existing legacy-cleanup behavior remains.

## 4. Work package A — owned candidate input and prepared graph facts

Owner: solver lane. Primary files: `environment/resolve_graph.go`, `resolve_expand.go`, narrowly scoped private helpers and resolver regression/benchmark files.

1. Clone a returned candidate slice after the existing response-size/work admission and before sorting. Preserve ordering and eligibility rules; rejected oversized responses must not cause the added allocation.
2. Strictly validate original declaration key/alias, source identity, endpoints, paths and selectors once when immutable package facts are admitted. Reuse canonical source keys and deterministic declaration order within this request.
3. Keep dynamic binding aliases separate from immutable declaration/source facts so old pins, root aliases and acquired manifest names retain current precedence.
4. Avoid blanket deep-copying of resolver objects or new persisted caches. Make ownership of reused facts explicit and keep provider inputs unchanged.

Acceptance: provider-owned candidates/dependency data remain unchanged on success, conflict and cancellation; invalid declarations are refused with current error categories; exact candidates, same-version releases and scoped eligibility keep their order and distinctions.

## 5. Work package B — incremental active graph with rollback

Owner: solver lane, integrated by root. Retain the existing DFS/candidate solver and ordered eligibility scan.

1. Carry branch-local graph state through successful choices instead of rebuilding from roots. Record only changed nodes/choices needed to undo a rejected branch.
2. Propagate outgoing declarations when a package is newly selected or its target set grows. New incoming selector demands invalidate acceptance, but do not themselves change outgoing declarations.
3. Continue target propagation to a fixed point, including shared nodes and cycles. Deduplicate logical demands without losing distinct parent/selector facts.
4. Restore node existence, demands, targets, exact-release promotion, owner aliases and local graph counts on rollback. Never roll back request-lifetime work/state counters.
5. Reuse a pure acceptance result only while selected immutable pin, source identity and full demand set are unchanged. Invalidate/restore that state with its branch; no eligibility callback result is cached by this optimization.
6. Build and validate the final lock through one existing finalization path, with deterministic packages, `RequiredBy`, selectors, targets and manifest digest.

Preserve the representative requirement passed to source callbacks, not just the final intersection of demands. The old expansion starts with sorted roots and a deterministic breadth-first queue. For roots `a,z`, edges `a -> b`, `b -> zz (^1.0)` and `z -> zz (^1.5)`, selecting `z` changes the representative for `zz` to `^1.5`; retaining the earlier first-seen requirement is incorrect. Prefer a narrow read-only traversal of prepared facts only when distinct competing requirements make callback selection ambiguous. Root precedence and equivalent requirements need no traversal; preserve exact-release promotion and conflicts between distinct exact releases. Charge this actual fresh work and keep cancellation checks. Do not introduce general dynamic graph-ranking machinery without measured need.

The 8,192 demand ceiling counts dependency edges, excluding direct-root demands. Preserve the deduplication identity of child source, parent owner alias and selector. Admit fresh declaration/candidate batches before sorting, copying or preparation. Include dense roots-plus-edges coverage, exact edge boundary and one-over refusal; do not freeze incidental work totals that should decrease with actual work.

Acceptance: the previously failing 1,009-node/8,008-edge graph resolves successfully and validates under the same limits; representative independent, deep, cyclic, fan-in and dense shapes show measured allocation/work reductions. Record before/after metrics on equivalent successful workloads separately from the old early-failure case. No wall-clock assertion enters CI.

Rollback regressions must cover a rejected candidate adding unique nodes, a shared descendant, new targets, stricter demands, alias changes and exact-release promotion. The accepted lower-candidate result must match a clean resolution without the rejected branch. Preserve cancellation inside preparation/propagation and after every callback class.

## 6. Work package C — verified existing-pin closure

Owner: contraction lane. Add one private path on the current graph solver before fresh search; root owns the small integration edit in shared solver entry/finalization code.

The previous lock does not preserve the old set of direct roots. Therefore, do not pretend to prove that the new manifest roots are a subset of the old roots. Prove the useful invariant instead: every resulting package is the same old immutable pin, every required declaration is satisfied by an old pin, and every inherited target set is contained in that pin's previous targets.

### Conservative applicability

- No aliases selected for update.
- Every current root maps to a previous source key and the same recorded requested selector. Preserve stable locked aliases, including a logical declaration-key rename where current contracts allow it.
- A new source, changed selector/source, expanded targets, or selected update makes this path inapplicable. Continue through existing fresh resolution rather than introducing a new refusal.
- Complete cheap applicability checks for every root before the first `DescribePinned` callback. Clearly fresh oversized requests must retain their existing early refusal without acquisition.
- Require the existing compatible-pin eligibility exemption for every reached pin. Do not infer a broader exemption from mathematical target containment: a valid old lock can contain unsorted targets, for which the current helper invokes `Eligible`. Such nonexempt histories fall back to existing fresh resolution; they are not newly classified as malformed. Preserve the original callback behavior there.
- Keep `ValidateCompatiblePins` and its schema, duplicate alias/source, malformed pin, unknown consumer, selector and inventory checks before this attempt.

### Verified traversal

Traverse current roots using verified `DescribePinned` declarations, loading each reached pin once within the request. Drain propagation among already-described pins before choosing the next unresolved source in lexical order. Check the old pin's acceptance before its first description, then use the verified selected pin for later demands. This preserves the existing fallback frontier: incompatible or unknown-version old pins must not be read or hydrated into a different candidate-selection decision. Do not first traverse every old `RequiredBy` edge and load that closure: old unnecessary edges must not cause unrelated removed packages to be read.

Compare described packages with the recorded immutable pin before using their declarations. Preserve the old ownership alias. Hydrating a previously absent Version is allowed, as in existing replay; a present version and every other immutable field must remain identical. Only graph intent (`Targets`, `RequiredBy`, requested selectors) may be rebuilt through existing finalization rules.

An absent old child, incompatible demand or target expansion makes the shortcut inapplicable before publication. Resolver failures, cancellation, corrupt retained content, malformed manifests, immutable fact mismatches and cleanup failures remain terminal; they do not turn into a speculative candidate retry.

Rebuild candidate edges from verified package declarations and validate the final lock against current requirements. Missing/unnecessary recorded edges must not newly prohibit explicit `deps lock` reconstruction or changed-intent sync: those commands already rebuild facts. Exact install and exact same-digest replay keep their existing stricter edge verification before this path.

Historical traversal checks cancellation during preparation, queue processing and target propagation, and after callbacks. It bypasses fresh source/demand/state/work ceilings, including the fresh per-package declaration preflight. On inapplicability discard speculative nodes, choices, queues, local counts and acceptance state. Retain only successfully validated immutable descriptions/prepared facts; apply those into clean fresh roots under ordinary fresh admission/work charging. Fresh-search counters are initialized once and never reset during search or backtracking. This separation adds no new historical-graph ceiling and cannot exempt a fresh solve from its existing limits.

### Acquisition and application boundaries

- Removed unreachable packages are never described or acquired.
- Valid warm retained cache permits zero candidate enumeration, moving-ref lookup or source downloads.
- Missing retained bytes may use existing exact immutable acquisition through `readReplayDependency`, including its current Directory/scope/security checks. Cache absence alone does not justify candidate search.
- Corrupt retained bytes remain a failure, not permission to redownload or replace the cache.
- The current CLI staging, environment locking, agreement recheck, selected bindings and transaction application remain authoritative.

Acceptance: 1,026 to 1,025 roots; target-only removal above the node limit; retained graph above the edge limit; deep/shared/cyclic retained closure; root-to-transitive retention and removal of unreachable cycles; deterministic exact pins/aliases/targets/edges. New-source/selector/target/update edits still use the existing resolver and preserve its results/refusals.

## 7. Work package D — indexed declaration verification

Owner: CLI verification lane. Primary files: `deps_replay.go`, `deps_check.go`, related private helper/regression files.

Build the recorded edge lookup once per verification operation, indexed by consumer/parent, and use it to compare each verified declaration set. Preserve every recorded edge, including duplicates and selector-distinct entries: an index must not overwrite evidence that the current comparison rejects. Retain source lookup and the existing missing/unexpected-edge error categories and wording. The current map iteration does not define which of several independent errors appears first; do not claim byte-for-byte ordering compatibility for those cases. Exact replay, import-with-install verification and offline check must use the same narrow comparison helper rather than duplicate indexing policy.

Do not cache verification across commands or skip package byte verification. Keep context checks, snapshot cleanup, stable issue codes, `deps check` outcomes and offline side-effect guarantees. A malformed or mismatched graph must not become accepted merely because lookup is faster. Add a focused cold-acquisition cleanup regression, then preserve both the original failure and `errDependencySnapshotCleanup` when cleanup fails; a cleanup failure must not publish a usable resolved package or fall back to candidate search.

Bound this cleanup repair to the existing retained verification/materialization owners: `readReplayDependency`, the pin/Directory failure branches in `materializeLockedPackage`, the failure branch in `loadStoredDependencySnapshot`, the existing `loadSnapshotMode` failure closure where loader/declaration/digest/security failures are cleaned up, and `Eligible` after successful materialization. Reuse local joining conventions; add no installer-wide lifecycle abstraction and claim no repair of every CLI cleanup path. Preserve cleanup/cancellation precedence over `ErrDirectoryIneligible` conversion in `Eligible` and, where shared loading can deliver the joined error, `AcquireCandidate`. A pure policy refusal may backtrack; policy refusal joined with cleanup failure must remain terminal. Regressions use real verified payloads and deterministic snapshot-ownership cleanup refusal, including success, pin mismatch and joined policy refusal, without privilege-dependent permission skips.

Acceptance: same verdicts/diagnostics for missing, extra, incompatible and shared edges, with representative large-graph evidence showing the repeated full-lock scan removed. Retain ordinary descriptive `deps status` behavior.

Explicit index regressions cover identical duplicate edges; two distinct compatible selectors from one parent where only one is declared (both edge orders reject); legitimate shared parents with different selectors; empty declarations; and missing child sources. Preserve current duplicate-source declaration semantics rather than broadening syntax in this optimization.

## 8. Work package E — end-to-end lifecycle integration

Owner: root/CLI E2E lane after the pure contracts settle. Extend existing graph cases where they represent the same behavior; add a small named case only for a distinct public contract. Keep the existing catalog groups, runner matrix and evidence validator.

| Public path | Required behavior |
| --- | --- |
| Saved `remove`, root or selected target | Contract a verified old graph, including an over-limit retained result, and preserve unrelated owners/data. |
| Edited manifest + `deps lock` | Publish the correct deterministic remaining agreement without moving exact pins. |
| Edited manifest + unlocked `deps sync` | Use the compatible existing-pin path where applicable, then normal installation composition. |
| Ordinary `install` with an old mismatched lock; `install --locked` | Continue refusing manifest digest mismatch; no silent contraction/re-resolution. |
| Exact install after the contracted agreement is saved | Replay/verify the new pair through the existing path. |
| Plain `deps import` | Continue transferring a structurally valid exact agreement without acquiring package bytes. |
| `deps import --install` and `deps check` | Verify retained declarations through the common comparison helper; offline check reads local snapshots only. |
| Dry run, cancellation, corrupt retained cache, input race, recovery/drift refusal | Preserve the existing failure and publication boundary; report partial application honestly where that existing contract applies. |
| User/project selection | Retain namespace, agreement and binding isolation; no unrelated state/client activity is newly authorized. |

Use valid cached payloads for retained-package scenarios. The old graph fixture deliberately contains missing payloads and forbidden acquisition and cannot be reused to imply that retained verification may ignore missing content. Avoid a huge Cartesian product: use a representative large portable lifecycle plus small focused critical boundary regressions.

Register the retained-over-limit journey inside the existing `agreement/graph-recorded-compatibility` catalog case so the normal four-platform runner actually executes it. Assert expected sources, aliases, targets and edges independently of the returned lock. Keep large graph-shape permutations mainly in pure environment tests; smaller CLI fixtures cover the distinct route and authority boundaries.

## 9. Work package F — verification, reviews and final delivery

During development run only relevant package/regression suites and configured lint for changed modules. New unit tests target the changed graph/rollback/authority boundaries, not every helper. Keep benchmarks separate from default test execution and retain profile binaries outside the repository.

Before merge:

1. Run the complete environment suite, targeted CLI lifecycle/check/replay suites, appropriate race checks, existing architecture/size lint and formatting.
2. Compare baseline/new behavior on a bounded generated differential corpus: shared/cyclic graphs, insertion-order permutations, multiple candidates, existing pins, targets, conflicts and selected updates. Expected improvements over a baseline resource refusal are recorded separately; equivalent completed searches must keep locks and meaningful error/callback behavior. Keep any reference-engine copy scratch-only, not as a second maintained implementation.
3. Independently review behavior/compatibility, rollback/resource correctness, and CLI/architecture/verification. Fix demonstrated findings minimally and have a different reviewer assess corrections.
4. After implementation, local checks and independent review/fixes, mark the PR ready once so the existing policy starts full package-manager E2E on Linux, Windows, macOS ARM and macOS Intel, plus applicable repository gates. Audit actual inventories, no hidden test skips, clean source/commit identity, real executable/scanner phase and retained artifacts. Do not manually duplicate this matrix while the PR is still draft.
5. Resolve material GitHub review findings, verify exact head/current base integration, merge with expected head SHA, and verify the actual merge tree. A queued/unrun/skipped case is not a pass. CodeRabbit quota or other unavailable external review is not counted as approval.

No full platform matrix is manually repeated after each small increment. Publish each coherent increment immediately; use the existing draft/ready CI policy. The final PR description is the exact-head evidence/merge ledger so recording finished results does not force a source-only CI restart.

## 10. Research on deferred Windows environments

Microsoft documents account-free local Cloud Files registration and placeholder creation. A focused future native fixture could create one real unhydrated placeholder and inspect its actual state; documentation does not establish that the existing hosted runner has a ready cloud filter. Real DEDUP additionally needs the server feature, an eligible dedicated data volume and optimization of real content. This infrastructure is disproportionate to the graph-only production changes here.

Retain the native long-path/held-reader/delete-sharing checks already present. DEDUP and placeholder attribute fixtures remain explicitly synthetic. Do not claim real DEDUP, a cloud-provider volume or OneDrive integration for this PR.

Primary references reviewed:

- [Register a Cloud Files sync root](https://learn.microsoft.com/en-us/windows/win32/api/cfapi/nf-cfapi-cfregistersyncroot)
- [Create native placeholders](https://learn.microsoft.com/en-us/windows/win32/api/cfapi/nf-cfapi-cfcreateplaceholders)
- [Inspect placeholder state](https://learn.microsoft.com/en-us/windows/win32/api/cfapi/nf-cfapi-cfgetplaceholderinfo)
- [Install/enable Data Deduplication](https://learn.microsoft.com/en-us/windows-server/storage/data-deduplication/install-enable)
- [Windows 2025 runner inventory](https://github.com/actions/runner-images/blob/main/images/windows/Windows2025-Readme.md)

## 11. Ownership and publication sequence

- Root: plan, PR/publication, shared integration seams, final evidence and merge.
- Solver worker: active solver/fact preparation/rollback/candidate ownership and critical tests/benchmarks.
- Contraction worker: new private existing-pin traversal and its pure regressions; no concurrent edits to solver-owned files without an explicit handoff.
- CLI worker: declaration index and its consumers/tests; public E2E files owned by one integrator after contracts settle.

Publish the researched plan first. Root then self-reviews and publishes any corrections; three independent plan critics cover algorithmic correctness, compatibility/authority, and scope/E2E/maintainability. Publish their dispositions and corrected plan before production implementation. Each source increment includes its local evidence and is pushed immediately to this same draft PR. Re-review the combined result; do not split this work into unrelated PRs.

## 12. Plan and execution ledger

- [x] Fresh-main inspection and overlap inventory.
- [x] Three independent research lanes, reproduced failures and baseline measurements.
- [x] Initial bounded plan written before production edits.
- [x] Initial plan published in [draft PR #697](https://github.com/777genius/universal-agent-plugins/pull/697).
- [x] Root self-review completed; corrections published before independent criticism.
- [x] Three independent plan critics; findings addressed before implementation.
- [x] A: candidate ownership and prepared graph facts.
- [x] B: incremental active resolution and rollback.
- [x] C: verified existing-pin closure.
- [x] D: indexed declaration verification.
- [x] E: public-command lifecycle integration/regressions.
- [ ] F: independent combined-code review/fixes, final native E2E and authorized merge.

Final evidence must distinguish observations from hypotheses, local fixtures from native execution, fresh solving from exact replay, and pre-publication preservation from existing post-publication partial results.

### Root self-review disposition

The initial plan was published as `ad97755797b20a39e2b59f448ec3486cc8372b8d` before any production edit. Self-review tightened the edge-index contract to preserve multiplicity and selector evidence, distinguished error identity from unspecified map iteration order, and identified the ignored cold-replay cleanup result for focused reproduction and repair. The existing public-command routing, old-root proof limitation, exact immutable cold acquisition, removed-package exclusion and unchanged search ceilings were checked against the source. No new API, policy layer, mandatory workflow, or unrelated platform scope was added.

### Independent plan review disposition

Three separate critics used the strongest requested model/effort and inspected the current source. They found no need for another framework, workflow, PR split or expanded platform scope. Root accepted the following concrete corrections before production implementation:

| Review | Finding | Disposition |
| --- | --- | --- |
| Algorithm/rollback | P1: first-discovery incremental state can change the representative selector passed to a source. | Preserve current root/BFS precedence where observable; use a small ambiguous-only lookup, with callback-argument regression and mixed-selector measurements. |
| Algorithm/rollback | P1: historical traversal and fallback could accidentally inherit fresh ceilings or leak speculative state. | Separate cancellation-aware history proof from all four fresh budgets; preflight every root, discard branch state on fallback, retain only validated immutable facts. |
| Algorithm/rollback | P2: direct roots must not consume the dependency-edge allowance. | Explicitly preserve edge-count identity and boundary regressions. |
| Compatibility/authority | P1: set containment could broaden the existing eligibility exemption for valid unordered historical targets. | Reuse the current exemption; nonexempt histories follow the old solver, preserving policy callbacks. |
| Both graph critics | P2: strict identity comparison could reject old absent-Version hydration. | Permit only existing version enrichment; keep stable aliases and every other immutable identity field. |
| Compatibility/authority | P2: cold snapshot cleanup is discarded at multiple retained-materialization owners. | Reproduced success and pin-mismatch cases; name the bounded repair chain and preserve terminal cleanup precedence over policy refusal. |
| Scope/E2E | P2: import wording could expand ordinary transfer authority. | Distinguish plain offline transfer, import with installation, and local-only check. |
| Scope/E2E | P2: requiring the native matrix before ready contradicts draft CI policy. | Complete local review, mark ready once, then use existing native gates before merge. |
| Scope/E2E | P2: selector-distinct edges need explicit index acceptance. | Test both edge orders, duplicate evidence, legitimate shared parents and missing/empty cases in the common comparator. |

Scratch probes reproduced representative-selector changes, unordered-target eligibility behavior, and discarded cold cleanup failures against unchanged production code. Root also recorded a reproducible 96-seed, 564-resolution differential baseline spanning fresh/permuted graphs, replay, selected updates, contraction and changed selectors. Source callbacks and final lock hashes are retained for comparison; no reference engine is added to the repository. The existing description cache is currently used only for one recorded pin per source; any reuse for candidates must preserve the selected package identity rather than substitute a cached whole package based only on tree equality.

### Implementation evidence and first code-review corrections

Candidate ownership was published as `0ec99bd86540cf04c750057c73893b10a102b0ae`, declaration indexing as `52456ca9326ce095caef818adf3859099374ad2c`, and snapshot cleanup preservation as `490e36d3842626ad3afc4303ae98c375c2bac0bc`. Each coherent increment was published immediately to the same draft PR.

The first independent authority code review reproduced two defects in the initial historical traversal before its publication: describing an already-incompatible old pin could block replacement on corrupt bytes or silently retain an older hydrated version; FIFO root traversal could read a later root before a newly reached lower source supplied its stricter demand. Both were corrected by checking recorded acceptance before description and by draining selected-node propagation before selecting the next lexical unresolved source. Permanent regressions compare complete locks and callback traces against fresh resolution, including legitimate exact-pin hydration followed by a later compatible demand. Root additionally reproduced and fixed successful `Eligible` materialization discarding its cleanup error; its actual callback regression now verifies terminal cleanup with no persistent publication.

After these corrections, all **564 differential resolutions match baseline lock hashes, errors and complete callback traces**. This includes the successful historical cases whose description ordering initially differed. The complete environment suite passes (3.024 s), configured environment lint reports zero issues, and the updated existing graph compatibility catalog case passes (11.456 s). That catalog case includes real cached 1,026-to-1,025 contraction, absent removed bytes, dry run, offline verification, stable repeat, saved removal, edited-manifest lock/sync, strict stale install, cold retained-only acquisition, corrupt retained cache and cancellation. CLI focused/race checks and its existing changed-line and architecture lint gates pass. These are local results; native platform qualification is still pending.

Equivalent same-host graph measurements, with unchanged fresh limits:

| Shape | Baseline elapsed | New elapsed | Baseline allocated bytes | New allocated bytes | New work events |
| --- | ---: | ---: | ---: | ---: | ---: |
| 1,024 independent roots | 4.119 s | 102.29 ms | 945,663,792 | 7,610,792 | 528,896 |
| 1,024-node chain | 2.583 s | 108.13 ms | 558,716,272 | 5,819,256 | 532,988 |
| 1,024-node fanout | 4.944 s | 110.13 ms | 1,051,976,624 | 5,682,592 | 532,988 |
| 1,024-node cycle | 2.699 s | 110.53 ms | 558,717,000 | 5,931,224 | 532,990 |
| 128-node / 8,128-edge dense DAG | 2.621 s | 47.10 ms | 253,218,944 | 15,590,096 | 41,280 |
| 1,002-node high fan-in | 6.636 s | 99.21 ms | 1,151,451,912 | 7,806,640 | 513,511 |

Separately, the formerly failing 1,009-node/8,008-edge shared graph now completes in 104.50 ms, allocates 15,122,752 bytes and consumes 544,605 work events / 1,010 states. Its old 27.853 s / 2,602,135,792-byte result was an early resource refusal, so it is not an equivalent completed-work speedup ratio. Repeated new shared runs were about 114 ms. The large shared case still performs 508,537 `Eligible` callbacks: source-policy behavior was preserved, and pure graph timings do not predict network or installation speedups. A mixed-selector shared fixture exercises exceptional representative traversal without a timing gate.

The complete declaration-index operation on 1,008 packages/8,000 edges measured 6.976 ms versus 57.716 ms for the old comparator. Its additional operation-local index increases allocation from 456,542 to 1,367,787 bytes. This is an explicit linear-memory tradeoff, not a claim of reduced allocation in every layer. Raw profiles and reference comparison code remain scratch-only; production includes representative benchmarks and semantic regressions, without a second maintained engine or elapsed-time assertions.

Final review dispositions, exact-head native artifacts/check results and actual merge identity will be recorded in the PR description, avoiding a documentation-only source commit after final qualification.
