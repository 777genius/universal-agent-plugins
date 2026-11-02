# Prepared project files: preserve fresh verifier evidence during repair — R1

2026-10-24. Separate bounded core convergence plan exposed by the twice-reviewed
Qoder Prepared adapter acceptance path. Two independent plan rounds precede any
shared source edit; two frozen code reviews follow. The Qoder adapter and its
strong failing assertions remain intact. No generic source has been changed.

## Actual result and exact call graph

The composed Qoder provider contracts pass0.129s, including selected command and
skill assets, a closed importable view, neutral native discovery and a forbidden
runner. The public SDK single lifecycle passes2.224s for default and observer
compositions, but its initial test checked repaired bytes without rechecking the
saved verification enum. The actual saved TLS Git CLI journey's stronger check
fails after drift repair: exact pinned bundle is restored, activation remains
Prepared, native/user/B files remain independent, and saved verification is
package_validated rather than installation_verified. This is a lifecycle evidence
gap, not failed file restoration or proof of native registration.

Current usecase path:

1. Repair verifies current target/recorded ownership and resolves the caller's
   exact source pin. A modified or absent managed package goes to repairPackage.
2. commitRepairPackage sets verifiedState.Verification=VerificationPackageValid,
   then commitRepairDirectory performs the existing verified transaction and
   persists the exact new managed package receipt plus real Mutated=true.
3. reactivateRepaired reactivates nativeLifecycleClient routes, or calls
   verifyRepairedLauncher for UsesManagedStdioLauncher. A scoped LifecyclePrepared
   route does neither, so its exact file verifier is never invoked and the
   package-only state remains. Do not falsely declare native or stdio traits.
4. Intact repair goes through repairNative, activeNativeDelivery and
   nonNativeRegistrationComplete (true for a file-only Prepared route), then
   verifyClientReadOnly. That method first uses the static domain
   ShouldReadOnlyVerifyForScope gate. LifecyclePrepared has no dedicated branch;
   Qoder's inherited auth-hint branch only admits a nonempty executable. The
   actual runner-free project contract has an empty native executable and does
   not need a client binary. The existing actual VerifierAvailable capability is
   not consulted to enable this file-only route, so intact repair after the
   downgrade cannot restore fresh file-verifier evidence either.
5. Ordinary repeat add/update already invokes the real adapter and uses existing
   verifiedPreparedObservationUnchanged only after exact receipt/lifecycle facts
   agree. Preserve that guard and the separate SDK handoff correction.

The public SDK requires an explicit PackageRoot; an advanced explicit root is
correctly rejected by its active-revision guard. The SDK test now retains that
refusal and supplies an immutable captured pin for repair. The saved CLI exercises
its actual source/cache resolver with a later advanced Git source. Do not remove
or relax either revision boundary.

## Minimal correction

Add one private operation predicate in usecase, with no client-ID switch or new
port/schema. It requires explicit project input and plan scope, a matching
project TargetContext with nonempty selected root and original user root equal
to the actual client, matching plan/client IDs, zero SelectedDelivery, and
scoped LifecyclePrepared with no tracked native effects. Require the existing
ActivationVerifierClassifier.VerifierAvailable for this exact client/plan/backend.
Missing or false capability must preserve the existing fallback; it cannot grant
verification. No path canonicalization, filesystem traversal, executable check,
new client discovery or authority inference belongs in this predicate.

Use this predicate at precisely two existing seams:

- In verifyClientReadOnly, permit the existing read-only activation path for this
  positive file-verifier contract even when static ShouldReadOnlyVerifyForScope
  is false. Keep the existing static gate and behavior for every other route.
  Pass the exact recorded managed delivery/objects and VerifyOnly=true as today.
- After commitRepairDirectory succeeds, reactivateRepaired may use the existing
  read-only verification/persistence shape for this contract. Name any extracted
  helper for its actual read-only purpose rather than pretending that the bundle
  is a stdio launcher. Pass the freshly committed delivery's exact active path,
  owned base, artifact digest and managed receipt, current plan/context, unchanged
  original client and preserved authentication. Request VerifyOnly=true and do
  not create a native attempt or execute an acting activation path. Keep native
  and launcher branches otherwise unchanged.

Only the adapter's actual successful file verification may return Prepared plus
installation_verified; do not synthesize those values from directory existence,
previous state or a true capability boolean. Preserve the adapter's fresh actions
and remaining native import/discovery limits. Keep the new managed package
receipt and all existing attempt/intent fences. Never adopt arbitrary returned
native objects or convert a NativeEffectCommitted/Uncertain/unknown report into
an Unchanged claim. Preserve real mutation facts even if the verifier or state
persistence fails; a post-commit verification failure remains incomplete/error
with the repaired package retained and no fabricated native completion. Reuse
existing lifecycle/error machinery; a newly discovered effect/recovery protocol
gap requires another explicit plan rather than a shortcut here.

User, project CLIRegistry, tracked NativeConfig, nonzero selected delivery,
unsupported/false-verifier and all other lifecycle behavior remain unchanged.
Grok's current project verifier reports unavailable, so this change must not turn
its source/package evidence into installation_verified or native discovery.
No flags, commands, classifiers, timeouts, workflows or user/native parsers change.

## Required finite acceptance

1. Strengthen the real Qoder SDK lifecycle assertions after every drift/missing
   repair and reopen: exact immutable bundle, Prepared/installation_verified,
   original binding identity/current managed receipt, useful fresh actions,
   NativeDiscoveryAttempted=false/Reconciled=false and zero forbidden native calls.
   Every repeat add/unchanged update/intact repair freshly verifies and leaves
   exact state bytes and Mutated/NoChange truthful.
2. The existing actual saved TLS Git CLI journey must pass its unchanged strong
   post-repair enum/bytes assertion after source advancement. Keep deps sync
   --locked, distinct user and A/B state, host-free status, changed update and
   exact removal/data preservation. Do not insert a separate add/update between
   repair and its verification assertion.
3. Focused core regressions use an existing injected verifier seam to prove the
   exact fresh delivery/context/receipt and VerifyOnly=true reach the verifier;
   failed verification and failed lifecycle persistence keep real Mutated=true
   and error, without new native ownership/attempt acknowledgement. Missing or
   false capability, malformed scope/client/context, nonzero delivery, user,
   CLIRegistry, tracked effects and actual acting requests cannot enter this
   exception. A verifier declaring unavailable must not be called.
4. Keep real pending-attempt/recovery and known foreign/drift refusal tests;
   successful file verification does not grant recovery eligibility or native
   convergence. Existing Grok, user Qoder and launcher behavior stay unchanged.
5. Run only focused Qoder provider/SDK/CLI and core cases plus small architecture
   and changed-source lint gates, with the shared pinned Go runtime/concurrency1
   after capacity is granted. No vendor process, MCP, trust/model/auth operation
   or broad module suite is needed. Record unsupported local process prerequisites
   honestly if the separately reviewed containment change is integrated.

This correction qualifies installed bundle files only. Actual Qoder registration,
loaded selection, shared cache ownership, MCP, trust, authentication and restart
remain unqualified exactly as the native evidence and adapter plan state.

## R2 — exact saved identity and no contradictory read-only success

Root and Claude independently gave R1 architecture GO on exactb6106274 after
reading the plan and actual core call graph. Both required an explicit outcome
gate: the existing zero-selection read-only wrapper delegates to the adapter,
and correctIntactLifecycle otherwise ignores NativeEffect. A Committed report
with unchanged saved dimensions could therefore be misreported as NoChange.
This correction will not rely on VerifierAvailable alone to establish success.

Distinguish applicability, valid authority and availability. The private helper
first recognizes an explicitly project-signaled route whose actual client's
scoped policy is LifecyclePrepared without tracked native effects. Any project
signal includes input/plan project scope, TargetContext project scope or a nonempty
ProjectRoot. For this candidate, mismatched scope/client/context, nonzero selection
or inconsistent saved identity is an error, never permission to fall through to
the inherited static executable/auth-hint gate. A valid candidate with missing or
false verifier capability performs no new verifier call. User and other lifecycle
routes retain their current branches and do not borrow this exception.

For a candidate require input.Scope and plan.Scope and TargetContext.Scope all
project, plan.ClientID equal actual client, the already selected nonempty clean
project root and original UserConfigRoot equal actual ConfigRoot. AddInput has no
independent ProjectRoot field: use its existing plan/target-resolver authority,
without inventing another root or doing filesystem canonicalization here. Require
zero selected delivery. Saved binding must have matching client/project scope,
exact computed binding ID/active locator/physical artifact/native profile and
namespace/authority under the current plan, ActivationPrepared, no outstanding
NativeActivationAttempt or PendingNativeIntent. The managed delivery must match
the exact saved package path, digest and complete managed receipt set. The
post-commit branch uses the freshly committed binding/delivery held by the repair
session, not stale predecessor objects. This is additional matching of existing
facts, not adoption of supplied paths or independent digest verification.

The new positive verifier route has a concrete success contract after preserving
existing authentication: no error, ActivationPrepared, VerificationInstalled,
PolicyAllowed, unchanged saved authentication, no activation/authentication
attestation, no AuthoritativeObservation, nil LocalEntryObservation, no returned
NativeObjects, and nonempty fresh UserActions. NativeEffect must be exactly
Unchanged or the existing empty legacy value for this declared nontracking
Prepared contract. Empty means not reported, not inferred Unchanged. No native
discovery observation is created. Grok's unavailable verifier still cannot gain
file-installed or runtime evidence.

Before any lifecycle-success or NoChange classification, reject Committed,
Uncertain and unknown nonempty effects and every contradictory outcome above.
Never rewrite the reported NativeEffect or adopt its returned objects. A reported
Committed effect sets/retains result.Mutated=true even when an error is returned
or saved lifecycle fields are identical; no result may simultaneously claim
NoChange. Uncertain/unknown effects remain visible in the returned actual outcome
and an explicit error with NoChange=false; they do not manufacture certainty or
new ownership/recovery authority. Existing known mutation facts remain unchanged.
This is a contract-violation report, not native reconciliation or a successful
recovery. No attempt/intent is cleared and no automatic recovery is authorized.

For an intact-package verifier failure, preserve the complete observed outcome in
the operation result and return error before correctIntactLifecycle or acting
fallback. Use a small private error classification if needed so existing
repairNative error handling cannot reapply or discard this exact route's observed
effect. Do not change errors for other routes. For a post-commit failure retain
Mutated=true, the exact repaired bundle and already committed PackageValid state,
and return incomplete/error without persisting a manufactured successful
lifecycle. A genuine adapter verification error retains its error cause. A failed
lifecycle persistence likewise cannot erase the actual package mutation.

Both seam paths require focused contradictory-outcome regressions, including
Committed with identical lifecycle values and an actual inert effect marker,
Uncertain/unknown, native objects, LocalEntryObservation, both attestations,
authoritative negative observation, missing actions and wrong lifecycle/policy/auth.
Assert exact saved-state/receipt preservation, actual effect reporting and no
acting retry, plus post-commit mutation retention. Add explicit saved binding
scope/client/locator/digest/receipt mismatch and outstanding attempt/intent cases.
Retain the complete actual SDK and saved CLI journeys; strengthen the SDK's
post-repair binding assertions rather than accepting its earlier byte-only pass.

Only the existing two verification seams and the precise error propagation they
require are in scope. No generic native-effect/recovery protocol, new domain
trait, interface, schema, launcher behavior or client-ID switch is authorized.
No shared production source has been changed. Both independent final R2 GOs are
required before implementation.

## Final plan approvals before implementation

Root and Claude independently gave final R2 PLAN GO on exact
3ffa62e1a8724d682148892b38e0a83debd5ac77 before shared source edits.
Both confirmed exact candidate/authority/capability separation, the strict real
outcome gate before correction, explicit committed-effect mutation retention,
no object adoption or attempt clearing, and post-commit failure truth.

## Candidate verification before frozen code review

The private core helper now validates the existing configured Service.ProjectRoot
against the selected plan root as part of the same frozen authority comparison.
Intact verification and post-commit verification use exact managed delivery and
saved objects. Typed refusals preserve the observed outcome and prevent both
repair's acting fallback and the existing authoritative-preview persistence path
from consuming a contradictory file-only report. No public contract, domain
trait or client-ID branch was added.

Go1.26.10, GOMAXPROCS3 and -p=1 focused checks passed: core contradiction/identity/
capability/persistence tests0.024s; Qoder composed provider tests0.139s; real public
SDK lifecycle2.010s; actual saved CLI/TLS Git lifecycle5.708s. SDK default and
enabled-observer compositions both use a forbidden native runner. Strong direct
post-repair Prepared/installation_verified assertions now pass after drift and
whole-package absence, with immutable-pin restoration despite advanced source.
Changed source update, state-stable repeat/intact repair, host-free status and
user/A/B/caller-data preservation are retained. Genuine raw Qoder global-list
bytes remain a conservative duplicate-ID user-parser refusal fixture.

These are source/inert-client lifecycle checks, not new Qoder native runtime
qualification. Full/changed-source lint, platform compilation and independent
frozen code review remain separate gates; no vendor, trust, authentication,
model or MCP command ran locally for this implementation.

## Independent code review: mutable packet correction

Root's frozen c317 review identified a concrete input-alias risk: an injected
capability classifier or verifier could edit an ownership slice shared with the
frozen repair binding, then return an otherwise valid file result. The finite
regression against unchanged c317 production failed0.014s in all four combinations
of classifier/activator and intact/post-commit verification. The intact case
corrects PackageValid to Installed and can persist the altered receipt.

The narrow correction gives capability classification and activation separately
cloned ownership/plan/observation/authority/client packets. A classifier's changes
cannot become the activator's authority. The returned transient guidance and
observations are copied as well. Existing immutable selected delivery and
profile namespaces keep their semantics; no schema or recovery protocol changes.
The complete focused PreparedProjectFile suite, including the four mutation
regressions and prior contradictory-effect/failure cases, passed0.026s after this
correction. No native command or model/MCP/trust action ran. Final independent
source reviews and exact-source static/platform gates remain required.

## Final ownership isolation review and verification

Root's independent review of frozen `c3170ac5` identified a real mutable input receipt alias. The added four-case regression failed on that unchanged source for intact lifecycle correction/post-commit verification and classifier/activator mutation. Correction `96117cd43064e50a4541387d5dc9a3aeddbdadeb` supplies separate copied classifier and activation authority/observation/ownership packets and freezes returned transient data; it neither adopts verifier objects nor changes state schemas. Root gave final full CODE GO after tracing the correction, and Skillshare independently gave full CODE GO for the entire Qoder/core source plus the later private Qoder-only mechanical delta. Exact final-source meaningful regressions, focused vet, configured changed-source lint, architecture/conformance and Windows SDK compile-only results are recorded in `qoder-project-lint-extraction-plan.md`. No vendor or native activation evidence is implied.
