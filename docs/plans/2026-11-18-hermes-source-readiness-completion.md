# Hermes completion: source sessions, native readiness and bounded recovery

Date: 2026-11-18  
Status: plan review round 1/7 completed; implementation has not started.  
Baseline: `fbab48c9d46219ba8e07d5361e6c2043eca9eb97` on `main`.  
Owner: this PR; changes, including plan revisions, are committed and pushed immediately.

## 1. Intended result

A Hermes installation should have a clear, honest next step: UAP prepares the selected
components automatically, verifies the native facts that it can actually observe,
and refuses an unsafe or ambiguous runtime without touching another profile.
The main new capability under investigation is an isolated Project MCP session
for a precisely qualified source-installed Hermes runtime. It must fit the existing
session and installer architecture, not introduce another installation engine.

This PR contains the plan, its seven review rounds, implementation, focused
regressions, native E2E evidence, review fixes and final merge. It does not publish
a release. There is no line-count target and no requirement to invent production
changes where the baseline already works.

## 2. Verified baseline and dependencies

The baseline already includes:

- Automatic User Skills and supported stdio/Streamable HTTP MCP preparation.
- Project Skills; confirmed plugin add grants eligible exact-root native trust,
  including supported prepared completion, with independent trust-list confirmation.
  Raw Skills add/picker and repeated saved dependency add have separate gaps below.
- Standalone-only Project MCP at `<project>/.hermes/uap-session`, selected through
  the existing explicit native session command.
- Exact component ownership, receipts, locks, staged Skill swaps, native YAML
  mutation, compensation and retained recovery on ambiguous effects.
- `Prepared` as the truthful result after file preparation or trust.
- Revoked trust preserved by update, repair, dependency sync and verification.
- Meaningful native Linux/macOS/Windows lanes from merged PRs #923, #956 and #994.

The current source-runtime refusal is intentional. Upstream Hermes at
`2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19` runs dependency preparation and update
recovery before ordinary configuration handling. A different `HERMES_HOME`
can select different dependency state; a manifest bypass is not a solution.

Overlaps to refresh before implementation and before merge:

| Existing work | Boundary in this PR |
| --- | --- |
| Open #957, generic native Skill readiness/UI | No unrelated clients, generic readiness redesign or picker UI. Reuse established ports; reconcile if it merges. |
| Open #931, native qualification continuation | Inspect exact changed paths before touching shared native harnesses or saved dependency consent. |
| Merged #994, automatic Hermes install E2E | Extend its fixtures only for changed behavior; do not rebuild the matrix. |
| VS Code hold | No changes to VS Code or its generated declarations. |

## 3. Scope

### Included

1. Resolve the source-runtime safety question with a pinned native experiment.
2. If it passes, admit one explicit, fully described source installation shape
   through a small Hermes-local runtime descriptor consumed by the existing
   Project session authority.
3. Improve Hermes native readiness only where a fresh, exact native observation
   supports the claim. Start with pure Skills; mixed Skills/MCP remains Prepared
   unless every relevant predicate is observed.
4. Close the proven raw Skills add/picker consent-and-disclosure gap, for fresh
   and exact prepared repeat installs; close repeated saved plugin-add completion
   through a repair already shown in review. Reuse transient explicit-add consent
   and prepared-setup contracts. Ordinary sync/update/repair gain no trust authority.
5. Exercise interrupted installation recovery through existing transactions and
   correct only demonstrated Hermes-specific gaps.
6. Update Hermes technical documentation, actionable limitations and the evidence
   record. Keep JSON shape and exit-code contracts stable.

### Excluded

- Arbitrary Hermes forks, unpinned upstream releases, every source installation
  layout, arbitrary worktrees, dotenv/managed/backend overlays, or global fallback.
- A dependency installer, environment copier, reconstructed Hermes package-manager
  facts, profile migration, second state store, second session engine or generic
  runtime registry.
- Global User MCP writes as a substitute for Project MCP.
- Live vendor credentials, billing or proof of model quality. A scripted local
  provider may prove the actual driver/tool-call path but is not live-provider
  authentication evidence.
- Restoring a whole User YAML file, overriding foreign content, restoring revoked
  trust, automatic trust on inspection/startup, and implicit session launching.
- Unrelated clients, public consent flags, default-scope changes, broad refactors,
  website translations, release publication, or unit tests for unchanged helpers.

## 4. Source-runtime qualification gate

Before adding support, record these facts from the actual pinned installation:

| Question | Required answer |
| --- | --- |
| Selected launcher and code | Exact physical launcher, code root, upstream revision and supported OS/architecture. |
| Dependencies | Existing selected interpreter/generation; which home owns its state; whether Project home starts synchronization, adoption or recovery. |
| Startup effects | Complete bounded set of expected writes and children; no unnoticed user-profile/source mutation or detached helper. |
| Native configuration | Exact selected Project config path and complete MCP mapping, independent of User/profile fallback. |
| Revalidation | Every mutable authority relevant to startup can be retained and rechecked before native work and final child. |
| Official entrypoint | Existing upstream invocation only; no private replacement bootstrap or synthetic PM state. |

Probe the normal official source install and a custom user home/external source
directory. A narrower existing legacy installation shape may be investigated,
but must not be described as general support for current official installers.

The probe must compare original User profile, Project A/B, source checkout,
runtime facts/dependency generation and process cleanup. Check a native metadata
command, config inspection and a real MCP tool call. A successful `--version`
or `config get` alone does not establish passive startup.

If the gate fails, keep the refusal and document the concrete upstream dependency.
Do not lower containment or transaction guarantees to make the test pass. Revisit
the remaining deliverables and state any unmet source-support goal explicitly;
a blocked capability is not counted as implemented.

## 5. Minimal implementation design

### 5.1 Runtime admission and session reuse

Keep `qualifyHermesBundle` and its standalone contract intact. If the experiment
qualifies a source shape, introduce only a Hermes-private representation of the
facts that differ: launcher, code root, runtime selection, physical identities,
qualification commitment and expected native metadata.

Adapt these existing seams without a new public port unless genuinely necessary:

- `Adapter.QualifyPlan`: inert capability admission before any install mutation.
- `prepareProjectSessionEnvironment`: exact runtime facts in the existing source
  commitment; no native execution while planning.
- `captureSessionPassive`: explicit source startup predicates and retained roots,
  without relaxing the standalone catalog.
- `verifySessionMetadata` / `verifySessionNativeSource`: exact native revision,
  install directory, config source and complete MCP mapping.
- Existing original-root mutation authority, process runner, cancellation,
  retained inspection and final revalidation remain the only execution path.

Planning, add, verify, update, repair and remove must not start a source bootstrap
as a discovery shortcut. `--print-runtime-command` is native execution because
the upstream launcher can run update recovery first.

### 5.2 Native readiness

The pinned CLI has no exact JSON Skill inventory: `hermes skills list` renders
names/source/trust/status without selected paths or digests. Existing E2E uses
installed-runtime `find_all_skills` and `skill_view(preprocess=False)` instead.
Moving that observer into production needs separately qualified runtime custody;
`PreparedSetupCompleter` is not an observation transport. No table/JSON shortcut.

Define the observable predicate before changing outcomes:

- Current selected runtime and profile, exact requested Skill identity, content
  and location, effective project trust and quarantine/discovery behavior.
- Same-name User, built-in or foreign content cannot satisfy a Project result.
- Bounded output, deadline, cancellation and verified process-family cleanup.
- Ownership/profile/runtime revalidation around the observation.
- Missing, shadowed, disabled, quarantined, malformed, truncated or ambiguous
  evidence retains Prepared or the existing typed failure.

Use the current Hermes lifecycle/prepared-setup extension. Do not infer Ready
from receipt existence, a trust list, config listing, a successful version command
or an E2E fixture's discovery result. Keep component-level distinctions visible:
file preparation, native discovery, MCP connection, tools/call and live model
authentication are different observations.

If no supported exact native observation exists, improve truthful guidance and
native E2E evidence instead of adding a weak readiness heuristic.

### 5.3 Explicit-add completion

Two proven routes need completion; existing direct plugin add is baseline.

**Raw Skills add/picker:** pass the explicit selected-add alias context through
preparation, review, stale-plan check, confirmed Apply and retry. Before enabling
native trust, carry the Hermes adapter's existing whole-root trust disclosure
from plan actions into the existing warning/review model. The disclosure covers
both Skill directories, future content, quarantine and persistence after removal.
No picker UI redesign and no new consent flag. Decline/dry run has no trust effect.

**Repeated saved plugin add:** convert an eligible unchanged exact binding to the
existing reviewed native repair before confirmation, and select the identical
operation during recheck. Reuse the saved-add repair hook, existing trait or
capability classification, `managementNativeRepair` and `PreparedSetupCompleter`.
Widening only the consent bit cannot fix the unchanged-action short circuit.

Both routes must:

1. Act only during newly confirmed explicit add and its reviewed selected closure.
2. Finish only an intact eligible prepared Skills-only Project binding.
3. Preserve source pins, installation/binding IDs, receipts and unrelated intent.
4. Reuse exact trust profile/root checks and preserve prior trust choices.
5. Leave ordinary sync, update, repair, verify, remove and startup without trust
   authority, including after `hermes skills untrust`.
6. Stop on stale review or unclosed processes with existing typed failures.
7. Keep action/result JSON and exit codes stable; trust/file preparation stays
   Prepared unless the separately qualified native predicate succeeds.

Reconcile `deps_sync.go` with open #931 and select the exact reusable mechanism
by round 7. Carry disclosure through existing backend review data; avoid #957's
independent picker rendering work.

### 5.4 Recovery

Use the current journal, staged Skill transaction, native-config kernel,
receipts and original-root handles. Classify cases before changing behavior:

| State | Allowed outcome |
| --- | --- |
| Entire exact-owned projection absent | Existing reviewed repair can recreate it. |
| Intact exact-owned projection; setup/discovery incomplete | Explicit add may finish eligible setup; ordinary verification reports facts. |
| Present partial/edited/foreign projection | Refuse overwrite; preserve evidence and user content. |
| MCP fails after Skill publication | Compensate only exact prior owned Skill state. |
| Authority changes or effects/process cleanup remain uncertain | Retain recovery, report uncertainty and stop automatic retry. |
| Native trust writes but readback fails | Preserve the user's trust decision and installed files; report unconfirmed setup. |

Persistent user trust is outside installer rollback. No YAML snapshot restoration
and no compensating untrust command.

## 6. Critical edge cases

Review existing evidence and exercise critical changed boundaries, not every
theoretical filesystem permutation. Add tests only where production behavior changes:

1. Source/launcher/runtime replacement after plan, including same bytes at a new
   physical object; root/parent replacement and Git worktree redirection.
2. Active profile, managed overlay, backend or environment expansion selects a
   different configuration source.
3. Project A/B/User/default-home overlap, Windows case aliases/reparse points and
   inherited home selectors.
4. Dependency adoption/sync/recovery, update journal, bytecode and scratch helper
   creation before the requested native command.
5. Mixed packages and whole-config comparison: no foreign/global MCP inheritance,
   no ignored mismatch in disabled entries, no fallback config.
6. Trust revoked between add and retry; future update/repair/sync must not grant it.
7. Cancellation or failure between file publication, native trust, readback,
   receipt save and final process cleanup.
8. Same-name foreign Skill, quarantine, malformed/truncated native output,
   disappearance or drift after observation.
9. Concurrent UAP/native edits: exact lock/revalidation or safe refusal; no stale
   observation may authorize a later mutation.
10. Nonterminal/JSON output: one structured result, preserved typed exit behavior,
    no prompt/progress contamination.

## 7. Test strategy

### Reuse first

Rerun existing Hermes profile, trust, Project session, retained-authority,
transaction/rollback and CLI cold tests. Do not clone their unchanged assertions
into new suites.

### Add only tests for changed decision boundaries

- One focused source-qualifier matrix for admitted shape and critical refused
  drift/overlay/recovery cases, if source support is implemented.
- Small pure-Skill discovery identity/ambiguity/cancellation tests, only if the
  production observer changes.
- Focused raw fresh/repeat Skills add and saved-repeat regressions: disclosure
  before confirmation, decline/dry-run preservation, exact reviewed repair,
  same IDs/pins and no-consent sync/update/repair controls.
- One fault-injection regression per actual recovery bug; reuse shared harnesses.

### Native E2E

Extend existing workflows, do not introduce a fourth overlapping matrix:

| Lane | Required evidence |
| --- | --- |
| `hermes-native-e2e.yml` | Actual pinned source installation, User/Project Skills discovery, trust/revocation, fresh/repeat/repair/remove, preserved unrelated content. |
| `hermes-project-install.yml` | Standalone regression, any newly qualified source Project shape, exact isolated config and User/Project A/B preservation. |
| `hermes-noauth-tool-call.yml` | Real native driver and MCP tools/call through a deterministic local provider; source/runtime identity recorded. |

Linux, macOS and Windows must actually execute the claimed supported shape.
A shape passing only one OS remains unsupported on others unless explicitly
bounded and documented. Preserve unsupported outcomes rather than skip tests
and call the matrix green. Record exact UAP source SHA, vendor pin, launcher and
native runtime, OS, scenario result, cleanup result and artifact/run URLs.

## 8. Delivery sequence and seven plan reviews

Initial plan is committed/pushed before the review sequence. Every revised plan
is a separate immediate commit/push. Reviewers assess the actual previous revision;
seven simultaneous checklists do not count as seven rounds.

| Round | Focus | Required result |
| --- | --- | --- |
| 1 | Baseline and scope | Remove already completed work; name exact new outcomes and non-goals. |
| 2 | Source feasibility and architecture | Resolve official/legacy runtime distinction, qualification gate and smallest integration seam. |
| 3 | Isolation, trust and native authority | Define irreversible decisions, configuration ownership and race/refusal cases. |
| 4 | Recovery, cancellation and concurrency | Explicit state transitions, transaction boundaries and retained recovery. |
| 5 | Readiness and CLI contract | Exact success predicates, Prepared fallback, consent and output compatibility. |
| 6 | E2E and evidence | Minimal meaningful tests; supported OS/source matrix; real final-source evidence. |
| 7 | Adversarial final simplification | Cross-review revised plan, remove unnecessary abstractions, settle open decisions and implementation order. |

After round 7: implementation in small reviewable commits, each pushed immediately.
Parallel work uses non-overlapping file ownership; one integrator commits combined
changes. Run focused verification after each meaningful slice.

Conduct at least three implementation review/fix rounds:
(1) adapter/runtime correctness; (2) isolation/recovery/adversarial cases;
(3) final diff, E2E evidence and simplification. Findings are fixed or explicitly
resolved with evidence; do not merely record three approvals.

## 9. Merge gates

- Seven plan rounds completed with real findings/dispositions and pushed history.
- Production code obeys repository layering, no-growth legacy ratchet and function
  size limits; no new blanket suppression or client-ID branch.
- `make lint`, `make test-core`, affected installer/native/rollback checks and
  required CI pass on the final source. Run additional compatibility or package
  manager lanes only where the changed behavior requires them.
- Relevant native Linux/macOS/Windows jobs and tool-call scenarios actually pass.
  Queued, skipped, cancelled, historical or evidence-only runs are not PASS.
- Final review considers current main and overlapping PR changes; resolve conflicts
  without overwriting unrelated work and rerun checks for affected integration.
- Convert Draft to ready only after implementation/review; merge the verified head.
  Check merge result and post-merge workflow outcomes without claiming cancelled
  post-merge runs passed.
- No release/tag/version publication as part of this task.

## 10. Estimates and review log

Source support has the highest uncertainty. A qualified source descriptor and
integration may require roughly 400–900 production lines and 800–1,800 focused
test/harness lines; this is a risk estimate, not a target. Native readiness and
saved-add completion are scoped only after examining exact reusable contracts.
If safe support requires Hermes dependency-state synthesis, stop that design.

| Revision | Finding | Change / disposition | Evidence |
| --- | --- | --- | --- |
| Initial | Source launch mutates more than config; much baseline work already exists | Put native feasibility first, reuse current architecture, separate observable readiness and existing recovery | Baseline audit at fbab48; upstream pinned source; merged #923/#956/#994 |
| Round 1 | Frozen 7ce3cfc4 omitted raw Skills consent/disclosure and treated proven saved-repeat gap as speculative | Made both explicit outcomes, required reviewed/rechecked repair and disclosure before trust; ruled out nonexistent CLI JSON inventory; baseline recovery remains regression-only | Reviewer: hermes_readiness_recovery; CLI skills_batch, deps_sync, environment_session and pinned native skills_hub inspected |
