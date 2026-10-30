# Clean staged resources after grouped preparing-progress refusal

Status: plan only, 2026-10-30. Independent plan review and coordinator publication
must precede implementation. No production or test source has changed for this
plan. The coordinator alone commits, pushes, opens the draft PR and merges after
the required reviews and checks. No release is included.

Base: `29480e1d3b1fbf021c8fc0c6e3f741ac0c1fdcbb` (`origin/main`, after the four
reviewed preceding PRs merged). Dedicated branch:
`fix/group-preparation-cleanup-20261030`; worktree: `uap-group-cleanup`. The owner's
GPT-6.1 Sol / xhigh selection applies to this task and its independent review.

## Confirmed defect and reachable boundary

`usecase/group_stage.go`, `stageGroupDeliveries`, checks preparing progress before
each planned target. If `reportGroupProgress` fails, that branch returns its
error directly. The other staging failures call the existing `cleanupStaged`
helper first. In `group.go`, `applyGroup` registers its cleanup defer only after
`stageGroupDeliveries` succeeds. A preparing-progress refusal for a later target
therefore skips both cleanup paths after earlier targets have allocated staging
and possibly new owned `PLUGIN_DATA`.

`reportGroupProgress` revalidates frozen profile authority before and after each
callback when the guarded service has `profileCheck`. This is a validation error;
the progress callback itself has no error result. A transient directory-observation
failure between targets can therefore leave resources behind even when the next
guarded cleanup could prove their custody.

The production installer supplies `Progress: e.reportGroupActivation` for grouped
apply, so these preparing checks run. That wrapper forwards only activating and
activated events to `Config.Progress`. This plan does not claim that an Engine
`Config.Progress` consumer receives preparing callbacks. The deterministic
reproduction uses the core `GroupInput.Progress` seam to arm the next verifier
failure; concurrent changes or observation errors can reach the same production
revalidation boundary without such a callback.

The failed operation still reports the original error and
`GroupPhasePreparationFailed`, with no managed publication, receipts or state
commit. The defect is omitted cleanup of uncommitted resources, not an unintended
managed commit or a claim that persistent authority loss permits removal.

## Reproduction evidence before implementation

A temporary Go source overlay exercised the unchanged production code using the
existing `serviceFixture`, real `providers.Stager` and `PluginDataManager`, real
profile-authority capture/revalidation, and the existing `guardFrozenProfiles`
wrappers. A recording stager delegates actual staging/discard. An executable
fixture supplies inert launcher bytes; no client, package runtime, npm or model
process is executed. Cursor and Kiro are distinct targets for the same immutable
test package. No VS Code implementation or dedicated fixture is involved.

After the first target stages a selected stdio package and obtains owned data,
the second target's preparing callback arms a one-shot error in the supplied
physical-profile verifier. The post-callback revalidation returns that original
error. The operation reports preparation failure, `Mutated=false`, no receipts,
empty installation state and absent active target paths. The first real staging
directory and newly owned data remain; the underlying `Discard` was called zero
times. Calling the unchanged `cleanupStaged` afterward, once the observation
error has cleared, safely removes both.

Additional diagnostic controls established the existing custody boundaries:

| Control | Observed result of the unchanged cleanup helper |
| --- | --- |
| Existing owned data with a marker and user file | Removes staging; preserves exact ownership-marker and user-file bytes |
| Persistent verifier refusal | Existing wrappers refuse discard/purge; staging and data remain |
| Actual rename/recreation of the later target's profile | Revalidation returns `directoryidentity.ErrChanged`; wrappers retain earlier staging and data |
| A pending journal at the earlier target's exact operation ID | The journal predicate skips that target's staging and data |

The five-case diagnostic passed in 0.097 s with Go 1.26.10. It explicitly asserts
the defective behavior and then exercises the unchanged cleanup controls; this
is reproduction evidence, not a passing regression for the proposed correction.
The overlay and probe are temporary research files under
`/tmp/uap-group-preparing-research`; no repository source was edited to run them.
The reproduction used the source shared with original main
`c08eaedeb24b27509cb75a6c7d0b4c91ac08af22`. The dedicated branch was then
fast-forwarded to the current base above without a conflict or source edit.
`group_stage.go`, `group.go`, `group_commit.go`, `native_binding.go` and
`directory_recovery.go` are unchanged between those main checkpoints.

## Bounded correction and preserved custody

Add one call to `session.cleanupStaged()` in the existing preparing-progress
error branch, immediately before `return err`. Keep the current caller defer and
all other staging error cleanup calls in place. This branch currently performs
no cleanup, so the correction does not duplicate another cleanup path.

Return the original error unchanged. Preserve `GroupPhasePreparationFailed` and
the existing no-commit result. Do not add a cleanup abstraction, new error type,
new API, cleanup retry policy, directory scan or recovery purge.

The existing helper remains the sole cleanup policy:

- Check `directoryRecoveryPending` for each exact indexed group operation ID
  before touching its staged delivery or data. Missing path authority or an
  unreadable/present journal continues to preserve recovery custody.
- Discard only a recorded nonempty staging path through the existing stager.
  Its path checks and physical-profile guard remain unchanged.
- Purge only receipts whose target recorded `dataCreated=true`. Reused or
  retained data, ownership markers and user bytes remain owned by their previous
  lifecycle. The existing data marker and path validation still apply.
- Preserve all physical-profile checks on discard and purge. Persistent profile
  replacement/refusal may intentionally retain resources; this change promises a
  guarded cleanup attempt, not deletion after lost authority.
- Preserve `clearCreatedPluginData` after successful commit and after committed
  or unknown kernel failures. The existing journal and commit custody rules
  continue to protect resources needed by committed state or recovery.

A refusal at the first target occurs before this operation allocates staging or
data. An earlier no-change target does not own newly staged resources; any
runtime prepared in its existing owned data remains retained. These cases do
not justify deleting pre-existing data or broadening cleanup scope.

## Focused regression and verification

Use a small package-local test file and existing filesystem/provider fixtures.
Keep the deterministic verifier failure at the same core seam as the research
probe, delegating capture and ordinary revalidation to the existing authority
algorithm. Do not add production injection seams or a parallel test framework.

1. Through `AddGroup`, fail revalidation at target two after target one has
   actually staged. Assert original error identity, preparation-failure phase,
   no mutation/receipts/state publication, absent active targets, removed staging
   and removed newly created owned data. The checks must verify real filesystem
   effects, not merely that a cleanup method was called.
2. Exercise the same failure with an already owned data locator containing an
   ownership marker and user file. Assert staging removal and byte-for-byte
   preservation of that data. Include a first-target refusal control so it is
   clear no later target starts or allocates resources.
3. Keep custody coverage proportionate: exercise the existing cleanup helper on
   staged resources with a pending exact-operation journal and with a real
   replaced profile. Both must preserve staging/data; no guard may be bypassed
   merely to make the new cleanup assertions pass.
4. Reuse the existing successful grouped lifecycle, retained-data switch/rollback,
   and configuring-callback regressions for commit and later-phase behavior.
   Do not create a second lifecycle suite or add native-client qualification.

Before accepting the regression, run the new later-target cleanup assertions
against unchanged base code, or an overlay restoring the original branch. They
must fail because staging/new data survive. A first-target refusal and the
custody-preservation controls must remain valid. Record this red control
separately from passing fixed-code results.

After implementation and independent code review, run the focused new tests
with `-count=1 -race`, plus these existing focused tests:

- `TestGroupedAddUpdateAndRemoveUseOneCommitDecision`
- `TestSwitchGroupPreservesOwnedPluginDataAcrossDistributionSwitchReverseAndRollback`
- `TestGroupedConfiguringCallbacksPrecedeFinalIdentityObservation`

Run the default usecase package once if the focused checks pass, changed-line
lint and the existing full-file size/architecture gates for that package, then
`git diff --check`. Run all applicable required CI for the exact published head.
Do not repeat unrelated process/native-client suites solely for this branch.
Record any environment limitation as a limitation rather than weakening a guard
or claiming skipped native execution as proof.

## Scope and ownership

The intended production change is confined to `usecase/group_stage.go`; new tests
and this plan are the remaining files. Preserve frozen VS Code source and
dedicated tests, legacy capabilities, client adapters, process supervision,
installer progress forwarding, and transaction/recovery policy.

The inspected open PR #575 owns portable package-manager acceptance work and
separate removal-attempt corrections; #520 owns remaining native project-client
qualification. Neither changes `group_stage.go` or `group.go`. Its inspected
`native_binding.go` change in #520 adds only `ManagedMCPNamesForScope` and does
not alter discard/purge guards. The separate projection-refresh lane may change
`group_plan.go`; it does not own this staging branch. Do not absorb those scopes
or claim their qualification as evidence for this correction.
