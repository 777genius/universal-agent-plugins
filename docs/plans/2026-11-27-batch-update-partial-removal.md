# Batch Update and Partial Removal Follow-up

Status: v1 — planning only; implementation has not started.
Date: 2026-11-27
Baseline: `main@472d0b157f21b1b5c2a5b716ccc8f49577e7262f`
Delivery: one Draft PR; publish every commit immediately.
Prior delivery: #1118 is merged and qualified. This plan addresses three separate remaining branches.

## Goal

Keep mass updates consistent with targeted updates, preserve actionable failure causes,
and describe partial removal accurately. Reuse existing installer authorities and presenters.
A smaller change is preferable when an existing path already satisfies the requirement.

The first candidate is a strong source-level hypothesis, not an executed defect.
The other two have source-level evidence; all corrections require meaningful public-path tests.

## Scope A — current Directory release and an outdated projection

Observed path: `updateAllSession.classifyInstallation` calls `skipDirectoryStatus`
before `prepareUpdateMany`. Directory status `current` only establishes that no later
eligible immutable release exists. Ordinary update can reload the recorded release and
refresh installer-rendered components at the same package revision.

1. Reuse the existing signed Directory fixture and command-projection rewind helper.
2. Through the public command, install an imperative Directory package with an installer-rendered command.
3. Represent an intact installation produced by older rendering rules: retained package identity
   and ownership digests agree with the older delivered tree. Do not confuse this with foreign drift.
4. Prove that targeted update refreshes it, and that `update --all` currently fails to do so.
5. Only if that public regression fails as predicted, route eligible current installations through
   the existing preparation/no-change decision and same-revision refresh.
6. Keep blocked/unknown Directory results refusing, select only recorded binding owners,
   and preserve signed source eligibility, immutable identity, security authorization,
   scoped ownership and saved-dependency authority.

Controls: healthy current package; actual refresh; repeat after refresh; dry-run;
revoked/blocked/unknown source; unrelated binding and foreign bytes preserved.
Do not add a projection detector, resolver, source fallback, migration or automatic repair.
Saved dependencies retain their existing pin-change contract.

Likely production owner: `cli/internal/agentpluginscli/update_all.go`.
Existing references: `update_commands_test.go`, `update_multi.go`,
`usecase/projection_refresh.go`, existing Directory fixtures.

## Scope B — retain batch-preflight causes and output failures

Observed path: `preflightAll` reduces original preparation/check failures to display strings,
discards `renderUpdateAll` errors and returns a fresh generic error.

Retain original typed causes where they exist, keep the existing preflight summary,
and join the rendering failure through existing error/output conventions.
Do not invent typed causes for policy descriptions which currently have no error value.
Keep one compatible JSON document and the existing nonzero exit behavior.
Keep human output safe; do not expose extra raw provider details.

Public-path tests must distinguish an original sentinel or filesystem failure,
a writer failure, both together, and cancellation during preflight. Confirm no apply
occurs and preserve installed/foreign bytes. Successful preflight remains unchanged.
Cover saved and imperative producers with the existing fixtures, avoiding a second error model.

Likely owners: `update_all.go` and, only if required for a cohesive helper,
`update_all_diagnostics.go`.

## Scope C — truthful human partial-removal results

Observed path: grouped native deactivation may complete for target A and fail for B.
The group returns before managed cleanup, but A's `external_completed` status renders
as simply `removed`. The overall failure and original error are already retained.

Interpret human per-target wording together with the group outcome so users can distinguish
removal from the client from completed managed cleanup. Reuse existing partial-removal language.
Provide safe contextual recovery guidance only if the existing result supplies authority;
do not invent a selector, command, force flag or automatic retry.

Keep transaction behavior, target phases, JSON schema and success-count semantics unchanged.
Do not delete retained materialization or try to complete a failed native operation in presentation.
Keep ordinary success, intentionally retained plugin data, blocked and unattempted results clear.

Use a public grouped-removal scenario with a controlled second-target failure.
Verify the first target's reported stage, retained managed bytes, unrelated bytes,
overall nonzero failure and original typed cause. Retry through the existing public path
after removing the injected failure and verify eventual cleanup.
This composed fixture is not proof of vendor authentication or real-model behavior.

Likely owners: `remove_multi_presentation.go`, and only if necessary
`remove_multi_apply.go`. No use-case or transaction-engine redesign is intended.

## Explicit exclusions and overlap

No release/tag/npm publication, website/landing work, new client support, flags,
background service, resolver, lock protocol, state store, retry engine or broad refactoring.
No public JSON/status-schema changes, ownership adoption, source-security waiver,
generated-file hand edits or new lint suppressions.

Current open PRs were checked at their actual heads:
- #1078: live/native audit fixtures and workflows.
- #961 and #1106/#1108/#1110: landing/dependencies, with #961 also owning native audit fixtures.
- #1112/#1113: runner routing in the polyglot workflow.
- #1114: deferred Skill resources/MCP probe plan.
- #530: frozen VS Code and associated harness/scanner work.
- #832: held Codex environment-name security investigation.

None currently changes the proposed update/removal presentation files.
Recheck before implementation and before merge; preserve independent work.
Do not reopen deferred/frozen work or absorb unrelated failures into this PR.

## Plan reviews, implementation and code reviews

1. Publish v1 in the single Draft PR.
2. R1: independent safety/behavior and scope/test critiques; record findings and revise v2.
3. R2: review v2 against real source/fixtures, resolve findings and publish v3 before implementation.
4. Add meaningful regression oracles first; establish executed RED evidence for confirmed defects.
5. Implement only bounded corrections justified by those oracles.
6. Review implementation independently for scope, source/ownership safety, truthful results,
   public compatibility and the strength of the tests. Fix concrete findings.
7. Conduct another final-source review; repeat if blocking findings remain.
8. Finish genuine final-source checks, mark Ready, use ordinary expected-head merge,
   and verify the actual merged source and its required checks.

A source review, a test declaration or an unexecuted workflow is not runtime evidence.
The local executor is currently unavailable; use the existing normal GitHub test lanes
rather than adding a new workflow or weakening a gate. If execution returns, use the
standard local commands without redundant reruns.

## Verification and acceptance

Follow `AGENTS.md`, `docs/ARCHITECTURE.md` and `CONTRIBUTING.md`.
Required commands are `make lint` and `make test-core`, plus affected lifecycle,
rollback and cross-platform checks from the existing workflows.

Register the new public-command regression scenarios in the existing package-manager
evolution map/catalog. Do not add a runner, evidence schema or parallel test framework.
Retain actual signed Directory/source verification and source/foreign-byte controls.
Use focused unit tests only for a necessary formatting boundary not covered by command scenarios.

Final PR: normal Core/Required gates, full PM E2E on four platforms,
and the applicable existing Native Client Runtime and recovery gates.
Actual merge: full PM E2E on six platforms and Required execution jobs.
Bind results to the exact tested commit/tree. Inspect actual case execution and artifacts.
Do not promote composed fixture evidence into vendor-native or released-binary proof.

Queued/running checks require normal waiting. Skipped, cancelled, blocked or
evidence-only checks are not successes. Avoid duplicate dispatches, gratuitous reruns
and branch churn; use waiting time for independent review and evidence inspection.

Stop when all three dispositions are resolved, final code has no blocking review findings,
the agreed checks pass, the PR is merged and actual-merge qualification is verified.
A nonreproducing A candidate is closed with the concrete evidence and no speculative fix.
Publication remains a separate task.

## Estimates and confidence

| Item | Estimate |
| --- | --- |
| Production changes | 50–120 added/substantially changed lines, usually 2–4 existing files |
| Regression tests | 200–400 lines, reuse existing fixtures |
| Catalog and plan | Existing catalog registrations and this document only |
| Complexity | 3/10; no new architecture or authority |
| Reliability benefit | 7/10 for the bounded consistency/error/partial-result cases |
| Current diagnostic confidence | A 8/10, B 9/10, C 9/10; runtime confirmation still required |
| Completion confidence | 8/10, conditional on executed tests and normal qualification |

These are planning estimates, not line-count targets or claims of successful execution.
