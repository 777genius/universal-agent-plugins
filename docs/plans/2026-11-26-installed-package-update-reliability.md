# Installed-package update reliability

Status: plan v2; R1 resolved in the plan, awaiting independent R2. Implementation remains blocked until R2 findings and plan v3 are committed.

## Goal and baseline

Make installed-package updates refuse unauthorized scanner risk before any batch effects, and, only if reproduced, reject a stale prepared local update before it can overwrite a newer installed revision. Keep this one update-safety change family.

Baseline: `main` at `ce789138a393bffd7e6d001c0e4f8aec05e47773`; work branch `fix/update-safety-20261126`. Read `AGENTS.md`, `docs/ARCHITECTURE.md`, `CONTRIBUTING.md`, and `docs/testing/package-manager-e2e.md` before implementation. Recheck main and overlapping PR file lists before code and before merge. PR #1115 (last observed head `58cbcb338e1e31c0e88b80d8cd00b872dced6736`) owns active install UX/outcome reliability; avoid its shared app/dependency rendering/repair changes and keep this work update-specific. Preserve merged #1104 ownership work. #1114 remains HOLD.

## Evidence and remaining hypotheses

### A: update-all scanner-risk authorization

Source-confirmed path: `cli/internal/agentpluginscli/update_all.go` prepares imperative packages with preview `opts.dryRun=true`, retaining `loaded.security` from `source_snapshot.go`. Its apply path in `update_multi_apply.go` reaches `applyPreparedUpdate` without the `authorizeSecurityAssessment` call used by single/multi update in `update_multi.go`. The authorizer in `security.go` intentionally permits preview under dry-run, so authorization must use the actual command options.

Production composition matters: `cli/cmd/agentplugins/compose.go` supplies `app.Lifecycle` as a direct use-case service; `lifecycle_helpers.go` returns that service, and the prepared service's `UpdateGroup` stages and commits. The separate installer facade's package-assessment gate is not on this path; replacing only `app.Installer` with a preview app does not gate lifecycle mutations. This is source evidence, not an executed reproduction yet.

Proposed behavior: authorize every changed imperative package during complete batch preflight, before `applySaved` and before any imperative effects. Use the actual `session.opts` and authorize all changed imperative preparations before `applyAll` / `applySaved`. Reuse the existing authorizer's decision and interactive behavior. The authorizer currently writes its own JSON failure; add only a small update-specific use of the existing command output seam to suppress that nested document so `renderUpdateAll` owns the one public JSON document. Do not create a new authorization framework or policy. The saved recheck/freshness pass must not authorize or prompt a second time. Preserve explicit `--accept-security-risk` and dry-run policy; `--yes` must not bypass authorization. Preparation/cache observations may follow existing semantics; refusal must preserve installed bindings, managed/native bytes, receipts and saved agreements.

### B: stale prepared local update (conditional)

Source evidence: `install/integrationctl/agentplugins/installer/confirmation.go` compares captured delivery/profile authority, but early-continues for zero SelectedDelivery plus nil ProfileAuthority without comparing the captured source/package revision. Claude refinement can retain this legacy zero projection. Apply reloads under the existing mutation lock; same-version local changed bytes are valid revisions.

Hypothesis requiring a red runtime reproduction: prepare updates A and B against C0, apply B, then apply stale A; A may overwrite B. Reuse existing copied-package/new-digest and stale-plan fixtures rather than inventing a second engine. If confirmed and accepted in both plan reviews, compare the selected `ClientBinding.PackageRevision` committed authority within existing locked confirmation, including the legacy zero-projection route. `domain.ClientPackageRevision` in `domain/state.go` records exact per-client package facts; installation-level Source/Package is shared latest accepted state and is unsuitable when siblings converge separately. Retain binding existence and `ClientBindingID` checks before any legacy early return. Explicitly distinguish nil historical revision from populated revision; two nil revisions do not invent authority. Reuse complete existing domain value semantics (including nested facts), without timestamps or whole-state equality. Read the actual confirmation and domain types before finalizing the narrow comparison; no new lock is needed. Reject through existing `ErrPlanChanged`, `OutcomeConflict`, and `plan_changed` contracts before effects. Do not compare an entire environment snapshot, as unrelated sibling changes must remain valid.

If the runtime reproduction does not fail as predicted, record the evidence and defer B without speculative production changes. A remains independently actionable.

## Invariants and scope

Preserve existing source/path containment, symlink handling, foreign-file ownership, immutable package identities, mutation/legacy lock ordering, receipts, rollback/recovery, public commands, JSON shape and exit-status/refusal contracts. Reuse existing domain authorities, authorizer, planner, parser, store and transaction seams. No duplicate installer/resolver/state engine, interface without a demonstrated need, feature flag, client-ID branch in generic code, or lint exemption.

Out of scope: frozen VS Code code and dedicated fixtures; Skill/MCP probing; release publication; broad #1087/#1093 audit; dependency version-selection redesign; generic install UX/outcome changes; generated source edits; native-client policy changes.

## Anticipated files

These are review boundaries, not permission to edit every file:

- A production: `cli/internal/agentpluginscli/update_all.go` and/or `cli/internal/agentpluginscli/update_multi_apply.go`; read existing `security.go`, `update_multi.go`, `source_snapshot.go` without changing policy.
- A ordinary regression: existing `cli/internal/agentpluginscli/update_all_test.go`, `update_all_saved_test.go`, or a focused update-specific test file.
- A production-composed/public route regression: `cli/cmd/agentplugins/saved_updates_e2e_test.go` or an update-specific companion using the actual composer. Extend the existing production installation catalog and `cli/cmd/agentplugins/testdata/package_manager_install_cases.json` only for a necessary qualifying journey.
- B conditional production: `install/integrationctl/agentplugins/installer/confirmation.go`, or a cohesive helper beside it if existing size constraints require one. Use existing domain/use-case confirmation contracts; determine whether any narrow accompanying file is necessary from the red reproduction.
- B conditional regression: `install/integrationctl/agentplugins/installer/engine_test.go`, which contains `TestUpdateCopiedPackageWithNewDigest` and `TestUpdateStalePlanChanged`.
- This plan records reviews, decisions and final evidence. Workflow changes are not anticipated.

Keep new production files within 500 effective lines and functions within existing length/complexity limits; existing exempt production files cannot grow. No mechanical split solely to evade limits. Any newly discovered required production path must be justified in this plan before expanding scope.

## Execution and review gates

1. Commit this v1 Markdown plan before production, test or workflow edits; open a Draft PR.
2. Independent reviewer R1 checks source evidence, batch authorization ordering, composition, B authority scope, overlap and verification. Record findings and dispositions; commit revised plan v2.
3. Independent reviewer R2 reviews v2 afresh. Record findings and dispositions; commit revised plan v3. Resolve both rounds before implementation. Ordinary hosted workers use gpt-6.1-sol, low reasoning, default tier per AGENTS.
4. Add and execute narrow red regressions against unchanged production code. Record exact commands and actual failures. Reconfirm B's conditional inclusion against both reviews.
5. Implement minimal A fix; implement B only if its runtime failure confirms the hypothesis. Keep tests independently asserting expected effects and bytes, not echoing implementation outputs.
6. Run focused regressions and applicable contract/native/rollback checks, `make lint`, and `make test-core`. Run install compatibility/polyglot checks when applicable under CONTRIBUTING.
7. Run `python scripts/package-manager-e2e.py` with disposable profiles and retained report/logs. A restricted-host archive run is supplementary and must retain `full_qualification: false`; it cannot stand in for full qualification.
8. Obtain independent implementation review; resolve findings, rerun checks affected by revisions, and recheck overlapping PRs/main. Rebase/resolve without weakening safety.
9. Mark ready and verify actual successful final-head CI, including existing four-platform full package-manager PR qualification and applicable native/runtime gates. Queued, cancelled, skipped, blocked, draft preflight or evidence-only jobs are not passing product evidence.
10. Merge only after reviews and exact-head required checks pass. Verify the actual merged commit's six-platform full package-manager run (Linux x64/ARM64, Windows x64/ARM64, macOS ARM64/Intel). Report remaining blockers honestly; do not claim published-release qualification or publish a release.

## Regression acceptance

A must reproduce through public CLI parsing and production composition, not only a helper or installer facade stub. Exercise a clean earlier imperative item followed by a blocked later item, and a mixed saved/imperative batch with a blocked item. The saved fixture must contain a real pending manifest/lock/materialization change, rather than a no-op graph. All installed packages, saved manifest/lock, managed/native bytes and receipts remain unchanged on preflight refusal; native mutation calls remain absent. Assert truthful single JSON output with existing failure/status semantics and no fabricated success: decode the first JSON value, then require a second decode to return EOF. Exercise `--yes` refusal, explicit accepted-risk success and read-only dry-run using existing policy. Run accepted-risk success on a fresh fixture separate from refusal snapshots. Successful scenarios clean up owned artifacts and preserve independently seeded foreign bytes.

Fixture security assessments can establish authorization routing. They cannot prove production scanner execution; retain the existing unchanged-executable cold production-scanner phase as separate evidence.

B's red case prepares A/B from C0 with same-version distinct local payloads, applies B, then attempts A through the real prepared-update API. After refusal, independently expected B payload/native bytes, current state, receipts and native mutation-call count remain intact. Freshly preparing A must still succeed; an unrelated sibling binding change must not invalidate a selected binding. Prove the legacy zero-projection route by explicitly asserting zero `SelectedDelivery` and nil `ProfileAuthority` on the relevant captured/current binding and delivery before apply. Use the existing locked confirmation authority; do not add a new snapshot/lock mechanism.

Proposed production catalog IDs are `update-all-security-batch-preflight` and `update-all-security-mixed-saved-preflight`, registered in the existing flat JSON array and `TestPackageManagerInstallE2E` map. The first journey includes refusal/yes, fresh accepted-risk and dry-run subcases; the second proves a real pending saved change remains untouched. Focused command from `cli/`: `go test -tags=uappm_e2e -count=1 -timeout=12m ./cmd/agentplugins -run '^TestPackageManagerInstallE2E/update-all-security-(batch|mixed-saved)-preflight and produce terminal passes with no skips. Assert exact independently expected payloads and effects. Native executable discovery/reload/MCP/model claims require the existing genuine native lanes; fixture runners do not establish them.

## Estimates and decision record

A: roughly 40–90 production LOC and 200–350 test LOC; complexity 3/10, reliability value 9/10, confidence 8/10 before runtime reproduction.
B adds roughly 40–80 production LOC and 150–250 test LOC; complexity 5/10, reliability value 9/10, confidence 6/10 until reproduced.
These are sizing estimates, not targets. Prefer fewer cohesive changes and meaningful scenarios over redundant tests or abstractions.

- v1: committed as `a747054a52ad2496d9c16e1b6ca146e426710faf` before implementation; Draft PR #1116.
- R1 / v2: reviewer `update_plan_r1` reviewed `a747054a52ad2496d9c16e1b6ca146e426710faf`. Resolved all four findings: one JSON owner with EOF assertion; actual session options and pre-apply authorization without freshness reprompt; selected per-binding revision plus nil/identity handling before legacy return; exact engine fixture path and catalog IDs/commands. R1 required these corrections before implementation; v2 awaits R2. No runtime PASS is claimed.
- R2 / v3: pending.
- Red reproductions and conditional B decision: pending.
- Implementation review, exact-head qualification, merge and merged six-platform evidence: pending.
`. Ordinary B reproduction from `install/integrationctl/agentplugins/`: `go test -count=1 ./installer -run '^TestUpdate(StalePreparedLocalRevision|CopiedPackageWithNewDigest|StalePlanChanged) and produce terminal passes with no skips. Assert exact independently expected payloads and effects. Native executable discovery/reload/MCP/model claims require the existing genuine native lanes; fixture runners do not establish them.

## Estimates and decision record

A: roughly 40–90 production LOC and 200–350 test LOC; complexity 3/10, reliability value 9/10, confidence 8/10 before runtime reproduction.
B adds roughly 40–80 production LOC and 150–250 test LOC; complexity 5/10, reliability value 9/10, confidence 6/10 until reproduced.
These are sizing estimates, not targets. Prefer fewer cohesive changes and meaningful scenarios over redundant tests or abstractions.

- v1: initial plan committed before implementation; R1 pending.
- R1 / v2: pending.
- R2 / v3: pending.
- Red reproductions and conditional B decision: pending.
- Implementation review, exact-head qualification, merge and merged six-platform evidence: pending.
` (the new test name is proposed). Full qualification remains `python scripts/package-manager-e2e.py` at the repository root.

Cross-platform catalog additions must register required case IDs and produce terminal passes with no skips. Assert exact independently expected payloads and effects. Native executable discovery/reload/MCP/model claims require the existing genuine native lanes; fixture runners do not establish them.

## Estimates and decision record

A: roughly 40–90 production LOC and 200–350 test LOC; complexity 3/10, reliability value 9/10, confidence 8/10 before runtime reproduction.
B adds roughly 40–80 production LOC and 150–250 test LOC; complexity 5/10, reliability value 9/10, confidence 6/10 until reproduced.
These are sizing estimates, not targets. Prefer fewer cohesive changes and meaningful scenarios over redundant tests or abstractions.

- v1: initial plan committed before implementation; R1 pending.
- R1 / v2: pending.
- R2 / v3: pending.
- Red reproductions and conditional B decision: pending.
- Implementation review, exact-head qualification, merge and merged six-platform evidence: pending.
