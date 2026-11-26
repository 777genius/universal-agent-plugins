# Update and Existing Installation Reliability Plan

Status: v3 — R1 and R2 complete; no reproduction, test or production pass claimed.
Date: 2026-11-26
Repository: `777genius/universal-agent-plugins`
Baseline: `main` at `4903e6794f29ce7f2c39bfd054bf37c5bf654f79`
Delivery: Draft PR #1118; every commit published immediately by the coordinating agent.

## Goal and review gates

Make repeated installation and updates reliable and understandable through the existing public CLI.
Prefer truthful outcomes, scoped next steps and accurate partial-success summaries without overengineering.
Preserve #1104 existing-installation diagnosis, #1115 install outcomes/console UX and #1116 batch security authorization.
R1 used read-only source and existing-test inspection; no candidate has been reproduced or tested in this work yet.

1. Publish v1 and complete R1: done; dispositions appear below.
2. Publish immutable v2 and independently review it as R2: done.
3. Resolve R2 findings in this v3; publication completes the planning gate before implementation or tests.
4. Reproduce only the bounded candidates below; retain only demonstrated failures or missing safe UX.
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

## Bounded candidates: maximum three, each RED-gated

### A. Update-all outcome counts

Hypothesis: `update_all.go` `applyAll` may report updated and increment `Updated` after a nil-error apply whose actual result does not justify that claim.
Identical sources often stop at `prepared.noChange`; a repeat add/update alone does not establish this defect.
Prove that supported public lifecycle/projection-refresh state reaches apply and then authoritative no-change, without private flags or artificial fixture branches.
`NoChange` and `Mutated == false` are not equivalent: assess per-target results, durable changes and native effects.
Compare human/JSON counts and status with those effects, files and state; allow legitimate lifecycle observations.
Add a mixed changed/no-change control so one real update cannot hide a miscount.
If existing locks/rechecks make the mismatch unreachable, stop candidate A and record it as unconfirmed.

### B. Scoped recovery guidance

Hypothesis: `install_failure.go` generic repair/update suggestions may omit the selector or scope needed to address the original installation safely.
Public oracle: create same-name User and Project installations and trigger the applicable diagnostic.
Execute its supported actual argv/runner/shell from another working directory, simulating independent user review/consent before any mutation; leave the sibling untouched.
Copyable guidance requires an authoritative supported saved alias or imperative selector, selected public target, explicit effective `--scope user` or `--scope project`, and a validated absolute `--project-root` for project scope.
A physical InstallationID is not automatically a valid saved alias; never add `--yes` or automatic repair/force.
Fix only the existing presenter using authoritative context; reuse POSIX `quoteResumeArgument` only when shell semantics are established.
Otherwise use contextual safe prose; do not expand the global classifier or introduce shell/command-builder architecture.

### C. Ordinary-update attention presentation

Hypothesis: ordinary `runUpdateMany` may expose raw `applyErr` without the classified attention presentation available to `--all`.
Reproduce through the public command and show an actionable presentation difference before changing anything.
Reuse `imperativeUpdateAgentItems` and current finishing logic only where the authoritative result supports it.
Also investigate the current loss of `renderErr` when `applyErr` is nonnil: preserve both through the existing typed output wrapper/`errors.Join` convention.
Use a public mixed peer-success plus failure/not-attempted case with diagnostics disabled; assert retained typed causes and accurate states.
Preserve exactly one compatible JSON document, without extra fields, documentation promises, retry or exit-policy changes.
Do not broaden error classification or change update continuation policy.

## Behavior matrix and evidence disposition

All rows require a final evidence disposition, not new code or duplicate tests.

| Case | R1 disposition / remaining evidence | Required outcome |
| --- | --- | --- |
| Repeat add, healthy same source | Existing diagnosis/outcome work; confirm existing proof | Truthful idempotent result; legitimate observations allowed |
| Repeat add, changed source/version | Existing identity checks; candidate B only for demonstrated guidance gap | Explicit safe existing route with correct target |
| Targeted unchanged/changed update | Existing lifecycle/apply authorities; candidate C presentation hypothesis | Actual outcome, checks and compatible output |
| Update all unchanged/changed/mixed | Candidate A public reproduction; existing authorization preserved | Accurate per-entry outcomes and counts |
| Update all blocked/eligible | Existing batch gate and continuation contract | No invented global transaction guarantee |
| Concurrent add/update/repair/remove | Existing OS-lock/death/retry proof; no architecture change justified | Existing refusal/serialization and wait/retry guidance |
| Missing/partial/drift-owned data | Existing diagnosis/recovery; overlap #1087 and #1023 | Preserve receipts/data and supported recovery route |
| Foreign data/path/symlink conflict | Existing safety authorities; no proposed change | Fail closed without adoption or cleanup |
| Cancellation/interrupted operation | Existing lifecycle/recovery coverage to map | Distinguish precommit refusal from durable or refresh partial effects |
| Preparation/network failure | Existing failure contract; hypothesis only | No claimed apply that did not occur |
| Apply partial failure | Existing durable outcomes; A/C only if demonstrated | Applied, failed and not attempted remain truthful |
| Human/single JSON | A/B/C only where public evidence identifies a gap | Actionable text; one compatible JSON result |

At most one deterministic public contention E2E may be added if existing public proof is absent.
It must exercise the actual process lock with boundary IPC, no sleeps, no loser mutation, and retry after holder exit.
It is supplementary coverage, not authorization to change locking or add a retry protocol.

## Ownership and exclusions

Audit only relevant CLI add/update presentation and existing installer/use-case/lock call paths.
Record exact affected files and map current tests before implementation; reuse existing fixtures and authorities.
Inspect open #1087, #1023 and #1117 before touching overlap; do not merge, close, absorb or replace them.
Narrow dependent slices when necessary and continue independent evidence-backed work.

No new flags, modes, resolver, state engine, daemon, abstraction, automatic force/repair or lock deletion.
No removal of useful existing code, foreign cleanup, ownership adoption or blanket native-effect retry.
No VS Code, held #1114 probes, release, website, unrelated client/source work or broad cleanup.
Do not hand-edit generated files or grow lint exemptions.

## Reversible implementation and verification after v3

1. Confirm baseline and overlapping work in the shared isolated checkout.
2. Reproduce A/B/C in disposable roots; record command, state, observed result and expected contract.
3. Classify each as already correct, reproduced defect, demonstrated missing safe UX or unresolved hypothesis.
4. Implement only demonstrated gaps with minimal existing-authority changes and meaningful regression oracles.
5. Publish each commit immediately; revise estimates explicitly before any material scope increase.
6. Exercise affected public CLI journeys and update only necessary documentation.
7. Complete independent implementation review, final-PR qualification, ordinary authorized merge and merged-source acceptance.

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

Estimates reflect read-only R1/R2 findings, not reproduction or passing tests.

| Dimension | Revised provisional estimate |
| --- | --- |
| Production changes | 30–140 lines across existing presenters/aggregation, only if RED |
| Meaningful regressions | 150–350 lines; up to 100 additional lines only for missing public contention proof |
| Supporting documentation | 20–60 lines beyond this plan, if needed |
| Complexity | 4/10; no lock or architecture changes planned |
| Reliability improvement potential | 6/10 for the bounded reporting/guidance candidates |
| Diagnostic confidence | 6/10 after read-only route audit; public reproduction still pending |
| Target completion confidence | 8/10 estimate, conditional on reproduction, review and applicable qualification |

Reduce scope for already-correct paths; do not spend the estimate merely to reach a patch size.
