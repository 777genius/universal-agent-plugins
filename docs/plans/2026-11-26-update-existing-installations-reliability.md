# Update and Existing Installation Reliability Plan

Status: v3 execution update — reviewed plan completed; bounded fixes published, normal qualification pending.
Date: 2026-11-26
Repository: `777genius/universal-agent-plugins`
Baseline: `main` at `4903e6794f29ce7f2c39bfd054bf37c5bf654f79`
Delivery: PR #1118; every commit published immediately by the coordinating agent.

## Goal and review gates

Make repeated installation and updates reliable and understandable through the existing public CLI.
Prefer truthful outcomes, scoped next steps and accurate partial-success summaries without overengineering.
Preserve #1104 existing-installation diagnosis, #1115 install outcomes/console UX and #1116 batch security authorization.
R1/R2 were read-only planning reviews; v1, v2 and v3 preceded RED reproduction and production changes.

1. Publish v1 and complete R1: done; dispositions appear below.
2. Publish immutable v2 and independently review it as R2: done.
3. Resolve R2 findings and publish v3: completed at `00eed526`, before implementation or tests.
4. Publish bounded RED checkpoint `50dfb3eb`, then A fix `9ff069` and B/C fixes `fb7c875`.
5. Keep final implementation review and merge qualification separate from planning reviews.

## R1 findings and dispositions

- Accepted: a no-op may persist read-only lifecycle observations; do not promise zero state writes.
- Accepted: unchanged source can legitimately require `RefreshProjection` under its separate existing locks.
- Accepted: group apply revalidates under its own existing lock; do not promise one whole-batch held lock.
- Accepted: trace the ordinary public route, not legacy private `runUpdate`, before claiming a defect.
- Accepted: existing OS lock, holder-death and retry coverage plus `ErrActive` wait/retry guidance justify no lock redesign.
- Accepted: bound investigation to three candidates with a failing public-path oracle before any fix.
- Accepted: preserve precommit versus durable/refresh partial cancellation outcomes; never assume blanket no change.
- Accepted: use existing presenters, result context and helpers; incomplete targeting authority requires safe prose.
- Accepted: qualification means full four-platform final-PR and full six-platform actual-merge product execution.

## R2 findings and dispositions

- Accepted: candidate A must bypass the real prepared-no-change shortcut through supported lifecycle/refresh behavior; no artificial private flags or fixture branches.
- Accepted: `NoChange` and `Mutated == false` are distinct; assess per-target, durable and native effects.
- Accepted: candidate B needs a supported saved alias or imperative selector, selected public target and explicit effective scope; a physical InstallationID is not automatically a saved alias.
- Accepted: project guidance requires a validated absolute project root; no `--yes` or automatic repair/force.
- Accepted: `quoteResumeArgument` is POSIX-specific; use it only with established shell semantics, otherwise use safe contextual prose.
- Accepted: candidate C must preserve both operation and rendering failures through the existing typed output wrapper/`errors.Join` convention.
- Accepted: public mixed-outcome regression must retain typed causes with diagnostics disabled and truthful successful/failed/not-attempted states.
- Accepted: external package-manager documentation informs UX principles only, not resolution or overwrite policy.

## Existing authority and audited route

The ordinary route is `newUpdateCommand` → `finishInterruptedOperations` → saved lifecycle or `runUpdateMany`.
For applicable updates it reaches `UpdateGroup` through the `projection_refresh` wrapper, then existing `applyGroup`/`RefreshProjection`, process lock and kernel authority.
Legacy `runUpdate` is not the ordinary route and its differences alone cannot establish a user-visible defect.
Preparation, projection refresh and group apply retain their existing boundaries and revalidation.
There is no new global batch transaction, lock protocol, queue, retry engine or installer.

Preserve source identity, receipts/data, owned/foreign distinctions, path/symlink guards, locks, rollback and recovery.
Preserve public JSON/schema/exit semantics and #1116 authorization before applicable mutations.
A truthful unchanged result describes actual work; legitimate observations or necessary refresh must be accounted for.

## Bounded implementation and observed RED evidence

RED checkpoint `50dfb3eb` preceded all fixes; production scope remains the three reviewed candidates.
Normal package compilation was resource-killed and establishes no product evidence.
The executed focused RED used package-only `-gcflags=-l`, `GOMAXPROCS=1`, `GOGC=25` and `-p=1`; it is supplementary, not a normal/full qualification pass.
Independent read-only oracle and production reviews found no confirmed blocking regression, with proof limits recorded below.
The focused GREEN build at `fb7c875` was also compiler-killed before tests; it provides no product result.
Normal lint/core and final-source PM4/PM6 qualification remain pending; no GREEN pass is claimed here.

### A. Update-all actual outcomes — reproduced and corrected

The public Cobra fixture reaches real apply through command projection review, returning authoritative `NoChange=true` and `Mutated=false` for alpha while beta changes.
RED human/JSON summaries counted alpha as updated despite that result; command-file identity/mtime and durable client bindings provide controls.
`update_all.go` now skips only when every selected target explicitly reports no-change and no mutation; a real changed peer remains updated.
This is composed command integration with real files, not production-composed or vendor-native qualification.

### B. Contextual existing-installation guidance — demonstrated UX gap, prose only

A natural local-facade typed `ErrUpdateRequired` refusal omitted effective Project/target guidance.
`installer_facade.go` now supplies human contextual prose using selected public target and resolved scope/root, preserving the typed cause and JSON behavior.
No command, alias, shell quoting, `--yes`, force or automatic repair is introduced; retained-revision handling remains intact.
The test's User and Project snapshots use separate fixture roots/clients and selected metadata only.
They do not prove same-home sibling safety, native bytes, or execution of a copyable recovery command; those claims are outside this fix.

### C. Ordinary-update attention and output causes — reproduced and corrected

RED ordinary update exposed a raw private provider message without classified attention and dropped a concurrent writer failure.
`update_multi.go` reuses existing finishing/attention presentation and joins operation/output errors through the existing typed wrapper convention, including preflight rendering failure.
`update_all_diagnostics.go` classifies a sole unknown failed peer from the aggregate cause while preserving known reports and successful peers.
Human attention and retained typed causes are covered; JSON controls retain one compatible document without new fields or policy.
The mixed fixture checks peer execution and authoritative result counts, not durable/native successful-peer preservation.

## Behavior matrix: existing source evidence and bounded additions

Existing test names below are source mappings, not claims that these checks ran in this work.
CLI paths are relative to `cli/`; installer/usecase paths to `install/integrationctl/agentplugins/`.

| Case | Existing concrete coverage / added disposition |
| --- | --- |
| Repeat add, healthy same source | `cmd/agentplugins/package_manager_install_cases_test.go`: catalog `no-save-first-repeat-remove`; `installer/engine_test.go`: `TestInstallGroupRepeatUnchangedReportsBothTargets` |
| Changed source/version | `internal/agentpluginscli/saved_add_existing_test.go`: `TestSavedAddExistingStateIsExplainedBeforeSelection`; `installer/engine_test.go`: `TestRetainedInstallDifferentDigestIsUpdateRequired`; B adds contextual prose only |
| Targeted unchanged/changed update | `internal/agentpluginscli/idempotent_update_test.go`: `TestImmediateMultiTargetUpdateReusesNativeBindings`; install catalog `component-set-replacement`; C adds presentation/cause retention |
| Update all unchanged/changed/mixed | A public apply-result regression; `update_all_saved_test.go`: `TestUpdateAllDoesNotCountPartiallyAttemptedSavedAliasAsUpdated` |
| Blocked/eligible/security | `update_all_security_test.go`: `TestUpdateAllSecurityPreflight`; install catalog `update-all-security-batch-preflight` and `update-all-security-mixed-saved-preflight` |
| Concurrent operations | `adapters/processlock/reliability_test.go`: `TestLockConcurrentContendersAndRetry`, `TestLockReleasedAfterForcedProcessDeath`; one additional public add/update OS-lock integration control |
| Missing/partial/drift-owned data | Install catalog `drift-recorded-pin-repair`; dependency catalog `adversarial/managed-native-drift`; existing diagnosis/recovery and #1023 boundaries retained |
| Foreign/path/symlink conflict | Install catalog `unmanaged-source-refusal`, `foreign-environment-refusal`, `dependency-ownership-refusal`; dependency catalog `foreign-native-settings`, `symlink-lock-refusal` |
| Cancellation/interruption | `cmd/agentplugins/project_cli_crash_restart_test.go`: `TestPublicCLICrashAfterDurablePhaseRecovery`; `claude_interrupted_remove_test.go`: `TestClaudeUserUpdateInterruptedAfterStateCommitResumesOnRerun`; `environment/apply_outcome_test.go`: `TestApplySyncCanceledContextStopsBeforeLaterBindings` |
| Preparation/network failure | Dependency catalog `evolution/graph-update-failure-retry`; `adapters/sourceacquisition/archive_test.go`: `TestArchiveFailuresCleanOwnedFiles`; no acquisition changes |
| Partial outcomes | `update_all_test.go`: `TestUpdateAllMarksRemainingPreparedItemsNotAttempted`; `update_all_review_test.go`: `TestUpdateAllFailureReportRetainsEarlierImperativeBindingSuccess`; `update_all_saved_test.go`: `TestUpdateAllPublishedSavedIntentReportsPartialAndUnattempted` |
| Human/single JSON | New A/C command integration controls plus existing install ownership JSON boundary; compatible schema and exit policy retained |

The new contention test uses a real child holding the actual OS lock, boundary IPC without sleeps, loser state/package checks and retry after holder exit.
It passed as a supplementary control in the focused RED execution; it is not vendor-native qualification and adds no locking/retry logic.

Six IDs are registered in the existing `evolution` map and shared JSON inventory:
`update-all-actual-outcomes`, `repeat-add-project-guidance`, `update-add-process-contention`, `update-attention-classification`, `update-mixed-peer-attention`, `update-apply-output-causes`.
They dispatch existing untagged regressions through `TestPackageManagerE2E`; no runner, group, report schema or framework is added.
All six are composed public-command integration; production-composed installation and real-scanner lanes retain their separate proof boundaries.

## Ownership and exclusions

Audit only relevant CLI add/update presentation and existing installer/use-case/lock call paths.
Record exact affected files and map current tests before implementation; reuse existing fixtures and authorities.
Overlap inspection preserved #1023 and #1117. #1087 merged at main `61542d3e` with qualification-script/workflow changes only, no production overlap.
No rebase is required absent conflict; do not absorb or replace unrelated work.
Narrow dependent slices when necessary and continue independent evidence-backed work.

No new flags, modes, resolver, state engine, daemon, abstraction, automatic force/repair or lock deletion.
No removal of useful existing code, foreign cleanup, ownership adoption or blanket native-effect retry.
No VS Code, held #1114 probes, release, website, unrelated client/source work or broad cleanup.
Do not hand-edit generated files or grow lint exemptions.

## Reversible implementation and verification after v3

1. Planning, bounded RED reproduction, published minimal fixes and independent read-only review: completed as recorded above.
2. Complete focused GREEN verification, then normal required checks on stable final source.
3. Complete final-PR qualification, ordinary authorized merge and actual merged-source acceptance.
4. Publish every subsequent commit immediately; resolve concrete findings without speculative expansion.

Run `make lint`, `make test-core` and applicable affected native, rollback and cross-platform checks.
Use the existing package-manager E2E catalog/runner: full four-platform final PR, full six-platform actual merge.
Retain applicable security preflight and existing cold real-scanner qualification.
Compare output with authoritative results, durable effects and recovery state; allow legitimate observations/refresh.
No real vendor subscriptions or production credentials; no live user installation fixtures.
Queued, running, cancelled, skipped or evidence-only jobs do not establish product success.
Wait normally; avoid duplicate dispatches, unnecessary reruns and constraint bypasses.
Respect current Go file/function/complexity limits without baseline growth.

## Official UX references

- [pip install](https://pip.pypa.io/en/stable/cli/pip_install/): prefer an installed version unless upgrade is requested.
- [Homebrew upgrade](https://docs.brew.sh/Manpage#upgrade-options-installed_formulainstalled_cask-): make targeted upgrade and preview intent explicit.

Use these only for truthful, explicit, reviewable UX; do not import Brew install auto-upgrade, force or resolver behavior, or add a dry-run flag.

## Acceptance, stopping and revised estimates

Every selected correction needs a public reproduction, minimal fix and meaningful passing verification.
Every matrix row gets evidence or an explicit unresolved/overlap disposition; speculative code is not required.
Stop expansion when evidence would require architecture changes, destructive recovery or a new contract.
Ask for a decision only when existing authority and user instructions cannot resolve the contract.
Ordinary merge is already authorized, subject to genuine review and qualification gates.

Actual bounded scope: four production files, three regression files, two existing catalog files and this plan.
Counts and confidence below describe scope, not qualification success.

| Dimension | Revised provisional estimate |
| --- | --- |
| Production changes | Approximately 80 changed lines across four existing files |
| Meaningful regressions | 369 lines across three files, including the single contention control |
| Supporting documentation | This plan only; no new evidence framework |
| Complexity | 4/10; no lock or architecture changes planned |
| Reliability improvement potential | 6/10 for the bounded reporting/guidance candidates |
| Diagnostic confidence | 8/10 after bounded RED and read-only review; proof limits remain explicit |
| Target completion confidence | 8/10 estimate, conditional on reproduction, review and applicable qualification |

Reduce scope for already-correct paths; do not spend the estimate merely to reach a patch size.
