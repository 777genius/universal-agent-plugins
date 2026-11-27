# Batch Update and Partial Removal Follow-up

Status: v3 — two independent critique/revision rounds completed; approved for bounded implementation. Code and test implementation has not started.
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

1. Reuse the existing rollout/command-projection helpers for a composed behavior oracle.
   The rollout fixture is preverified and does not itself check a signature. The final A
   scenario must execute the production Directory verifier/client with synthetic test-only
   Ed25519 trust and the existing signing/transport pattern; do not create a general signing
   framework. Real Git transport and vendor-native proofs remain separate existing gates.
2. Through the public command, install an imperative Directory package with an installer-rendered command
   and explicit `--no-save`. Build the command before snapshot/digest/signing, or consistently
   rebuild the entire test-only release/evidence tuple before install. Freeze that source afterward.
3. Represent an intact installation produced by older rendering rules: retained package identity
   and ownership digests agree with the older delivered tree. Do not confuse this with foreign drift.
4. Use independent equivalent rewound fixtures for targeted and batch controls so the targeted
   refresh cannot erase the batch defect. Retain only the installed release, with no newer
   eligible release. Prove targeted refresh and the intended batch failure, not an integrity refusal.
5. Only if that public regression fails as predicted, route eligible current installations through
   the existing preparation/no-change decision and same-revision refresh.
6. Keep blocked/unknown Directory results refusing, select only recorded binding owners,
   and preserve signed source eligibility, immutable identity, security authorization,
   scoped ownership and saved-dependency authority.

Controls: healthy current package; actual refresh; repeat after refresh; dry-run;
revoked/blocked/unknown source; recorded owners and foreign bytes preserved.
Verify expected generated command bytes, immutable source/release identity, persistent data
and unrelated projection bytes. The rewind helper accepts one installation/binding; seed
foreign sentinels directly or add an unrelated binding only after rewind.
Healthy/repeat assertions use actual applied no-change/mutation facts and preserved managed
bytes/mtime. A command-containing preview may legitimately be planned and require confirmation;
do not require planned=0. Allow existing observation/cache bookkeeping without treating it
as a package change.
Do not add a projection detector, resolver, source fallback, migration or automatic repair.
Saved dependencies retain their existing pin-change contract.

Likely production owner: `cli/internal/agentpluginscli/update_all.go`.
Existing references: `update_commands_test.go`, `update_multi.go`,
`usecase/projection_refresh.go`, existing Directory fixtures.

## Scope B — retain batch-preflight causes and output failures

Observed path: `preflightAll` reduces original preparation/check failures to display strings,
discards `renderUpdateAll` errors and returns a fresh generic error.

Retain original typed causes where they exist, keep the exact existing safe preflight summary,
and retain the rendering failure through existing error/output conventions. Reuse
`installFailureWithCause` for safe outward wording and the existing
`declaredInstallResultError` marker for a batch document already attempted; keep the
original/output causes behind those wrappers. Do not use a raw %w summary that newly prints
provider details.
Do not invent typed causes for policy descriptions which currently have no error value.
Keep one compatible JSON document and the existing nonzero exit behavior.
Keep human output safe; do not expose extra raw provider details.

Public-path tests must distinguish an original sentinel or filesystem failure,
a writer failure, both together, and cancellation during preflight. Confirm no apply
occurs and preserve installed/foreign bytes. Successful preflight remains unchanged.
Cover saved and imperative producers with the existing fixtures, avoiding a second error model.
Cancellation must be triggered inside a witnessed preflight dependency, not before command setup.
A classification-sensitive retained cause must not make the real entrypoint policy emit another
JSON envelope: verify ShouldWriteAgentFailure is false, one JSON value followed by EOF, and no
second output attempt after a partial write. Keep the same safe error text even with a private
canary in the cause. Pure writer failure on successful rendering remains its existing branch.

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

Use a public grouped-removal scenario with a controlled second-target failure whose native
outcome positively proves unchanged effects (or an existing settled outcome). Delegate previews
and normal successful calls to the existing activator. An arbitrary uncertain native failure
must retain its recovery refusal; never clear attempt receipts or edit state to force retry.
Verify the first target's reported stage, retained managed bytes, unrelated bytes,
overall nonzero failure and original typed cause. Retry through the existing public path
after removing the injected failure and verify eventual cleanup.
Use independent equivalent fixtures for human and JSON assertions; preserve the exact JSON
status/count/envelope contract despite improved human wording. Distinguish intentionally retained
plugin data from managed package materialization.

Human wording controls must distinguish these cases:
- Completed removal: removed; existing retained-plugin-data notices remain separate.
- A externally completed and B failed before cleanup: removed from the client; managed cleanup incomplete.
- Managed removal committed but finalization failed: preserve the committed stage and incomplete finalization;
  do not claim that managed files remain.
- Cleanup outcome unknown: retain uncertainty, with no claim of completed cleanup or known retained files.
- Blocked/not applied: preserve that distinction and never call the target removed.

Reuse the existing committed-interruption regression for the finalization control, or add a small
formatter boundary control if that exact phase is not faithfully produced by the public fixture.
Do not change unrelated wording just to satisfy a table. No generic failed-group retention banner.
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
8. After final-source review, mark Ready to enable the existing full PR gates; finish those checks,
   use ordinary expected-head merge, and verify the actual merged source and its required checks.

A source review, a test declaration or an unexecuted workflow is not runtime evidence.
The local executor is currently unavailable; obtain Draft RED through existing Core/test-core
(exact make test-core). Required/test, full PM E2E and native runtime skip Draft, so those are
not Draft evidence. Use their normal Ready transition rather than adding a workflow or weakening a gate. If execution returns, use the
standard local commands without redundant reruns.

## Verification and acceptance

Follow `AGENTS.md`, `docs/ARCHITECTURE.md` and `CONTRIBUTING.md`.
Required commands are `make lint` and `make test-core`, plus affected lifecycle,
rollback and cross-platform checks from the existing workflows.

Register the new public-command regression scenarios in the existing package-manager
evolution map/catalog. Do not add a runner, evidence schema or parallel test framework.
Retain actual signed Directory/source verification and source/foreign-byte controls. The final
A case checks test-only signed bytes through the production reader, while controlled source/client
ports remain explicit. Existing real-source and native checks provide their own bounded evidence;
no one fixture is promoted into production-issuer, network or vendor authentication qualification.
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

## R1 review dispositions

Independent A/B and C reviewers requested revisions before implementation. Accepted:
- distinguish preverified fixtures from production signature verification;
- freeze command/source identities before install, use independent targeted/batch fixtures;
- judge no-op from actual results and managed bytes, allowing existing observation bookkeeping;
- preserve safe error text and one-document ownership while adding typed causes;
- trigger cancellation inside the target preflight boundary;
- distinguish cleanup finalization/uncertainty from known retained materialization;
- inject positively unchanged removal effects for retry, without recovery waivers;
- execute Draft RED through Core, then mark Ready before full qualification.

No source, test, CI or release implementation was added during R1.

## R2 review and final implementation boundary

Both independent reviewers approved v2 after checking the revised source/fixture contracts.
A/B review confirmed that current-release preparation retains existing authority, signed-byte
verification remains honestly bounded, and the safe wrapper/result-reported marker protects
typed causes and single JSON ownership. C review confirmed that only external-completed target
wording needs enclosing outcome context; use the operation status, not a failure count.

v3 records those approvals before any test or production implementation. Keep code review and
executed RED/GREEN evidence separate from these planning approvals. No new scope was added in R2.

## Estimates and confidence

| Item | Estimate |
| --- | --- |
| Production changes | 50–120 added/substantially changed lines, usually 2–4 existing files |
| Regression tests | 350–650 lines after signed-reader, JSON-ownership and removal-stage controls; reuse fixtures |
| Catalog and plan | Existing catalog registrations and this document only |
| Complexity | 3/10; no new architecture or authority |
| Reliability benefit | 7/10 for the bounded consistency/error/partial-result cases |
| Current diagnostic confidence | A 8/10, B 9/10, C 9/10; runtime confirmation still required |
| Completion confidence | 8/10, conditional on executed tests and normal qualification |

These are planning estimates, not line-count targets or claims of successful execution.
