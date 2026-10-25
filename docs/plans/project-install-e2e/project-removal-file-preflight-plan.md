# Project removal: observe owned native files before an attempt — R2

The new real Cline+Codex grouped removal test exposed a specific pre-existing
SDK gap. A foreign replacement of an owned native skill is preserved and removal
fails, but only inside confirmed deactivation, after core beginNativeAttempt has
persisted a pending attempt. Saved state changes although no native file effect
was performed. The existing single/group SDK removalPreflight validates the
managed package and selected-delivery entries; historical zero-selected project
component files are not observed there. Do not weaken the foreign-file test,
clear the attempt after failure or change generic effect/recovery semantics.

## Reuse the already mandatory prepared observation

Add a private installer helper called from existing removalPreflight after the
independently resolved exact target and managed package digest have passed, and
before PLUGIN_DATA checks finish. Limit it to explicit configured project scope,
zero selected delivery and existing scoped LifecycleNativeConfig. Other scopes,
Prepared/CLIRegistry lifecycles and selected native-entry preflight retain their
existing paths. No client ID list, new port/schema or native executable.

Build only the observation DeliveryPlan from the existing resolved DeliveryTarget
and saved binding: exact target context/root/anchor/active/native root, client and
project scope, physical artifact, saved selected delivery, saved previous native
objects and existing profile authority/namespace. Thread the actual installation
DeclaredName through the existing private removalPreflight call sites; do not
invent a name, reparse moving source or re-plan desired components. Components
are empty because removal inspects the recorded set only. Existing adapter
ProjectComponents.Inspect first verifies every prior object with allowMissing;
unrelated native entries remain outside ownership and are preserved.

Invoke the existing providers.NativeIdentityObserver.ObservePreparedIdentity with
real Stager, nativeconfig kernel and registry, with Runner omitted. It already
validates canonical project/original user context and combines prepared registry
and scoped file ownership. Require no error, NativeIdentityManaged, exact recorded
managed digest and ReceiptReconciled, with NativeDiscoveryState exactly
NativeIdentityAbsent and neither NativeDiscoveryAttempted nor
NativeDiscoveryReconciled. An empty or unknown discovery state is not absence. Unknown, collision, mismatch, unavailable inspector or
read errors refuse. Missing native objects retain existing allowed absence;
a missing/changed managed package still refuses under the earlier package guard.
No observation becomes native discovery or runtime/activation evidence.

All existing Prepare, Apply, under-lock confirmation and immediately-pre-dispatch
calls go through the same removalPreflight helper, so this check repeats at each
existing fence. Thread the name through confirmRemovalPreflight using its exact
approved installation from already-loaded state, not an extra mutable lookup.
Clone the supplied ownership slice and observation/authority values so an
inspector cannot mutate the frozen in-memory comparison basis. Keep existing
whole-state/journal callback comparisons and profile checks in place.

This is a repeated read-only fence, not exclusion of every concurrent filesystem
writer. A foreign change after the last pre-attempt observation may still be
refused inside the native adapter after an attempt has been persisted. Preserve
that pending evidence and uncertainty; never clear it to manufacture a no-change
result. The no-attempt acceptance cases concern foreign content already present
at preparation or introduced after Prepare but before Apply observes the files.

## Finite acceptance

- Real single and grouped Cline/Codex removal reject foreign native skill or MCP
  bytes before any pending attempt, saved-state/journal change or deactivation;
  original user/B and every unrelated native member remain identical, runner0.
- Repeat these failures after Prepare and with the second group target foreign,
  proving no first target is removed before the later target is checked.
- Missing owned native objects can still be removed when the managed package is
  intact; actual normal, one-absent and repeated group cleanup remain passing.
- Keep actual managed-package drift, profile/binding changes, pending journal,
  missing PLUGIN_DATA and selected-delivery guards. A swapped saved profile that
  remains inside the project must be refused; outside-project receipt saving is
  already rejected by the store and is not bypassed for a test.
- Narrow recording-inspector negatives check exact context/receipts/no runner,
  malformed/unknown findings, and unchanged nonproject/nonNativeConfig routes.
- Full-file installer architecture and changed-source correctness lint remain
  mandatory. This is early detection of existing refusal, not new removal
  authority or a generic native-effect protocol correction.

Two independent plan rounds precede this shared preflight source edit, and two
frozen code reviews follow. The group admission change remains a separate slice;
its strong foreign-file acceptance is currently failing as described above.

## Independent R1 critique resolved

Both APM and Skillshare reviewed R1 commit ce4c9443e49986f03120bbc789f72afe7c769204
and gave architecture GO. R2 makes the positive absent discovery-state requirement
explicit and limits the no-attempt claim to the actual observation boundaries.
No source or native effect contract changed during either plan review.
