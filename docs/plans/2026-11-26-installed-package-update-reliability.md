# Installed-package update reliability

Status: reviewed plan v3, implemented A correction, targeted runtime green and independent implementation review complete; full final-head qualification remains pending. B's public Engine regression passes against unchanged production and demonstrates existing protection, so no B production fix is authorized by this evidence.

## Goal and baseline

Make installed-package updates refuse unauthorized scanner risk before batch effects and, only if runtime reproduction confirms the defect, reject stale prepared local updates before they overwrite a newer selected binding revision.

Baseline main: `ce789138a393bffd7e6d001c0e4f8aec05e47773`. Branch: `fix/update-safety-20261126`. Draft PR: #1116. Read `AGENTS.md`, `docs/ARCHITECTURE.md`, `CONTRIBUTING.md` and `docs/testing/package-manager-e2e.md`. Recheck main and overlapping PRs before implementation and merge. PR #1115, last observed at `58cbcb338e1e31c0e88b80d8cd00b872dced6736`, owns install UX/outcome reliability; keep edits update-specific and avoid its shared app/dependency rendering/repair paths. Preserve merged #1104 ownership work. #1114 remains HOLD.

## Evidence and proposed behavior

### A: update-all scanner-risk authorization

Source-confirmed: `cli/internal/agentpluginscli/update_all.go` prepares imperative packages with preview dry-run options and retains `loaded.security` from `source_snapshot.go`. The apply path in `update_multi_apply.go` reaches `applyPreparedUpdate` without the authorization used by `update_multi.go`. The existing authorizer in `security.go` deliberately permits dry-run preview, so actual `session.opts` must govern mutation authorization.

The production composer in `cli/cmd/agentplugins/compose.go` supplies a direct use-case `app.Lifecycle`; `lifecycle_helpers.go` returns it and the prepared service's `UpdateGroup` stages and commits. The separate installer facade assessment gate is not on this path. A dependency preview app replaces only `app.Installer`. This is source evidence, pending runtime reproduction.

Authorize all changed imperative preparations with actual `session.opts` during complete batch preflight, before `applyAll`, `applySaved` or any imperative effects. Reuse existing decisions and interactive policy. Preserve explicit accepted risk and dry-run semantics; `--yes` cannot bypass risk authorization. Saved freshness/recheck must not authorize or prompt again.

The authorizer currently emits a JSON failure itself. Use only a small update-specific application of the existing output seam to suppress that nested JSON response; `renderUpdateAll` owns one public JSON document. Preserve original input, context, visible warnings and prompt/output channels in human terminal mode. Do not introduce a new authorization framework or change policy.

### B: stale prepared local update, conditional

Source-confirmed: `install/integrationctl/agentplugins/installer/confirmation.go` has a legacy early return for zero SelectedDelivery and nil ProfileAuthority. It can bypass selected binding revision confirmation. Claude refinement may retain this legacy projection. Apply reloads under the existing mutation lock, while same-version local changed bytes remain valid revisions.

Read the actual authority definitions in `install/integrationctl/agentplugins/domain/state.go` and `install/integrationctl/agentplugins/domain/state_installation.go`. `ClientBinding.PackageRevision` points to `ClientPackageRevision`, the exact per-client committed revision. Installation-level Source/Package represents shared desired/latest state and must not substitute for that authority.

Runtime hypothesis: prepare A and B against C0, apply B, then stale A may overwrite B. B is conditionally accepted in this PR only after this intended runtime red reproduction confirms the defect. If it does not reproduce, record evidence and defer B without a speculative production fix.

If confirmed, compare selected binding authority within existing locked confirmation. Binding existence or ClientBindingID changes conflict before the legacy early return. Nil versus populated PackageRevision conflicts in either direction; two historical nil revisions remain compatible under the other existing checks, with no installation-level fallback. Populated revisions compare completely, including nested facts, using existing domain value semantics. No timestamps, whole-state equality or new locking mechanism. Reject with existing ErrPlanChanged, OutcomeConflict and plan_changed behavior before effects.

## Invariants and scope

Preserve source/path containment, symlink handling, foreign-file ownership, immutable identities, mutation and legacy lock ordering, durable receipts, rollback/recovery, public commands, JSON schema, exit statuses and typed refusals. Reuse existing parsers, authorizer, planner, domain authorities, store and transactions. No duplicate installer/resolver/state engine, new feature flag, client-ID branch in generic code, unjustified interface or lint exemption.

Out of scope: frozen VS Code and dedicated tests; Skill/MCP probes; release publication; broad #1087/#1093 audit; dependency version-selection redesign; generic install UX/outcomes; generated source edits; unrelated native-client policy.

## Anticipated files

- A production: `cli/internal/agentpluginscli/update_all.go` and/or `update_multi_apply.go`, with a focused update-specific helper if needed. Read `security.go`, `update_multi.go` and `source_snapshot.go`; preserve shared authorization policy.
- A ordinary regression: existing `cli/internal/agentpluginscli/update_all_test.go`, `update_all_saved_test.go`, or a focused update-specific companion.
- A ordinary production-composed regression: `cli/cmd/agentplugins/saved_updates_e2e_test.go` or an ordinary untagged update-specific companion, using the actual composer and public CLI parsing. Proposed meaningful name: `TestComposedUpdateAllSecurityPreflight`.
- A qualifying tagged journeys: existing production installation catalog in `cli/cmd/agentplugins/`, `package_manager_install_e2e_test.go`, and `testdata/package_manager_install_cases.json`. Reuse existing fixture seams.
- B conditional production: `install/integrationctl/agentplugins/installer/confirmation.go`, or a cohesive helper beside it if existing size constraints require one.
- B regression: `install/integrationctl/agentplugins/installer/engine_test.go`, containing `TestUpdateCopiedPackageWithNewDigest` and `TestUpdateStalePlanChanged`; proposed new name `TestUpdateStalePreparedLocalRevision`.
- This plan records reviews and final evidence. No workflow edits are anticipated.

These are anticipated boundaries, not a mandate to edit all files. Justify newly necessary production paths before expanding scope. Respect 500 effective production lines per file and existing function/complexity limits. Exempt production files cannot grow. Avoid mechanical splitting and redundant tests.

## Regression acceptance and commands

### A acceptance

The ordinary production-composed test must reproduce through public CLI parsing and actual production wiring, with a focused internal CLI regression as useful supporting evidence. A helper-only test or installer facade stub is insufficient.

Exercise a changed clean earlier imperative item followed by a blocked later item. Also exercise a mixed saved/imperative batch whose saved fixture has a real pending manifest, lock or materialization change, rather than a no-op graph. On refusal preserve installed bindings, managed/native bytes, receipts and saved agreements; native mutation calls remain absent. Existing preparation/cache observations may follow current semantics.

JSON must report truthful existing failure/status semantics exactly once: decode the first value, then require EOF on a second decode. Cover noninteractive `--yes` refusal and read-only dry-run. On a fresh fixture separate from refusal snapshots, explicit accepted risk permits the batch. In human terminal mode, rejection keeps the whole changed batch intact, acceptance permits it, visible warning and prompt behavior remains intact, and saved recheck never repeats the prompt. Successful scenarios clean up owned artifacts while preserving seeded foreign bytes.

Fixture security responses prove routing only. Existing unchanged-executable cold production-scanner execution remains separate evidence; fixtures cannot establish real-scanner or native-agent qualification.

Proposed catalog IDs are `update-all-security-batch-preflight` and `update-all-security-mixed-saved-preflight`, registered in the existing flat JSON array and TestPackageManagerInstallE2E map. They qualify the public journeys across existing platforms.

Focused ordinary red commands, from `cli/` (meaningful test names, no disguised filters):

```sh
go test -count=1 ./internal/agentpluginscli -run '^TestUpdateAllSecurityPreflight$'
go test -count=1 ./cmd/agentplugins -run '^TestComposedUpdateAllSecurityPreflight$'
```

Focused tagged catalog command, from `cli/`, used for final qualifying PASS after implementation:

```sh
go test -tags=uappm_e2e -count=1 -timeout=12m ./cmd/agentplugins -run '^TestPackageManagerInstallE2E/update-all-security-(batch|mixed-saved)-preflight$'
```

### B acceptance

Prepare A and B from C0 with same-version distinct local payloads, apply B, then attempt stale A through the real prepared-update API. Explicitly assert zero SelectedDelivery and nil ProfileAuthority on the relevant captured/current binding and delivery to prove the legacy route. The intended pre-fix failure must show stale A overwriting B or otherwise violating the independent expected refusal.

After the fix, assert existing plan_changed/conflict behavior before effects, independently expected B payload/native bytes, current state, receipts and unchanged native mutation-call count. Freshly preparing A must succeed. A positive sibling case changes the sibling revision and shared desired fields while selected binding revision/identity remains unchanged; it must still succeed. Cover binding absence/identity changes and nil/populated transitions without inventing historical authority.

Focused command, from `install/integrationctl/agentplugins/`:

```sh
go test -count=1 ./installer -run '^TestUpdate(StalePreparedLocalRevision|CopiedPackageWithNewDigest|StalePlanChanged)$'
```

Before fixes, record exact commands, source SHA and expected failing assertions as red evidence, never as PASS. After fixes, rerun the applicable commands and separately record passing evidence. The tagged catalog is later qualification, not the initial ordinary red proof.

## Execution and review gates

1. Commit plan v1 before implementation; open Draft PR.
2. Independent R1 reviews v1; resolve findings and commit v2.
3. Independent R2 reviews v2; resolve findings and commit clean v3. Verify authoritative sections, balanced code fences and complete commands by readback before any test or production edits.
4. Add narrow ordinary red regressions against unchanged production code. Execute and retain their intended failures. B stays conditional on its intended runtime red.
5. If local execution is unavailable, use existing Core fast on Draft synchronize. Its normal test-core execution includes ordinary installer, internal CLI and cmd CLI tests; no manual/filter input exists. Push meaningfully named ordinary regressions to obtain actual red runtime evidence there. Do not change workflows or dispatch extra jobs for red tests. A queued/cancelled/blocked job is not executed evidence. Tagged uappm_e2e tests are not covered by this initial default lane.
6. Implement minimal A changes and B only if reproduced. Run focused tests, applicable contract/native/rollback checks, `make lint`, `make test-core`, and CONTRIBUTING install compatibility/polyglot checks when applicable.
7. Run `python scripts/package-manager-e2e.py` from the repository root, preserving report and raw logs with immutable source identity. Restricted-host archive selection is supplementary with full_qualification false; it does not replace full qualification. Required cases must terminate with PASS, independently expected bytes/effects and no skips.
8. Obtain independent implementation review, resolve findings and rerun affected checks. Recheck main and overlap. Preserve safety while resolving any conflicts.
9. Mark ready and verify actual final-head required CI, including existing four-platform full package-manager PR qualification and affected genuine native/runtime gates. Queued, cancelled, skipped, blocked, draft preflight and evidence-only jobs are not product PASS.
10. Merge only after reviews and exact-head checks pass. Verify actual merged SHA's six-platform full package-manager run: Linux x64/ARM64, Windows x64/ARM64, macOS ARM64/Intel. Report blockers accurately. No release publication or claims about published npm bytes.

Native discovery/reload, MCP execution and live model behavior require their existing genuine native evidence. Normal native/PM matrices remain later gates and are not replaced by fixtures.

## Estimates

A: roughly 40–90 production LOC, 200–350 test LOC; complexity 3/10, reliability value 9/10, confidence 8/10 before runtime reproduction.
B adds roughly 40–80 production LOC, 150–250 test LOC; complexity 5/10, reliability value 9/10, confidence 6/10 until reproduced.
These are estimates, not targets; prefer cohesive minimal changes over extra abstractions or redundant tests.

## Review and decision record

- v1: `a747054a52ad2496d9c16e1b6ca146e426710faf`, committed before implementation.
- R1 reviewer `update_plan_r1`, reviewed v1. Findings resolved in v2: one JSON owner with EOF assertion; actual session options and authorization before all effects without recheck reprompt; selected binding revision/nil/identity authority; exact fixture path and catalog commands.
- v2: `1ee0312a73f09215ca31c963e9212b6e0fdda052`. Its command/tail corruption is corrected by a complete clean v3 rewrite.
- R2 reviewer `update_plan_r2`, reviewed v2. NO-GO on v2; GO after v3 amendments. Dispositions: repaired complete commands and one authoritative section each; separated intended red evidence from final PASS; explicit human reject/accept and original channel/context preservation; binding existence/identity and precise nil/populated rules before legacy return, no installation fallback; sibling revision/shared-field positive case; full state_installation.go source reading; B remains runtime-conditional.
- R1 CI follow-up incorporated: ordinary production-composed/internal/B regressions can execute through normal Draft Core fast synchronize when local execution is unavailable; tagged catalog remains later qualification, with no workflow changes.
- v3: ready for red regressions after successful clean remote readback. Red evidence, B inclusion decision, implementation review, final-head qualification, merge and merged six-platform results remain pending.

## Overlap recheck before production changes

Root independently verified clean v3 readback. A subsequent path recheck of PR #1115 at `58cbcb338e1e31c0e88b80d8cd00b872dced6736` confirms overlap in `cli/internal/agentpluginscli/update_all.go`, `update_all_saved.go`, and added `update_all_diagnostics*` files. Its changes concern applyAll failure reporting, renderUpdateAll human attention/output error handling, and appliedAgentItems in applySaved. It does not add batch security authorization or alter preflightAll/classifyInstallation; installer confirmation.go remains untouched. This supersedes any earlier implication that update-all production paths do not overlap.

Our boundary remains an update-specific authorization helper and a preflight call. Preserve #1115's apply/render/saved diagnostic behavior and recheck/reconcile current main before final merge. This records newly observed overlap without expanding scope or restarting the completed plan reviews. Ordinary red regressions are next; production changes remain subject to intended red evidence. The local executor is currently unavailable; use the already planned execution fallback without retrying it.

## Runtime scope decision after B reproduction attempt

This section supersedes prospective B implementation steps above; historical v1–v3 reviews remain unchanged. Current production correction scope is A only: installed-package batch update security authorization. B has no demonstrated defect and its production fix is deferred. Retain one public Engine regression protecting the existing stale-update refusal; private semantic tests are removed.

Root's runtime evidence used unchanged production baseline `ce789138a393bffd7e6d001c0e4f8aec05e47773` in the checkout at plan-only commit `e4458fb0dc6f646c9eb51459694af628606b1fd1`. `TestUpdateStalePreparedLocalRevision` explicitly verified zero SelectedDelivery and nil ProfileAuthority, prepared A/B, applied B, and observed stale A refused with ErrPlanChanged / OutcomeConflict / plan_changed. B's payload/native bytes, state, receipts and effect counts were preserved; freshly prepared A succeeded. Existing confirmationPlanner compares PreviousNativeObjects, including managed_package_directory.ManagedDigest, and provides the protection the initial hypothesis overlooked.

From `install/integrationctl/agentplugins/`, the following command passed all three selected tests with exit code 0:

```sh
go test -count=1 -v ./installer -run '^TestUpdate(StalePreparedLocalRevision|CopiedPackageWithNewDigest|StalePlanChanged)$'
```

Retained public Engine regression blob: `47b94c3f24032f9b18279123f4ff52bf4e64a022`. This passing regression demonstrates existing protection, not a failing-before reproduction or a new B production fix. Do not implement the proposed PackageRevision comparison on this evidence.

A runtime red evidence is recorded in the following section. Existing #1115 overlap boundaries, ordinary red execution requirements, authorization acceptance criteria, independent implementation review, final-head CI and merged six-platform qualification remain in force.

## A runtime red evidence before production changes

Root executed new ordinary regressions against unchanged production HEAD `e4458fb0dc6f646c9eb51459694af628606b1fd1`. From `cli/`, the command below exited 1; retained log: `/tmp/uap-update-security-red.jsonl`.

```sh
PATH=/workspace/scratch/344235915a64/toolchains/go1.26.11/go/bin:$PATH GOTOOLCHAIN=local GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.hooksPath GIT_CONFIG_VALUE_0=/dev/null go test -count=1 -timeout=8m -json ./internal/agentpluginscli ./cmd/agentplugins -run '^(TestUpdateAllSecurity(Preflight|HumanConsent)|TestComposedUpdateAllSecurityPreflight)$'
```

Actual run/fail events reached the intended internal preflight and production-composed imperative/mixed refusal assertions. Preview left state unchanged and targeted beta update correctly refused, establishing that the fixture assessment and targeted policy were active. The batch then incorrectly returned nil error, result success and status completed: two imperative items updated in the imperative journey, and three items including saved docs updated in the mixed journey. Installed state, native bytes and receipts changed, and native activation occurred. The imperative human decline case also observed no security prompt and ignored the intended rejection. These are intended product red failures supporting A's batch-preflight correction.

Explicit accepted-risk imperative success passed. A mixed accepted-risk assertion separately assumed the saved Skill DeclaredName was docs, while its actual generated name was skill-hash. The fixture now captures the saved installation identity and checks its persisted docs ownership receipt. The corrected composed regression rerun (`go test -count=1 -timeout=8m -json ./cmd/agentplugins -run '^TestComposedUpdateAllSecurityPreflight$'`, log `/tmp/uap-update-security-red-corrected.jsonl`) passed both explicit-risk acceptance and cleanup journeys while both refusal journeys still failed for the intended unauthorized effects. The earlier oracle error is not product red evidence; these runs are not final qualification.

Production scope remains A only. B remains deferred with an existing-protection regression. Preserve #1115's apply/render/saved diagnostics, implement the minimal preflight authorization boundary, then rerun corrected ordinary tests and all later review/qualification gates.

## Implementation and focused acceptance checkpoint

Red tests were published in `998e0f0cc81654c4aa1ba3f48c4de01d934fa13e`. The production correction is `aa03de9cb7f235ce40d1724c3fd9f57c38bb8f16`, with `update_all.go` blob `7d37a95dbfba9d1029723c59c526056a80fd3385`. Its 21 added lines authorize each changed imperative preparation using actual session options before any batch apply. The shared authorizer retains policy and human input/context/output; batch JSON owns one document and retains the assessment in existing target output fields.

The materialized correction passed the focused ordinary CLI and production-composed regressions: exit 0, 15 test/subtest pass events, no fail/skip (`/tmp/uap-update-security-green.jsonl`). Both new catalog cases also passed the exact tagged selector below: exit 0, seven test/subtest pass events, no fail/skip (`/tmp/uap-update-security-pm.jsonl`). These checks include imperative and mixed-saved refusal without effects, explicit acceptance with independently expected updated payload and lock, human consent once, retained data, foreign files and owned cleanup. The assessment is injected for routing proof; genuine pinned-scanner qualification remains in the existing executable phase.

```sh
go test -tags=uappm_e2e -count=1 -timeout=12m -json ./cmd/agentplugins -run '^TestPackageManagerInstallE2E/update-all-security-(batch|mixed-saved)-preflight$'
```

Independent read-only implementation and fixture review of the exact production blob against the red checkpoint found no blocking correctness issue. The review confirmed the complete mixed-batch authorization boundary, actual options, one JSON owner, original human channels, unchanged saved rechecks and preserved #1115 diagnostics. The existing-protection B regression remains the only B change.

Full `make lint` passed after ordinary Git history restored the shallow checkout's missing base: all four whole-file and four changed-line gates reported zero issues, and the baseline check retained 137 exclusion rules with no growth. Full core and final-head CI are separate gates. The restricted local executor's full core run encounters unavailable procfs process-tree observation and Unix-domain socket prerequisites; these failures are not product PASS. Hosted CI must execute the complete gate. PR and merged qualification results are tracked in #1116; no release is included.
