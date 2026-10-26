# Preserve a verified project file reobservation at the SDK handoff — R1

The actual composed SDK Cline+Codex lifecycle regression exposed a remaining
underlying defect while validating Prepared group completion. A repeated install
reported Mutated=true although saved state and native file bytes were identical.
The real Cline adapter observed PackageUnchanged=true, VerifyOnly=false and
NativeEffectCommitted. Do not suppress this actual reported effect in presentation.
The current complete-native-config contract reports committed when its Activate
path is invoked; identical bytes alone do not prove absence of file writes.

Source cause: installer/seams.go seamActivator.Activate unconditionally clears
VerifyOnly for a resumed binding whose activation is Prepared. The core already
computed VerifyOnly=true after verifying the installed package, recomputing the
complete native projection and matching its owned receipts. The generic handoff
predicate confuses project files being verified with native session activation.

Keep initial pending detection, actual host callbacks, profile revalidation,
confirmation, ownership admission, native attempt/recovery guards, and core
invocation unchanged. In the existing resume branch only, preserve an incoming
VerifyOnly=true when PackageUnchanged is true, the delivery is explicit project
scope with matching project TargetContext, and its existing scoped traits declare
LifecycleNativeConfig. Use no client ID list and infer no native discovery.
The existing core read-only decision is the authority; the SDK must not invent it.

Do not turn any false VerifyOnly into true, skip the fresh adapter verifier,
clear an attempt/intent, grant activation, or modify receipts/state to make an
operation appear unchanged. User scope and project CLIRegistry/Prepared/manual
lifecycles retain their existing handoff behavior. Changed/missing projections
continue through the core's existing acting path and report their real effect.

The actual public SDK group test must cover install, replay/update/intact repair,
changed source update, missing native-file repair, callback success leaving
Prepared, callback failure, reopen and remove, with original user/B state intact.
Add a narrow seam regression distinguishing preserved project-native-config
VerifyOnly from user scope, other project lifecycle kinds and actual acting
requests. Existing selected-delivery, pending attempt and handoff regressions
remain required. The presentation completion correction stays separate; its
Mutated=true and failed-load evidence must never be overridden by this change.

Two independent plan rounds precede this narrowly scoped source edit and two
frozen code reviews follow. No production source for this handoff correction
has been changed.

## R2 — exact predicate and preserved group authority

APM and Skillshare independently gave architecture R1 GO exactee36799f after
reading the actual SDK/core call sites. Incorporate both sets of refinements:
the exception is inside the existing resume branch and requires incoming true
VerifyOnly, PackageUnchanged, zero SelectedDelivery, matching Client/Plan IDs,
explicit project Plan.Scope and TargetContext.Scope, a nonempty canonical project
root and original TargetContext.UserConfigRoot matching the actual client. The
existing scoped trait must be LifecycleNativeConfig. Do not canonicalize paths
or infer new filesystem/profile authority in this pure decision helper.

In grouped operations a.facts can describe the first target. Compare the current
request's client, plan and target context; never require each client to equal the
first group's a.facts.ClientID/BindingID. All already established core ownership,
receipt and selected projection checks remain authoritative. No new bypass for
unknown/missing state or unresolved attempts is introduced.

Tests cover incoming false, missing/mixed scope/root/user context, client-plan
mismatch and nonzero selected delivery. User scope, project CLIRegistry and
Prepared lifecycles retain their existing acting behavior. Actual SDK group and
single replay must freshly verify, successful callback must still precede the
verifier/profile recheck, failed callback must retain its failure, and missing or
changed native projections must stay acting or refuse under existing ownership
rules. Preserve actual native effects and all existing pending/recovery tests.

No production seam source has been edited for this correction. Final independent
R2 reviews precede that edit.

## Implementation and focused acceptance

Both APM and Skillshare gave final R2 PLAN GO on exact49028da815077c4ce9793bad271a5d5655032f0e
before production edits. The source exception preserves only incoming VerifyOnly
for an unchanged zero-selected explicit project NativeConfig request whose
client, validated TargetContext and original user root agree. It uses existing
scoped traits and NewTargetContext (pure lexical validation), without IO or a
client-ID switch. Acting requests, user/CLIRegistry/Prepared lifecycles and
selected-delivery confirmation retain their previous routes.

Focused installer acceptance on the assembled source passed3.667s: actual group
and single file replay, narrow real seam/provider dispatch, strict Prepared group
reporting, existing committed-callback retry and file lifecycle regressions.
The actual Cline+Codex group covers fresh read-only repeated install/update,
intact repair, callback success/failure, changed update, partial native-directory
refusal, positive whole-directory absence repair, reopen and per-client removal.
Original user/B files remain identical and the forbidden runner is never called.
The single replay checks the actual adapter request, observation, state and file
bytes; existing completed-with-no-mutation presentation is preserved.

Group Repair intentionally restages managed packages under the existing core
contract (!Repair in the unchanged predicate); its real state/receipt mutation is
reported Completed/Mutated even if native bytes match. Removing just SKILL.md
from a still-present owned native directory is integrity drift and remains a
refusal; removing the entire independently owned object is typed absence and is
repairable. Neither contract was relaxed to satisfy a test. Group removal's
historical SDK beta guard has a separate twice-reviewed admission plan; this
handoff test uses existing public single removals until that slice is complete.

The focused seam test preserves user/project-CLI/project-Prepared behavior,
incoming false, malformed/mixed context, client mismatch and selected-delivery
refusal. Profile failure remains fatal and a resumed request never repeats the
post-commit callback here. Group facts deliberately identify another client so
the exception cannot accidentally be tied to the first target. Architecture and
changed-source lint results will be recorded separately, before publication.
