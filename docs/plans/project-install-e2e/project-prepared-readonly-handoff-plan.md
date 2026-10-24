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
