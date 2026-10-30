# Same-revision group native projection refresh

Date: 2026-10-30. Planning base: `29480e1d3b1fbf021c8fc0c6e3f741ac0c1fdcbb`.
Branch: `fix/group-native-projection-refresh-20261030`.
Status: bounded implementation and local validation complete; frozen publication
checkpoint for fresh independent code review and exact-head CI. The coordinator
owns commits, publication, the draft PR, final review, and merge. No release is
authorized by this plan. Workers and reviewers follow the current `AGENTS.md`
model and scope instructions; this lane uses the specifically requested
GPT-6.1 Sol / xhigh.

## Problem and severity

An explicit `usecase.Service.UpdateGroup` can report successful completion and
`NoChange` for an unchanged package revision even when its prepared OpenCode host
requires changing the installed native configuration from V1 to V2. The reverse
transition has the same missing decision. A requested host projection change is
therefore silently ignored.

This is a **P2 correctness issue in the exported usecase API**. The public
installer facade deliberately refuses grouped OpenCode install/update/repair in
`installer/group.go:validateGroupTargets`, because its group handles do not retain
per-target native host snapshots. Preserve that refusal. Existing usecase tests
already intentionally support grouped OpenCode through prepared authority
(`TestOpenCodeHostBridgeSingleAndGroup` and
`TestOpenCodeGroupPreparesMissingAuthorityBeforeNativeProjection`). Do not describe
this finding as a currently reachable public CLI group migration bug or as native
client qualification evidence.

### Evidence and reproduction

The current single update guards its same-revision shortcut with
`nativeDialectProjectionChanged` (`usecase/apply.go:noChangeOrResume`). That helper
in `usecase/native_attempt.go` explicitly compares the OpenCode client id,
prepared dialect strings, and historical MCP receipt kinds.

The equivalent group predicate in `usecase/group_plan.go:recordGroupTarget`
checks package revision, component removal, repair/switch state, and affected
surface, but omits a host projection change. The resulting path is:

1. `recordGroupTarget` marks the installed target unchanged.
2. `group_stage.go:stageGroupDeliveries` skips package staging for it.
3. `group_activate.go:groupActivationRequest` calls `activeNativeDelivery` on the
   old installed package and sets `VerifyOnly` when those old receipts match.
4. OpenCode's `BuildOpenCodeNativeObjects` reads the codec from that old package's
   `.agentplugins-opencode.json`; the requested prepared V2 host does not replace
   those old V1 projection bytes.
5. Read-only verification succeeds for the installed V1 objects, so the group
   reports completion without the explicitly requested projection change.

A temporary Go-overlay regression uses `serviceFixture`, `hostBridgeInput`, the
real planner/stager/activator/native kernel/state store, and a pure trusted host
fixture whose `ConfigDialect` changes from `opencode_v1` to `opencode_v2`. It adds
one group target, retains the same immutable package, then calls `UpdateGroup`.
The observed red result was:

```text
=== RUN   TestAuditGroupSameRevisionDialect
err=<nil> phase=completed mutated=false noChange=true configUnchanged=true
config={"mcp":{"docs":{"type":"remote","url":"https://docs.test/mcp"}}}
accepted same-revision V2 group update without materializing V2 projection
--- FAIL: TestAuditGroupSameRevisionDialect (0.03s)
```

The actual temporary test and overlay are
`/tmp/uap-next-critical-repro/group_dialect_test.go` and
`/tmp/uap-next-critical-repro/projection-worktree-overlay.json`. From the core
module directory in this worktree, the reproduction command is:

```bash
PATH=/tmp/uap-e2e-tools/go/bin:$PATH \
GOCACHE=/tmp/uap-audit-gocache \
GOMODCACHE=/tmp/uap-audit-gomodcache \
/tmp/uap-e2e-tools/go/bin/go test \
  -overlay=/tmp/uap-next-critical-repro/projection-worktree-overlay.json \
  ./usecase -run '^TestAuditGroupSameRevisionDialect$' -count=1 -v
```

The first reproduction was run against `d85dbb70` in the original architecture
worktree, then against `c08eaede` before this plan worktree fast-forwarded to the
planning base above. The relevant projection/usecase sources are identical in
those revisions. The final repeat on
`29480e1d3b1fbf021c8fc0c6e3f741ac0c1fdcbb` produced the same red output above:
`usecase` failed in 0.037s because the explicit V2 update was incorrectly reported
as a completed no-op. This is expected red reproduction evidence, not a passing
validation gate.

The overlay source embeds `contracttest.OpenCodeV1Host` and overrides only
`ConfigDialect` for the V2 request. This is an injected, read-only host contract
fixture, not a real host probe or permission bypass. The minimal red case does
not wire a transition recorder because the bug never attempts a transition.
The permanent successful-transition regression must wire the existing recorder.
The repository remains untouched by the overlay.

## Chosen design

Add one ephemeral, generic boolean decision to `domain.DeliveryPlan`, stored in
the private field `requiresNativeProjectionRefresh`. Expose
`RequiresNativeProjectionRefresh() bool` for readers and
`WithNativeProjectionRefresh(bool) DeliveryPlan` for trusted planners to return a
value copy with the current decision. This follows the existing private
profile-authority/value-copy pattern on the same plan. Its precise meaning is:
**the current adapter's prepared host projection differs from a recognized
recorded native representation, so an explicit update must not use only package
revision equality to skip staging**.

An initial implementation using the originally proposed exported boolean passed
behavior and recovery tests, but the operational golden harness deliberately
reflects every exported field regardless of JSON tags. It therefore added a false
decision to every stored plan snapshot, including frozen client cases. Preserve
the golden bytes and the harness's complete exported-field coverage: private
storage omits the new transient implementation detail without editing fixtures,
changing shared normalization, or weakening a test. The public JSON contract also
remains identical because it cannot encode/decode the private field. This is an
encapsulation refinement of the same decision, not a new policy or capability.

The decision is computed through the existing `clients.PlanRefiner`; no new
capability interface, dispatcher, service, recovery engine, persisted receipt,
schema, or configurable policy is needed. OpenCode owns the dialect comparison
inside its adapter. The generic usecase consumes only the decision.

### Decision provenance and lifetime

`usecase/intent.go:planInstall` already selects the exact stored planning binding,
clones its native receipts, and sends them in `domain.PlanRequest.PreviousNativeObjects`.
The generic planner clones those facts into `clients.PlanInput`, and OpenCode's
existing `RefinePlan` already receives that input and the prepared immutable host.
Use those predecessor objects, not ambient native configuration, rediscovery,
constructor guesses, or the desired staged objects, to compute the decision.

The generic planner starts with the zero decision. The OpenCode refiner replaces
its plan with the value returned by `WithNativeProjectionRefresh` using the
current inputs on every successful refinement, so a reused plan cannot retain an
unrelated earlier true value. Other adapters keep the zero decision until they
have a concrete host projection requirement of their own. Struct copies and the
existing plan-cloning helpers preserve it inside the operation. The value-copy
method must also leave its receiver and all other plan facts unchanged. It is not
written to installation state, package projection metadata, native intent,
mutation receipts, public installer DTOs, diagnostics JSON, or reflection-based
exported-field goldens. JSON decoding therefore cannot restore it as a plan
authority.

The decision grants no native ownership or mutation permission. True only
prevents an optimization; all confirmation, frozen profile, package verification,
native identity, exact receipt, state lock, durable transition, and drift gates
still apply. False is not an attestation that native activation is complete.
Existing lifecycle/runtime checks still decide whether to verify or resume.

`ports.DeliveryPlanner` remains a trusted injected planning dependency. A custom
planner that wants to support host-induced same-revision updates must produce or
preserve this generic decision, just as it must produce the rest of the plan.
All production planning in this repository uses the registry/refiner pipeline.
Document and test this contract; do not retain an OpenCode-specific fallback in
the generic usecase or claim that arbitrary external planners are corrected by a
new field they do not populate. The independent critic should assess this
source-compatibility tradeoff before implementation.

### Adapter classification policy

Move the existing comparison rather than broaden host or ownership admission.
Use the adapter's existing OpenCode object-kind constants. Preserve the current
host-validation order in `RefinePlan`; do not introduce a new codec-selection
validation pass solely to compute the boolean.

| Prepared host and recorded objects | Decision and behavior |
| --- | --- |
| V1 host, only recognized V1 MCP receipts | False; an unchanged update may take the existing runtime/lifecycle path. |
| V2 host, only recognized V2 MCP receipts | False; same behavior. |
| V2 host with a recognized V1 MCP receipt | True; explicit update must stage the requested projection. |
| V1 host with a recognized V2 MCP receipt | True; explicit update must stage the requested projection. |
| Recognized V1 and V2 MCP receipts mixed | True if any recognized receipt differs from the host; existing identity/transition validation must reject unsupported mixed-source authority rather than normalize or adopt it. The boolean itself does not certify transition eligibility. |
| No prior objects, only the managed-package receipt, or skills-only receipts | False; a first install or skills-only package does not invent a dialect migration. |
| No prepared host or no dialect accessor | False as in the old helper; the existing refiner refusal for desired native effects without a host still applies. Empty/metadata-only planning gains no new host requirement. |
| Unknown dialect or unrecognized object kind | No new migration authority or new compatibility admission. Existing host, projector, observer, and transition validation retain their present refusal behavior. |

Do not accept mixed receipts because a subset matches the target. Do not discard
foreign or unknown objects to make a transition appear eligible. Do not turn a
missing native root into a new profile selection.

### Usecase behavior and existing runtime preparation

In `applySession.noChangeOrResume`, replace the OpenCode-specific helper check in
the **replace/update branch only** with the generic plan decision. Remove the
now-unused helper from `usecase/native_attempt.go`. Preserve the existing
registration-migration, component-removal, revision, identity, and managed-package
checks and their order.

In `groupSession.recordGroupTarget`, make an explicit update target ineligible
for the package no-change optimization when the plan requires refresh. Guard
this with the existing `session.replace` operation mode. `Repair` and `Switch`
already have their separate group rules; do not route them through a new policy.
The update proceeds through ordinary staging, data/runtime preparation, managed
directory transactions, activation, transition publication, and result reporting.

Repeated `Add`/`AddGroup` must continue to verify/resume the installed projection
without replacing its managed directory merely because a different host dialect
was supplied. Ordinary `Repair`/`RepairGroup` retain their current behavior,
including exact reconstruction and command-rendering rules. `RefreshProjection`
retains its explicit projection-changing purpose. This flag introduces no new
repair refusal, repeated-add migration, or automatic refresh command.

For a matching dialect, retain the existing `finishExistingLifecycle` path in
single update and `stageGroupDeliveries`' no-change path in group update. They
still prepare the installed MCP runtime before declaring success, verify managed
objects, resume incomplete activation, and retain unresolved native attempts.
Do not implement an early boolean return that skips these checks. A changed
dialect uses the existing staged runtime path before activation. Preserve
`PluginData` ownership and runtime locator checks in either path.

## Bounded file scope and exclusions

Expected production edits are limited to:

- `domain/clients.go`: add private storage for the ephemeral plan decision.
- One small `domain/plan_projection_refresh.go` file: document and implement the
  reader and value-copy methods, including trusted custom-planner responsibility.
- `clients/opencode/plan.go` or one small adjacent adapter helper: compute it
  during existing refinement from frozen predecessor receipts and prepared host.
- `usecase/apply.go`: consume it in single update's existing shortcut.
- `usecase/group_plan.go`: consume it in explicit group update's shortcut.
- `usecase/native_attempt.go`: remove the relocated OpenCode-specific helper.

Add focused adapter and usecase regression tests in their own files. Existing
facade/recovery tests should run unchanged. Only add a narrowly justified test
assertion if a concrete coverage gap remains; do not alter their recovery setup
to make the new test pass.

Do not edit `usecase/group_stage.go` or its separately owned cleanup tests; that
parallel lane owns them. This lane relies on its existing staging/runtime paths.
Coordinate any unavoidable conflict before editing. Do not edit the frozen VS
Code implementation or dedicated fixtures, release code, authoring phases 9–11,
schemas, capability/transport tables, public support labels, dependency-selection
logic, or the current independent identity/process/environment implementation
lanes. Do not widen `installer.validateGroupTargets`, replace the OpenCode
recorder, generalize native journal registration, or change historical
group-removal or missing-intent recovery policy.

## Validation and acceptance

### Adapter decision tests

Use a small table against the real OpenCode `RefinePlan` for both supported
dialects, matching and opposite recognized MCP receipts, mixed V1/V2 receipts,
unrelated managed/skill objects, empty predecessors, and a reused plan whose
previous decision was true. Check exact predecessor slices remain unchanged.
Exercise missing-host native refusal and hostless metadata-only behavior. Keep
unknown-host/unknown-kind cases within the existing admission policy; test that
the new decision adds no authority rather than inventing a new rejection rule.
Test that the value-copy method returns its explicit true/false decision without
mutating its receiver or other plan facts, that JSON cannot encode/restore it,
and that ordinary operation-local copies retain it. A trusted custom planner
test must demonstrate that single/group usecases consume the generic decision
for another client without knowing OpenCode's representation. Leave the shared
golden normalizer and every frozen fixture unchanged.

The matrix should establish that a matching subset cannot hide an opposite
recognized receipt, while malformed/mixed state still passes through the
existing fail-closed observer/transition checks.

### Real grouped update regression

Use the real planner, stager, activator, native kernel, state store and existing
`opencode.NativeTransitions` composition. Start from an actual V1 native install,
then update the exact same package identity through a V2 prepared host; repeat
for V2 to V1. Include a second independent client target to exercise the ordinary
group path without changing its group admission rules. No real client process
or model invocation is required for this correctness regression.

Assert the old implementation fails the test because it reports a no-op. The
corrected implementation must commit the changed package projection/native
configuration, report mutation for the migrating target, retain logical native
object IDs, acknowledge the target dialect receipts, clear the attempt only
after the existing durable acknowledgement, preserve the owned data locator and
foreign configuration bytes, and leave the independent target's projection and
receipts intact. A following same-dialect update must converge without an extra
package/native transaction. Native config readback must inspect the requested
codec/selector, not merely compare a success enum or the new boolean.

Add negative behavior cases for mixed-source receipts or late foreign edits
using the existing refusal mechanisms. Confirm absence of a durable recorder
cannot become successful migration: it retains the existing explicit refusal
and effect/attempt handling. Do not weaken native containment to run a test.

### Operation and runtime semantics

Cover repeated `Add`/`AddGroup` with an opposite prepared dialect: they must not
replace the recorded projection. Cover representative intact repair behavior
and retain the existing repair/projection-refresh regression suite. Check a
matching-dialect same-revision group update still prepares the existing MCP
runtime and propagates its failure before native effects, while an opposite
dialect uses the existing staged runtime preparation. Reuse current runtime
fixtures or a narrow manager wrapper; avoid a test that only mirrors a new
conditional. Keep no-change lifecycle resume, unresolved attempts, profile
revalidation and component-removal tests applicable.

### Existing single update and recovery proof

Run these current suites unchanged on the corrected tree:

- `TestOpenCodeNativeFacadeDurableRoundTrip`: actual facade V1 to V2 to V1 on the
  same package and stable logical receipt IDs; fresh engine inspection and
  stored removal/recovery remain process-inert.
- `TestOpenCodeNativeFacadeTransitionRefusesForeignFacts`: equal foreign
  destination, edited source, foreign source entries, ambiguous config variants.
- `TestOpenCodeNativeFacadeCrashRecovery`: fresh-facade native/state crash
  recovery and changed-state refusal, with no supplied host or projection bytes.
- `TestNativeTransitionReopensEveryDurableBoundary` and
  `TestProjectOpenCodeTransitionReopensEveryDurableBoundary`: existing durable
  recorder behavior remains exact for user/project contexts.
- The relevant usecase projection-refresh, command projection, runtime preparation,
  selected delivery, and group lifecycle tests, plus architecture guards.

The ephemeral plan decision must not be required during stored recovery. The
existing native receipts, attempt identity, transition record, and state digest
remain its only authority. No stored host probing or fallback profile discovery
is introduced. A missing root or foreign state remains a refusal.

Run focused changed-package tests first, then the core module's relevant
regressions and mandatory lint/architecture gates, including unchanged golden
contracts and `git diff --check`. Test selection should resolve concrete risks;
do not multiply native-client E2E matrices for this read-only plan decision.
Use Go 1.26.10 with the existing shared caches. Native process-containment
limitations must be reported honestly; never disable a safety gate or substitute
a mocked native qualification claim. Exact-head required CI and independent code
review remain merge gates managed by the coordinator.

## Review checkpoints and open choices

1. Coordinator publishes this concrete plan in a draft and obtains fresh
   independent critique before implementation. Correct and record any findings.
2. Confirm the trusted custom-planner contract described above. The chosen
   design reuses `PlanRefiner` and a generic ephemeral field; it intentionally
   avoids a new provider capability or OpenCode-specific compatibility fallback.
3. Confirm the compatibility-preserving missing/unknown/mixed classification
   matrix. The boolean requests staging; it does not classify malformed state as
   eligible or replace existing transition refusals.
4. After the implementation gate, land the regression and bounded change, then
   run the listed behavior and recovery evidence. A fresh critic checks both
   transition directions, update-only consumption, decision provenance, unchanged
   recovery authority, and honest scope/severity reporting.
5. Root manages all commits, pushes, PR publication and merge. No source changes,
   new package-manager/recovery machinery, group-facade widening, or release work
   are authorized by the plan-writing step itself.

### Independent review and encapsulation checkpoint

The independent plan critic `/root/final_review_application_boundaries` approved
the original bounded plan with no requested corrections on 2026-10-30. The critic
explicitly accepted the documented trusted custom-planner tradeoff, existing
`PlanRefiner` reuse, update-only consumption, both recorder-backed transition
directions, no-change runtime checks, preserved repeated add/repair semantics,
and fresh-facade recovery gates. Root published the approved plan as draft PR
[#595](https://github.com/777genius/universal-agent-plugins/pull/595), original plan
head `c9b863c69e80b74857b4a4bfee15d3d6f2dbf6ad`.

The exported-field golden observation above was reported before any golden or
normalizer change. Root selected private storage plus the reader/value-copy
methods as the preferred refinement. The same independent critic explicitly
approved that refinement, including zero/copy/reset and JSON no-restore semantics.
Root published the amended plan alone at
`a9ceee1c4a883a38a1eecc56867fa402b5a65282`, blob
`6ab6f28dc2382d093dd5728f53763434f2e2543f`, before authorizing the source refinement.
The worktree was safely mixed-reset to that plan head after confirming the exact
published plan blob; existing uncommitted implementation/test files were retained.

Acceptance is: both explicit same-revision grouped dialect transitions really
materialize and acknowledge the requested native representation; matching updates
retain their runtime/lifecycle checks; repeated-add/repair semantics and all
existing refusal/recovery boundaries remain exact; generic usecases contain no
OpenCode dialect/object-kind decision after the extraction.

## Implementation and local validation evidence

The implementation follows the amended plan. `DeliveryPlan` stores a private
boolean; the small domain reader/value-copy methods document trusted planner
responsibility. OpenCode's existing refiner computes the decision from the frozen
predecessor objects and prepared host. Single update consumes it in the existing
replace shortcut; group update clears the no-change candidate when replace is
requested and the decision is true. The former OpenCode-specific usecase helper
is removed. No lifecycle, staging/runtime, recorder, facade admission, schema,
capability table, golden fixture, shared golden normalizer, or frozen VS Code
implementation was changed. The separate `group_stage.go` cleanup lane is untouched.

`TestGroupSameRevisionNativeProjectionRefresh` was added before production edits.
It uses two real client targets, real package/data/native/state effects, the
existing native observer, and the existing durable transition recorder. On the
old code **both V1-to-V2 and V2-to-V1 cases failed** because update reported a
completed no-op (`usecase` 0.201s). On the corrected code both pass, including
read-only preview, actual requested-codec readback, changed package projection,
stable logical IDs and data receipt, preserved foreign comments/large integer,
unchanged independent client, cleared pending attempt, and exact stable-dialect
replay without another transaction.

Additional tests cover adapter decisions for matching/opposite/mixed receipts,
missing/unknown host facts, unrecognized receipts, untouched predecessor slices,
and resetting a reused plan. Domain tests cover zero/copy/reset/receiver behavior
and JSON no-restore. A custom trusted planner sets the decision for Cursor and
proves both single/group update consume it without OpenCode identity knowledge.
Real usecase cases prove matching/opposite dialect runtime failure occurs before
native effects, repeated single/group add and intact single repair retain their
old projection, a missing recorder refuses migration, and a late foreign edit
stops managed commit while retaining the foreign bytes.

Final local verification used Go 1.26.10 and the shared Go caches:

| Gate | Result |
| --- | --- |
| Full `./domain` tests | PASS, 0.048s. |
| Full `./clients/opencode` tests | PASS, 2.386s; includes existing user/project durable-boundary recovery and unknown/mixed stored-codec refusal tests. |
| Full `./internal/archtest` tests | PASS, 1.690s. |
| Full `./planner` tests | PASS, 0.051s; unchanged operational plan goldens now pass without fixture changes. |
| Focused `./usecase` projection, runtime, host, command refresh, add/repair, custom planner and group regression selection | PASS, 2.241s on the final refined code. |
| Focused `./installer` existing native facade and projection-refresh selection | PASS, 7.889s; includes durable roundtrip, foreign-fact refusal, and fresh-facade crash recovery. |
| Golden selection in planner/providers/clientdetect/clients-all | PASS, 0.035s / 0.244s / 0.014s / 0.007s; all stored golden bytes remain unchanged. |
| Scoped whole-file size/architecture lint on domain/OpenCode/usecase | PASS, 0 issues with repository golangci-lint 2.13.2. |
| Scoped correctness lint using a complete patch including all new/untracked files | PASS, 0 issues (`--new-from-patch=/tmp/uap-projection-complete.patch`). |
| Baseline and whitespace checks | PASS; 138 lint exclusions remain unchanged; `git diff --check` is clean. Existing already-landed rename advisories require no scope change. |

The final focused test selection was
`Test(GroupSameRevisionNativeProjectionRefresh|GroupProjectionRefresh|NativeProjectionRefreshPreserves|TrustedPlannerProjection|OpenCodeHostBridge|OpenCodeGroupPrepares|OpenCodeNativeFacade|UpdateRefreshesCommandProjection|UpdateGroupRefreshesCommandProjection|RepairGroupRefusesAnOutdatedCommandProjection|RuntimePreparationFailure|RefreshProjection)`
across `./usecase ./installer`. This is targeted evidence, not a claim that every
core/CLI test or real client/model E2E ran.

An initial custom-planner test reused the fixture's already-committed operation
ID and failed with the expected existing replay refusal. Only the new test was
corrected to give its update a distinct ID; the full focused selection then passed.
An initial exported-field golden run exposed the reflection issue that motivated
the separately reviewed/published private-storage refinement above. An ambient
provider golden run, and one broader installer projection-refresh selection,
also encountered the pre-existing duplicate proxy-environment casing refusal in
Codex. Their final passing invocations removed a lowercase proxy key from the
child test process only when its uppercase counterpart existed, as previously
authorized by the coordinator. Product environment validation and the parent
environment were not modified. The first linter invocation could not create its
cache under the read-only home; rerunning with the task-owned writable
`GOLANGCI_LINT_CACHE=/tmp/uap-projection-lint-cache` completed normally.

The source and test changes remain uncommitted for the coordinator's publication
from parent `a9ceee1c`. Fresh independent code review and exact-head required CI
remain outstanding at this checkpoint. No commit, push, merge, publication,
release, native-containment weakening, lint exclusion, or support-scope widening
was performed by this worker.
