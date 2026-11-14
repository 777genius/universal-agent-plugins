# Go legacy debt: bounded refactor program

Date: 2026-11-06
Status: reviewed plan; implementation must remain Draft until exact-HEAD evidence.
Initial base: main 9444d2fae2b9c0ab3438f309a3f89663ab95caa2.
Owner scope: reduce proven accumulated legacy without deleting uncertain capabilities. This is not a feature project, migration, release, or rewrite.

## 1. Ground truth, not an assumption that all old code is bad

The repository is a Go multi-module system. Current docs/ARCHITECTURE.md and AGENTS.md already define domain, ports, use cases, Go client adapters, CLI composition roots, strict dependency direction and executable archtests. The latest merged architecture work includes #681, #682, #683, #700, #706 and #713. Preserve it instead of starting another Clean Architecture implementation.

The tree at the initial base contains 3,839 Go files including tests, with roughly 20.8 MB of Go sources. This is an inventory size, not a tech-debt estimate.

Concrete remaining debt candidates, ranked by verified code/documentation:

| Candidate | Observation | Priority | Risk /10 | Action |
| --- | --- | --- | --- | --- |
| Directory transaction kernel | transaction/kernel.go is 788 physical lines and has six large exported/private methods for apply, group apply, remove, group remove, recover and pending owner assembly; it is grandfathered in the shrink-only lint baseline | P0 | 9 | Refactor by responsibility without changing physical transaction semantics |
| Transaction group validation | ApplyDirectoryGroup validates identity, IDs, paths and state before any effects, then validates a second time as effects occur; duplication may be deliberate defense in depth | P0 (analysis) | 10 | Keep both checkpoints, extract reusable pure validation only if operation order remains exactly equivalent |
| State/journal recovery | Recover and pendingOwners share the kernel file with mutating apply/remove operations | P1 | 10 | Separate recover/pending owner orchestration with unchanged journal and refusal contract |
| Management adapter | docs/ARCHITECTURE.md explicitly records that the installer-facing ManagementBackend still depends on CLI App and dependency preparation even after #700 extraction | P1 candidate | 8 | Characterize, consider only a demonstrably smaller seam, independent PR after transaction checks |
| CLI dependency and Directory flows | Large CLI files exist, but PR #725 currently changes deps_resolver, deps_sync, source, directory read and related paths | Excluded until ownership clears | 9 | Do not create competing edits |
| Client-specific lifecycle and native E2E | #698, #710, #724, #726 and related active changes own current capability/native lanes | Excluded | 10 | No edits, no changed baselines, no speculative deletion |
| Historical plugin.yaml authoring and retired command exposure | AGENTS.md explicitly requires preserving useful implementation, tests, dependencies and records without deletion authorization | Excluded | 10 | Inventory only; do not delete |

No assumed savings from filename counts, raw file size or a label containing "legacy". Prove behavior and dependency ownership before refactoring.

## 2. Non-negotiable scope and architecture

- Go-native modularity only. Do not add Get Modular, TypeScript framework, DDD framework, plugin container, DI framework, event bus or any new runtime system.
- Preserve existing package/module boundaries. Keep domain free from external I/O. Use existing ports for actual I/O, client capabilities under clients/<id>, clients/all only in explicit executable composition roots. Keep transaction and ownership authorities singular.
- Preserve public CLI flags, JSON, exit codes, manifest/lockfile formats, file paths and file permissions, package identities, receipts, journal schema, transaction replay, cancellation and recovery contracts.
- Never weaken path containment, symlink checks, physical owner profiles, directory state, filesystem atomicity, lock order, verification, refusal or rollback. Fail closed on uncertainty.
- No new public package ABI, migration, feature flags, generated source edits, lint suppressions, behavior-changing "cleanup" or dependency upgrades. No changes to frozen VS Code sources/tests.
- Do not touch active PR-owned source: #698, #710, #720, #724, #725, #726; recheck overlap before each wave and after rebases.
- Keep all historical authoring functionality that may still be used. No deletion without a separate inventory and explicit owner approval.
- Existing lint baseline is shrink-only: every touched grandfathered production source must not grow in token-bearing code lines. No file move that grants a new exception. New production Go files must satisfy 500 effective lines/file, 60 lines/function, 40 statements/function, cyclomatic complexity 20 and cognitive complexity 25.
- Reviewable changes are organized into coherent PRs: no mixing a transaction refactor with an unrelated website, catalog, or client rollout.
- The requested 10k-20k changed-code-line scale is a program estimate/upper planning envelope, **not** an acceptance check. Only justified code changes count; stop early if real debt ends. A huge cosmetic diff is a regression.

## 3. Delivery design and order

### Wave 0 — baselines and characterization (this draft before production edits)

1. Pin initial main and check all open overlapping PRs.
2. Read AGENTS.md, docs/ARCHITECTURE.md, CONTRIBUTING.md and applicable transaction/code ownership docs; do not infer current architecture from archived rewrites.
3. Map transaction ordering: owner acquisition, admission, state Load, duplicate/physical-path preflight, directory Apply, Verify, pending proof, durable state decision, directory Commit, final receipt Save, rollback and recovery. For groups, record preflight-before-first-effect and per-item recheck separately.
4. Identify exact safety/compatibility tests already in transaction and integration test suites. Add only missing high-risk characterization, no redundant snapshot tests.
5. Record baseline exceptions from .golangci.yml so the final diff can demonstrate shrinkage.

Exit: no concurrency, journal or state format changes are hidden in "mechanical" moves.

### Wave 1 — core transaction responsibility boundaries (FIRST IMPLEMENTATION LANE)

Owned Go paths: install/integrationctl/agentplugins/transaction/kernel.go plus new cohesive files under the *same* transaction package, and focused transaction tests. Only narrow changes to existing test fixtures when actual behavior is being characterized.

1. Extract the private group identity/path preflight as a small pure helper which can run on the complete group *before* any directory effects. Keep per-step validation in the apply loop, because an earlier mutation can change state and because the double checkpoint has an explicit fail-closed purpose. Any deduplication must preserve the original errors and first-failure ordering; otherwise preserve both loops.
2. Separate directory apply, directory remove and recovery/owner gathering into cohesive, named implementation files without moving the security, state or journal authority to a new package. Preserve exported Kernel methods and public return contracts.
3. Break oversized functions at existing phase boundaries (admission, planning, physical Apply/Verify, decision, finalization) **only** where extracted helpers are cohesive and testable. The call sequence, cancellation boundaries and rollback context must remain identical. Do not add a new generic transaction engine.
4. Shrink kernel.go and narrow/remove its existing lint exclusions only when a full configured lint run across supported operating systems proves the rule still passes. Do not rename exempt source to buy an amnesty.
5. Regression tests: empty/duplicate group, duplicate operation ID and physical target, missing owner/state, failing Apply, Verify, VerifyPending, Save, Commit, failed Rollback, final receipt Save and recovery after interrupted commit. Tests must distinguish Unchanged, RolledBack, Unknown and Committed; test result is not just "error occurred".
6. Run exact-HEAD core tests, race checks where feasible, architecture guard, lint gate and CI-native transaction E2E before accepting this lane.

Explicit non-goals: no changes to Directory v1/v2, client discovery, native activation, state-v2 schema, existing receipt version or lock implementation.

### Wave 2 — recovery and ownership clarity (same PR only if it stays one transaction family)

1. Make the pending-owner/journal-recovery logic auditable in focused helpers, retaining authoritative owner scopes and stable error handling.
2. Avoid map-order nondeterminism changes in public receipts or diagnostics. Do not "sort for neatness" where ordering is contract-sensitive.
3. Prove journal consistency under missing receipts, ambiguous ownership, mismatched authorities and incomplete or pending native effects.
4. If recovery isolation requires cross-package interface churn or new state concepts, defer rather than broaden.
5. Re-run full Wave 1 tests with race/cross-OS gate.

### Wave 3 — residual management/CLI seam (SEPARATE DRAFT PR after transaction wave)

1. Reassess actual ManagementBackend coupling to App and shared dependency preparation against currently merged management code, not against old plans. It is a documented remaining dependency, not permission to build another application.
2. Identify only business decisions incorrectly held in CLI. Use existing management Application/backend contracts and inject the minimum real port at the existing composition root.
3. Keep state/HTTP layers as implemented in #700. No alternative persistence, no new HTTP server and no ABI additions.
4. Prove management restart, accepted/cancelled/recovered operations, private handles, failure persistence, explicit availability and target scope E2E.
5. If findings do not justify the abstraction, close this wave with a documented no-change decision.

### Wave 4 — optional tight follow-ups, only after review of Wave 1–3

Small historical usecase or provider methods may qualify **only** with a specific forbidden dependency, security issue, duplicated authority, or no-growth lint debt and no overlapping PR. Create a separate draft PR and plan addendum with a bounded file list first. Never use this wave merely to reach 10,000 lines.

## 4. Review process and change-control

Reviews must assess *behavior* rather than approve line movement. Each wave receives independent review angles, and any material changes trigger another pass:

1. **Security/transaction critic:** Can any new error path escape rollback, ownership checks, physical journal verification, lock release, or symlink containment? Is recovery fail-closed?
2. **Architecture/SOLID/DDD critic:** Does any dependency now point outward from domain/usecase, is any domain invariant duplicated, or has a false abstraction/service locator appeared? Do new names match actual Go responsibilities?
3. **Compatibility/E2E critic:** Has an existing client, CLI JSON/exit outcome, receipt, installation path, OS behavior, authoring capability or native proof changed? Are test skips mistakenly called passes?
4. **Simplification critic:** Can any new helper/port/file be removed without loss of cohesion or tested contract? Does a function split only mask complexity?

Review round 1 (plan, 2026-11-06):
- Architecture objection: initial idea of an organization-wide modular framework or broad package migration would duplicate existing Go boundaries; removed. Get Modular explicitly excluded by owner.
- Safety objection: apparent double validation in directory groups has a pre-effect versus within-effect role; the plan forbids deleting either checkpoint without proof.
- Scope objection: active Directory, Devin, ChatGPT Desktop, group-update, SkillSpector and live E2E PRs would conflict with a broad installer refactor; paths excluded.
- Compatibility objection: existing plugin.yaml code and frozen VS Code must be preserved; no deletion or automated API modernization.

Review round 2 (plan adversarial revision):
- Potential anti-pattern: splitting a legacy-exempt 788-line file into new oversized exempted files. Added hard new-file lint budgets and baseline shrink-only requirement.
- Potential anti-pattern: a generic transaction wrapper to deduplicate Apply/Remove, hiding differing rollback/committed semantics. Rejected; only focused pure/preflight and phase helpers qualify.
- Risk: native E2E requires credentials/installed clients and may not run in every environment. Require exact-HEAD external CI evidence and explicitly report any inaccessible lane rather than declare success.
- Risk: attempting all four waves in a single 20k-line PR defeats review. Wave 3 and Wave 4 must be separate PRs and are optional.

Review round 3 (implementation admission — to record after CI):
- Independently recheck source diff, live PR overlap, baseline strictness, paths/ownership/refusal, and test coverage before every code commit.
- Do not mark "independent reviewers completed" unless distinct independent review evidence exists. Human/automated review comments require explicit resolution and rerun.

## 5. Verification and required proof

For every substantive source commit:
- Commit, push immediately to the *same* draft PR. No silent local-only batch of fixes.
- Check gofmt, go vet and relevant Go package compilation.
- Run make lint and make test-core (all configured module targets, no new suppressions), go test -count=1 in affected module, race tests on affected transaction/management packages where supported.
- Run make test-install-compat and make test-polyglot-smoke when touching install/runtime behavior.
- Run python scripts/package-manager-e2e.py if source acquisition, lock, saved install or dependency flows actually change.
- Compare public outputs against unchanged golden fixtures; intentional changes to public contracts require fresh owner authorization rather than implicit golden regeneration.
- Hosted CI: verify each *latest HEAD* job result, Linux/Windows/macOS where applicable, relevant real native E2E, and all actual check status. Queued, skipped, cancelled and evidence-only runs are NOT passes.
- For recovery or lifecycle changes, negative test cases and post-restart proof are mandatory; no merge based only on an in-process unit test.
- Final reviews: multiple rounds until no substantive findings remain; fix only verified in-scope issues, then rerun checks.
- Merge gate: independently reviewed scoped diff, all required exact-HEAD checks green, no conflict with concurrently merged work. Keep Draft otherwise. Do not publish a release as part of this program.

## 6. Risk register and rollback

| Risk | Level | Containment |
| --- | --- | --- |
| Ownership/symlink or cross-profile destructive effect | Critical | No policy/code weakening; same validated adapters, refusal-path tests |
| Journal/state decision straddles physical commit | Critical | Exact phase order, crash/recovery tests, no schema change |
| Group preflight accidentally moves behind first effect | Critical | Preflight characterization plus explicit final-code review |
| Concurrent PRs changing same transaction/call graph | High | File-level overlap check and rebase before each wave |
| Linux-only tests miss Windows or macOS runtime semantics | High | Hosted multi-OS workflows, no assertion of pass for unrun suites |
| New abstractions and DDD boilerplate | Medium | Reuse Go package/contracts; simplification critic; no new DI framework |
| Change size hides defects | High | Independent scoped PRs, immediate pushes, per-commit gates |

Rollback is a normal revert of the specific PR, never rewriting receipts, reinstalling clients or altering historical state formats. Retain old behavior until the new code is proven equivalent.

## 7. Completion criteria and truthful reporting

Completion requires real code debt eliminated, strict reduction of relevant legacy exceptions, verified equivalent behavior (including negative/restart cases), reviewed and CI-qualified exact HEAD, and no open substantive findings. Count changed Go source lines separately from tests/docs/generated files and report net code and file changes. A plan-only PR is **not** a completed refactor. Unrun tests, unavailable native clients, missing reviewers or an open draft are reported as outstanding rather than PASS.

Progress is recorded in the PR description and this plan after each evidence checkpoint. The owner requested maximum autonomy but not arbitrary risk, scope enlargement, release, or generated-line inflation.
