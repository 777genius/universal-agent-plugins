# Mandatory project file ownership with optional native discovery

2026-10-24. Plan only; obtain two independent plan-review rounds before code.
This is a narrow shared composition correction for the authorized project
installation program. It adds no native command, schema, client capability,
configuration writer, or automatic recovery mechanism.

## Reproduced problem

The Cline implementation worker reproduced a foreign project skill collision
through the public SDK with the default EnableNativeObserver=false. The foreign
skill survived and no client ran, but the result was incomplete/Mutated=true and
state-v2.json grew from absent to 4080 bytes. The managed package had committed
before ProjectComponentMutation's requireAbsent refused the native skill.
The diagnostic source/result are retained at the task's cline-observer-gap path.
This diagnostic is not a passing acceptance test.

Engine.service currently composes providers.NativeIdentityObserver only for
EnableNativeObserver && Runner!=nil. Its fallback inspects plan.ActivePath,
which is the managed package, not separately projected component files. Merely
calling ObservePreparedIdentity currently does not fix Cline: its prepared
registry hook validates plan identity, while InspectNativeRegistry receives the
full binding receipts and native kernel needed to inspect actual components.

Local file ownership is necessary admission for project component installation.
The live-registry opt-in must continue to control native process discovery.
Neither a prepared filesystem check nor a file-registry match establishes
native session discovery, activation, or permission to clear an uncertain attempt.

## Concrete design

Reuse the existing NativeIdentityObserver, PreparedIdentityObserver port,
PreparedRegistryInspector, RegistryInspector, native kernel and scope policies.
Do not add client-ID switches, a generic installer, new CLI flags, or another
ownership/path implementation.

1. Before selecting the extra file-inspection path, require matching client
   and plan ClientID, both plan.Scope and TargetContext.Scope equal to project,
   a nonempty exact canonical ProjectRoot and valid original-user TargetContext.
   A malformed project hint must refuse, not select a user/fallback path. For
   an admitted project whose effective scoped lifecycle is LifecycleNativeConfig,
   also inspect
   the adapter's existing file-backed native registry with a copy of the
   observer whose Runner is nil. The exact injected kernel, client authority,
   selected plan and prior binding remain unchanged. This is the same file
   inspector already used by normal project lifecycle admission, including
   symlink/member/digest checks. Native CLI/whole-plugin scope policies retain
   their current prepared-registry behavior.
2. Merge that file finding into the prepared finding using the existing
   conservative ordering: collision, indeterminate, expected, clear. Propagate
   errors. Finish with native registry finding Clear and attempted=false so
   NativeDiscoveryAttempted and NativeDiscoveryReconciled stay false; do not
   launder a file match into native recovery evidence. Existing managed-package
   digest/receipt verification remains necessary. An absent interface or unknown
   file finding remains indeterminate, never clear.
3. In the SDK composition root, preserve the existing fully enabled observer
   when the caller enables it and supplies a runner. When the Engine has an
   explicit project root but live observation is unavailable/disabled, compose a
   small prepared-only observer adapter. Both its port methods delegate to the
   existing provider's prepared path. It receives no command runner. The default
   user-scope compatibility composition remains unchanged.
4. Because normal preview already calls ObservePreparedIdentity, it receives
   the same mandatory project file checks without invoking a host. Ordinary
   saved-state Inspect remains host-free and is not changed. No lifecycle,
   NativeEffect, transaction, receipt, or convergence rule is rewritten. Making
   a formerly nil observer non-nil can enable the existing
   observeGroupRecoveryEligibility branch for SupportsPreparedRecovery. Verify
   this configuration effect explicitly: only known-receipt, exclusively owned
   missing-managed restoration is eligible, with current file collisions refused.
   That existing prepared restoration contract does not claim native discovery
   and does not authorize clearing an unresolved native attempt.
5. Clarify the existing SDK flag documentation: disabling live observation does
   not disable project filesystem ownership admission. Do not invent a second
   flag or an opt-out from foreign-file checks.

The narrow assumption to verify is that a qualified project's native-config
policy describes a file-backed registry. Inspectors receive Runner=nil even if a
caller supplies a live runner. A client that cannot perform the needed check
without a process must refuse, or require a separately reviewed capability
contract; no fallback to a user profile or broad automatic CLI invocation.
Existing concrete adapters remain responsible for validating both scope fields,
project/original-user identity and complete receipts before reading native paths.

## Required validation

- Convert the actual default-SDK Cline foreign-skill example into an acceptance
  regression after integrating the reviewed Cline adapter. Add a foreign MCP
  member case. Exercise both omitted runner and supplied forbidden runner with
  observation disabled, and enabled observation with nil runner. Public
  Prepare/Apply must refuse before managed-package/state/receipt/data publication;
  exact foreign/user/B bytes remain. Temporary preparation snapshots and
  operational lock metadata are distinct from durable installed state.
- Exercise existing owned project lifecycle with observation disabled: initial
  install, unchanged replay, update, missing-component repair, removal and reopen;
  tampered owned skill/MCP must refuse with previous installed state intact.
  Cover Cline plus an already supported representative component adapter rather
  than duplicating the entire source-by-client matrix. Exercise actual public
  group repair of a missing managed directory with live observation disabled,
  known receipts and exclusive restoration; a foreign native component must
  still refuse. Retain the existing prepared-restoration eligibility meaning.
- Provider prepared-observation contracts cover component collision and malformed
  config, exact supplied receipts/kernel, unowned Expected refusal, missing
  inspector/unknown finding fail-closed, and no attempted/reconciled discovery
  bits. Include a project native-CLI adapter with a forbidden runner to prove
  its prepared path still never launches a process.
- Add a real pending-attempt negative regression through normal SDK effects
  (or reuse the genuine process-death fixture), not a forged clean journal.
  Even a valid managed package receipt plus Expected local file observation
  must not clear that attempt or promote lifecycle through the new default
  prepared-only composition. Verify owner recovery refusal and exact retained
  state/bytes, beyond merely asserting that discovery flags are false.
- Retain all original-App namespace/physical-profile guards, user behavior,
  source acquisition rules, activation attestation preflight and crash fencing.
  Existing all-client provider, installer, usecase and public project CLI suites
  exercise the composed union; relevant vet/architecture gates must pass.
- Obtain two independent code reviews at a frozen source checkpoint before merge.
  Fixtures prove local ownership, not vendor discovery or release availability.

## Review record

R1 reviewed exact local 96259f319fb870bec05f4f9dd9fa2794d2b26218,
published with identical tree as 689296a62ea2415b32fd00036cceaac4790df250.

- skillshare_scope: GO. Audited the current qualified project file inspectors,
  including Codex/Copilot whose global user-CLI trait is true. Required explicit
  scope/client admission and a real pending-attempt recovery refusal test.
- claude_codex_scope: GO. Confirmed prepared ports, physical-profile wrappers,
  kernel and recovery boundaries. Added the missing-managed group-restoration
  eligibility case because composing a non-nil observer changes that branch.

Those conditions are incorporated above. No global
UsesNativeRegistryExecutable()==false gate is introduced: scoped policy and
an explicitly runner-free inspection control this path. Missing inspector,
missing kernel and unknown findings remain fail-closed.

R2 independently reviewed exact local
`75e2f12f1ac7c587e817092ad90ce2d4fa9a2dc2`, before production implementation:

- skillshare_scope: GO. Traced groupRecoveryPostApplyVerify and its exact recorded
  digest requirement; prepared restoration does not require or promote native
  reconciliation. The genuine pending-attempt and foreign-component negatives
  remain mandatory implementation gates.
- claude_codex_scope: GO. Rechecked complete scope/client/canonical-root/original
  authority admission, runner-free dispatch, missing capability refusal and the
  documented group-restoration eligibility effect. No remaining plan blocker.

Both review rounds are complete. Production implementation must still pass the
concrete regressions above and two independent frozen-source code reviews.


## Repair admission amendment — R1

The required real public SDK regressions exposed a second boundary while the
observer implementation was still uncommitted. First-install foreign skill/MCP
collisions now refuse before managed/state publication for all three disabled or
unavailable observer configurations. Single-target `Service.Repair`, however,
does not call either identity observer. It verifies the managed package, then
begins a native attempt; a changed Cline/Codex skill is only refused by the
adapter after the binding has been persisted as failed (`Mutated=true`). The
foreign bytes survive, but this does not satisfy the plan's intact-state refusal
requirement. The failing lifecycle regression remains unchanged.

Proposed bounded correction, awaiting two independent amendment review rounds:

1. Add one project-native-config prepared admission helper to the existing repair
   preconditions. Invoke it after exact target/receipt/digest preflight, before
   either intact-native repair or managed-package restoration. Both dry-run and
   confirmed single repair use it, as does the same existing preflight used by
   RefreshProjection. User scope, CLI-registry and prepared-plugin policies keep
   their current paths. No lifecycle outcome, transaction, native-attempt, schema,
   registry writer or capability declaration changes.
2. Require the existing PreparedIdentityObserver capability for that project
   file policy; no nil/fallback package-only observation may stand in for local
   component ownership. Call through the existing prepared observer and complete
   frozen plan/binding so the independently supplied kernel, root and receipt
   checks remain authoritative. The prepared path never invokes a process.
3. Compare the whole prepared observation with the already measured managed
   package condition. For an intact package require Managed with the same
   nonempty receipt digest, via the existing identity validator. For a known
   absent managed package permit only Absent. For a known digest mismatch permit
   only Indeterminate carrying exactly the same nonempty measured actual digest.
   The provider attaches a digest only after local component findings are clear
   or expected; collision/unknown/malformed local findings cannot supply that
   managed-drift proof. Arbitrary errors, an unmatched state/digest, or a changed
   package since the first verification refuse without state publication.
4. A missing exact-owned native component remains repairable: its clear local
   finding plus the intact managed package still produces Managed. Existing
   controlled managed-directory restoration, receipt validation and commit-time
   preconditions remain unchanged. Do not treat a known digest mismatch as
   native discovery or use it to clear uncertainty. Existing unresolved-attempt
   recovery fences still precede this branch.
5. Retain the concrete Cline+Codex tests: intact state on tampered skill and MCP
   refusal, normal missing-native repair, managed drift/absence restoration,
   real two-target group repair with known receipts and foreign-native refusal,
   and a genuine callback-created pending attempt whose managed digest and local
   native files both verify but still cannot recover. Add any gap identified by
   independent critics before implementing this amendment.

This amendment is a precondition addition required by observed behavior. It does
not relax the original plan's separation of filesystem ownership and native
activation evidence. Production repair code has not been edited.
