# Update and Existing Installation Reliability Plan

Status: v1 — initial proposal; implementation and new regressions follow two plan reviews.
Date: 2026-11-26
Repository: `777genius/universal-agent-plugins`
Baseline: `main` at `4903e6794f29ce7f2c39bfd054bf37c5bf654f79`
Delivery: one new Draft PR, every commit published immediately.

## Goal

Make repeated installation and updates reliable and easy to understand through the existing CLI.
Prefer truthful no-op results, a clear existing update route, actionable refusals, and accurate partial-success summaries.
Preserve merged behavior from #1104, #1115, and #1116.
The matrix below contains hypotheses to investigate, not an assertion that each case is defective.

## Plan review sequence

1. Publish v1 without production or test changes.
2. Review R1 against the current source; record dispositions and publish v2.
3. Independently review v2 as R2; resolve findings and publish v3.
4. Only then reproduce selected gaps and implement evidence-backed corrections.
5. Keep final implementation review separate from these planning reviews.

Read-only source, existing-test and documentation inspection supports the plan reviews.
No need to wait for CI while improving a documentation-only plan.
Do not use an unavailable local executor as a reason to stop independent source/review work.

## Existing authorities to preserve

- #1104: safe diagnosis of an existing installation and its ownership.
- #1115: truthful install outcomes and console presentation.
- #1116: batch security authorization before applicable mutations.
- Existing source identity, receipts, persistent data, JSON/schema and exit semantics.
- Owned versus foreign files, path/symlink containment, locks, rollback and recovery.
- Current transaction boundaries; this work does not introduce a global batch transaction.
- Already-correct paths use existing evidence rather than duplicate implementation or tests.

## Initial behavior matrix

These acceptance targets remain subject to the existing documented command contract.

| Case | Question to establish | Safe and understandable outcome |
| --- | --- | --- |
| Repeat add, healthy installation, same source | Is existing idempotence and reporting sufficient? | Truthful no-op; no unnecessary native or state mutations |
| Repeat add, changed source or requested version | Does the diagnostic name the right existing command? | Preserve identity checks and offer a scoped explicit update route where valid |
| Targeted update, unchanged package | Does unchanged describe the actual checked state? | No applied claim for a no-op; preserve existing validation |
| Targeted update, changed package | Does the public route retain all transition checks? | Existing update pipeline, actual outcome and compatible output |
| Update all, changed and unchanged entries | Are counts and affected identities truthful? | Distinguish applied and unchanged packages |
| Update all, blocked and eligible entries | Are preflight and later failure semantics clear? | Existing authorization/continuation policy; accurate per-entry outcome |
| Concurrent add/update/repair/remove | Do existing locks and revalidation cover the public path? | Serialize or refuse through the existing authority; actionable retry guidance |
| Missing or partial owned installation | Does the current diagnosis select a safe next step? | Preserve receipts/data and the supported recovery or repair route |
| Owned-file drift | Are refusal and guidance accurate? | Keep established overwrite/ownership policy |
| Foreign files or conflicting ownership | Can a repeat command affect somebody else's content? | Refuse safely without ownership adoption or foreign cleanup |
| Cancellation or interrupted earlier operation | Is the recorded state and suggested action truthful? | Preserve recovery evidence and supported retry/recovery semantics |
| Preparation/network failure | Does output imply an apply that never happened? | Accurate refusal/failure, with no claimed apply |
| Failure after some applies | Do counters and next steps match durable effects? | Applied, failed and not-attempted work remain distinguishable |
| Human and JSON outputs | Can a user act and automation interpret the same result? | Concise action/result/reason/next step; one compatible JSON document |

## Code and ownership boundaries

Inspect the smallest public CLI path through add/update handling, presentation, current facade/use cases, and existing locks/recovery.
Trace actual callers; a private-helper difference without a public-path failure is not a defect.
Map current tests to these behaviors before adding new coverage.
Record the exact affected files before implementation.

Inspect open #1087, #1023 and #1117 for overlap before changing their owned behavior.
Do not merge, close, replace or absorb those PRs.
If a slice depends on unresolved work, continue independent slices and establish the dependency explicitly.

## Explicit exclusions

- No new installer, resolver, state engine, queue, daemon, retry protocol or speculative interface.
- No new command flags, configuration modes, automatic force/repair, lock deletion or ownership adoption.
- No removal of important existing logic to make an implementation or test simpler.
- No blanket automatic retry of failed or interrupted native effects.
- No VS Code, held #1114 probes, release, website, or unrelated client/source work.
- No broad refactor, generated-file hand edits, expanded lint exemptions or unrelated cleanup.
- Package-manager comparisons inform clear UX; they do not import different overwrite or resolution policies.

## Evidence-first implementation

For each candidate, record command, fixture/state, observed result, expected contract and current coverage.
Classify it as already correct, reproduced defect, demonstrated missing safe UX, or unresolved hypothesis.
Change production only for the two middle categories.
Use existing fixtures, parsers, errors, renderers and extension points.
Add only regression oracles that distinguish the bad behavior from the intended public result.

1. Finish v1 -> R1 -> v2 -> R2 -> v3 with written review dispositions.
2. Confirm main and overlapping work; choose an isolated branch/worktree.
3. Reproduce bounded candidates in disposable roots, without live user profiles.
4. Reduce the implementation scope and estimates to the evidence.
5. Add meaningful regressions and minimal fixes; publish every commit immediately.
6. Run affected public CLI journeys and update only the necessary maintainer/user guidance.
7. Review implementation independently; fix concrete findings.
8. Complete required gates on final source, ordinary expected-head merge, then actual merged-source acceptance.

## Verification

Use `make lint` and `make test-core`, plus applicable affected native, rollback and cross-platform checks.
For install/dependency changes, use the existing package-manager E2E catalog and runner.
Require full four-platform qualification for the final PR and full six-platform qualification for the actual merge.
Do not replace product execution with a documentation/evidence-only lane.
Queued, running, cancelled or skipped jobs are not successful product tests.
Wait normally for selected checks; no duplicate dispatch, unnecessary rerun or constraint bypass.

Verify human output and a single JSON result where behavior changes.
Compare refusal/no-op reports with files, receipts, persistent data, native effects and state.
Compare partial-failure summaries with actual durable effects and recovery state.
For a concurrency correction, test the real shared lock/revalidation boundary across relevant public operations.
Retain security preflight before applicable mutations and existing cold real-scanner qualification.
Use no real vendor subscriptions or production credentials for fixtures.
Follow current Go function/file/complexity gates without growing suppressions.

## Acceptance and stopping expansion

Every selected defect needs a public-path reproduction, minimal correction and meaningful passing verification.
Every matrix row gets a disposition; already-covered behavior does not require another implementation.
Messages and counters accurately describe the work actually completed and the safe next action.
Ownership, security, data preservation and compatibility contracts remain intact.
If evidence points to a new architecture or destructive recovery policy, keep that change out and continue safe work.
Ask for a contract decision only if the existing code, docs and user authorization cannot resolve it.
No speculative improvement is required merely to increase the patch size.

## Provisional estimates

These estimates will be revised after source review and reproduction.

| Dimension | Initial estimate |
| --- | --- |
| Production changes | 80–250 lines, only where evidence justifies them |
| Meaningful regression coverage | 250–500 lines; less when existing coverage suffices |
| Supporting documentation | 60–100 lines beyond this plan, only if useful |
| Complexity | 5/10; concurrency/recovery findings may change the estimate |
| Reliability improvement potential | 7/10 if candidate gaps reproduce |
| Current diagnostic confidence | 4/10 before bounded audit |
| Target completion confidence | At least 8/10 after reproduction, review and applicable E2E |

Reduce estimates for already-correct behavior. Review any material scope increase before implementation.
