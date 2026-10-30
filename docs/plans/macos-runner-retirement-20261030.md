# Retiring macOS runner labels

Date: 2026-10-30. Base: `29480e1d3b1fbf021c8fc0c6e3f741ac0c1fdcbb`.
Branch: `fix/macos-14-retirement-20261030`.
Status: plan only; root publishes this plan in a separate draft CI PR and obtains
independent approval before implementation. Root owns all commits, publication
and merges. Worker/reviewer model: GPT-6.1 Sol / xhigh as specifically requested.

## Evidence and scope

GitHub's [official retirement notice](https://github.blog/changelog/2026-10-01-github-actions-macos-14-runner-image-retirement/)
and [runner-images announcement](https://github.com/actions/runner-images/issues/13518)
schedule the macOS 14 retirement for November 2, 2026 and the current brownout for
October 30, 14:00 UTC through October 31, 00:00 UTC. Both list `macos-15` as a
replacement; the changelog explicitly includes it among arm64 labels. The clock
read during planning was October 30, 20:44 UTC, inside that window. Most current
repository macOS jobs already use `macos-15`, with distinct Intel labels where
needed; preserve the existing Darwin arm64 matrix entry here.

Root observed #594's Authoring Milestone A E2E run `41121594885`, Darwin arm64 job
`124298240600`, and targeted retry `124301340837` cancel after 41/44 seconds,
without runner assignment or any steps. The timing and retired label are
consistent with the official brownout; **the direct cancellation cause remains
an inference**, because those jobs expose no reason or executed-step evidence.
Repeating the old attempt cannot provide a durable replacement for the label.

All tracked regular-file references at this base contain two live runner labels
and one existing workflow-test assertion. The complete implementation scope is:

| Path | Change |
| --- | --- |
| `.github/workflows/authoring-milestone-a-e2e.yml` | Replace the Darwin matrix runner `macos-14` with `macos-15`; retain `platform: darwin, arch: arm64`. |
| `.github/workflows/milestone-a-public-channels.yml` | Replace only `macos-14` in the runner matrix with `macos-15`. |
| `npm/agentplugins/test/milestone-a-e2e.test.js` | Update the existing unfiltered/secretless/pinned/bounded/failure-preserving workflow assertion's expected macOS label. Keep every other assertion. |
| This plan | Record review, execution and integration evidence. |

Do not add a test that merely duplicates these label substitutions. Do not switch
to a floating `macos-latest`, change architecture, or alter application source.

## Preserved contracts

Keep every workflow trigger, condition, step body, tool/action pin, timeout,
permission, concurrency policy, matrix size/channel exclusion, and artifact
contract identical. In particular retain exact-head checkout and `EXPECTED_HEAD`,
the sealed candidate artifact ID transfer, candidate modes and Darwin read-only
APFS image checks, writable project prerequisites, failure receipts, always-run
cleanup and artifact upload. Keep Go 1.26.10 and Node 22.23.2 pins. Do not remove
the Darwin E2E leg, skip its native prerequisite, accept cancellation as success,
raise timeouts, relax sealing/containment, or reuse another candidate's artifacts.

The current `AGENTS.md`, authoring capability-preservation contract, owner holds
and release runbook were read. This task does not reopen frozen VS Code work,
authoring phases 9–11, legacy retirement, publication or release qualification.
The public-channels workflow remains manual with its exact release inputs. Do
not dispatch it, publish a package, invent release inputs, or change its retained
legacy channels merely to validate this runner edit. Any real public-channel run
requires a separately valid, owner-authorized release context.

## Proportionate validation

After plan approval/publication and the implementation gate:

1. Review the full implementation diff: only two runner scalars and the existing
   test's expected scalar change. Compare parsed workflow structure when a local
   parser is available; all remaining values/step bodies must be identical. No
   repository actionlint/yamllint gate was found, so do not introduce new tooling
   or a bespoke committed label test for this change.
2. Run `node --test npm/agentplugins/test/milestone-a-e2e.test.js` and
   `python3 scripts/test_public_channel_e2e.py`. These existing suites cover
   candidate/workflow safeguards and the manual exact-input public-channel
   contract without executing released packages. Report the local Node version
   honestly (currently 24.22.1); actual CI continues to use its unchanged 22.23.2
   pin. Run `git diff --check` and repeat the tracked-reference inventory: no live
   retired runner or stale test assertion remains. Historical plan evidence is
   not an active runner reference.
3. Obtain independent review of the exact code diff, then let the CI PR's normal
   required checks and Authoring Milestone A E2E run on its new exact head. Verify
   the Darwin arm64 job actually receives a macOS 15 runner and executes its
   unchanged prerequisites, candidate checks, journeys, cleanup and evidence
   upload. Confirm the candidate/receipt head and artifact IDs match that run.
   Local contract tests cannot substitute for this native CI evidence.

## Safe integration with #594 and #595

1. Keep the fix in its independent four-file CI PR. After code review, root may
   propagate the **exact three implementation blobs** to #594 and #595 while
   preserving every already-approved feature blob. Limit the forward update to
   those two workflows and the existing assertion file; inspect any conflict
   rather than replace an entire feature tree with this older base.
2. The CI PR and both updated feature heads may validate in parallel. Every new
   feature head must run valid exact-head required CI and fresh Authoring
   Milestone A E2E from its inherited workflow. Rerunning an old cancelled run
   uses its old workflow/source revision; neither those attempts nor previously
   prepared bundles qualify the new source head.
3. Merge the reviewed CI PR first if its gates permit, then the feature PRs after
   their current exact heads satisfy review and required checks. If a feature
   head changes again while inheriting current main, qualify that resulting head
   under existing policy; do not carry success or artifacts across a source
   change by assumption. Preserve existing run-attempt/evidence admission rules.
4. Record the cancelled jobs as infrastructure evidence with the causal caveat
   above; do not waive their required E2E. New allocated/executed successful jobs
   supply the missing proof. If macOS 15 exposes a concrete test failure, report
   it and fix that defect through the normal reviewed boundary rather than skip
   the platform or weaken its tests.

Acceptance: both active workflow matrices use the supported explicit label,
their architecture and security/artifact semantics remain exact, existing local
contracts pass, and the CI PR plus inheriting feature candidates obtain valid
new-head native evidence. No release or public-channel execution is implied.
