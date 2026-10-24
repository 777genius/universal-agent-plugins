# SDK project groups: successful Prepared operation result — R1

2026-10-24. Plan only. Two independent plan rounds precede source edits, then
two frozen code reviews. This is separate from the reviewed grouped mutation
reporting amendment and does not broaden its already frozen source.

## Actual defect and existing contracts

The actual default SDK Cline+Codex group repair regression added in reviewed
source c86859167511dbb3244fab1b3dfe20b3924a501f restores a missing managed
package from its immutable pin, preserves exact native skill/MCP files, and
leaves both bindings Prepared/installation_verified. The core group completes
without an activation error. The SDK instead returns OutcomeIncomplete and
`committed binding is not activated`. Single SDK operations accept this same
prepared file result. Fresh project native loading remains a separate claim.

Source cause: reconcileGroupHostHandoff marks prior Prepared bindings pending,
even without an OnCommittedBinding callback. That conservatively forces the
core to run again, which is useful and must remain. After fresh successful core
completion, finishGroup asks anyGroupHostHandoffPending, which uses the prior
activation-oriented predicate and rejects the newly verified Prepared result.
It therefore conflates an incomplete native session with an incomplete file
installation operation. A no-change retry can also never reach the SDK's
OutcomeUnchanged branch while this pending boolean remains true.

The core already exposes operation-specific evidence: GroupPhaseCompleted,
per-target GroupTargetExternalCompleted, a nil failure, and the actual fresh
ActivationOutcome. These fields describe completion of this operation, not
native runtime activation. Do not reimplement the core verifier/receipt/digest
engine inside SDK presentation or introduce a client-ID switch.

## Minimal proposed correction

1. Preserve initial pending detection, host callback execution, profile checks,
   confirmation and actual group invocation. Do not change the early unchanged
   Apply branch, transactions, observer composition, native attempts, recovery,
   client policies or automatic activation. Native callbacks/errors remain
   authoritative; an error cannot be downgraded to successful preparation.
2. After core completion and existing error handling, recognize only an explicit
   project operation whose fresh GroupResult.Phase is Completed and whose
   nonempty target set matches the requested project target identities. Require
   every target to be ExternalCompleted with no failure; exact project scope and
   selected canonical project context; and a terminal successful operation
   outcome. Pending targets may be Prepared only with installation_verified and
   no activation attestation or uncertain native effect. Other targets may retain
   their already successful Active/NotRequired outcome. Empty, Manual, Failed,
   unknown verification, partial/not-attempted phases, wrong/mixed scope or
   missing/duplicate target identities cannot qualify. No newly invented
   native-discovery evidence, attestation or lifecycle dimension is written.
3. For that actual successful project operation only, the prior pending flag no
   longer forces an Incomplete SDK result or blocks the existing unchanged
   classification. Keep all per-client activation/authentication/policy and
   verification values from the actual outcome and saved state. A mutated file
   operation returns Completed; a truly unchanged reobservation returns Unchanged.
   Reuse the separately reviewed Mutated propagation: no stale preview may
   override an actual effect. User scope retains existing behavior.
4. Preserve the actual per-target UserActions in the existing Result.ManualActions
   array in target order, copying strings without rewriting executable arguments
   or deduplicating distinct client instructions. No new public schema is needed.
   Prepared success must retain the client's native-loading/trust/session limits
   and useful next steps; it does not mean all manual actions are complete.
5. State reload failures, committed uncertainty and host callback failures remain
   outside this success exception. A missing or inconsistent live binding cannot
   establish a completed Prepared handoff. Use the already loaded exact state in
   finishGroup when validating the matching target view; do not load it again
   through an ambiguous boolean that turns unknown state into absence. Preserve
   existing mutation facts even when inspection prevents a completed result.

Keep the shared helper small and presentation-only: the fresh core operation and
existing live binding are trusted evidence from the already verified service.
Do not run new client commands, traverse files, reproduce native receipt checks
or persist state merely to classify a result. Any discovery that requires a
new lifecycle/ownership protocol is a separate plan, not an implementation
shortcut in this slice.

## Required regressions and finite verification

- Upgrade the actual two-target Cline+Codex default SDK repair regression to
  require restored managed bytes, exact untouched native/user/A/B/data state,
  Mutated=true, OutcomeCompleted, both Prepared/Installed and retained actions.
  Its foreign-file branch still refuses before publication with unchanged state.
- Exercise real grouped install, unchanged replay/update/intact repair, actual
  native missing-file repair where the current core supports it, changed source
  update, reopen and removal. Check exact states/receipts and mutation flags. A
  newly exposed underlying repair defect requires its own explicit diagnosis;
  do not force expected results or relax evidence to close this SDK report.
- Cover mixed project targets, missing/duplicate target identities, mismatched
  project context, Manual/Failed/unknown states, incomplete core/target phases,
  uncertain effect, activation attestation, missing/failed reload and actual host
  callback failure. Every refusal preserves real mutation facts and existing
  failure reasons without claiming native convergence.
- Verify unchanged Prepared retries still invoke the real observer/adapter; the
  classification change cannot bypass fresh discovery or ownership admission.
- Preserve existing user-scope grouped handoff, callback, attempt, cancellation
  and mutation-reporting regressions. Run only focused installer/usecase tests
  and the changed-package lint/architecture gates, with the shared pinned Go
  runtime and bounded build concurrency. No vendor process or native gate
  assertion changes are required by this SDK presentation correction.

No implementation has been written for this plan.


## R2 — exact identity and fail-closed presentation after both R1 reviews

Both independent reviewers (APM and Skillshare) gave R1 architecture GO with
concrete admission refinements. This revision incorporates every refinement
before source edits; their final R2 reviews remain required.

The exception is scoped to the final presentation of an actual completed project
operation. It does not change initial handoff detection or skip fresh core work.
The shared helper receives the existing prepared operation, fresh GroupResult,
and the state already loaded by finishGroup. It performs no new I/O, native
commands, persistence, receipt verification, or lifecycle transition.

Exact identity must agree across the frozen prepared target, fresh core delivery
plan and loaded binding: installation ID, binding ID, client ID, physical artifact,
active locator, explicit project scope, canonical project root, original user
configuration root, selected delivery, profile authority and native profile
namespace/root. Use the exact binding ID (derived by the existing domain helper
where the core result lacks a direct field), not the first binding with a matching
client ID. Reject empty, duplicate, missing, extra or inconsistent target identities.
Each target must belong to the same requested installation and project context.
No path canonicalization, discovery or new authority inference is added here;
these are the already canonical values supplied by preparation and the core.

Recognize only positive known completion values. Require GroupPhaseCompleted,
every target GroupTargetExternalCompleted with nil failure, and the existing
successful terminal lifecycle contract. Prepared requires installation_verified,
no ActivationAttested, and the real fresh UserActions retained; it stays Prepared.
Active and NotRequired may retain their successful existing verification contract.
Authentication is limited to known NotRequired, NotChecked, Pending or Authenticated
values; Failed and unknown values refuse. Pending and NotChecked never imply
successful authentication. Require PolicyAllowed: blocked or unknown policy cannot
qualify. NativeEffect must be explicitly Unchanged or Committed; unknown or uncertain
effects refuse. The loaded binding must have an empty NativeActivationAttempt and
nil PendingNativeIntent. Do not introduce new native discovery or runtime evidence.

For project groups, a failed state load or missing/inconsistent exact binding
returns Incomplete/error with the observed Mutated value preserved. It must not
fall through the legacy anyGroupHostHandoffPending boolean, which treats unknown
state as absence. Validate using the already loaded state and retain the existing
core/host callback error when present. The user-scope branch keeps its behavior.
An unqualified project result cannot become Completed merely because no older
Prepared binding was found.

Only that complete exact project result clears the old pending condition for
final SDK result classification. Preserve the separately reviewed actual mutation
propagation and require !result.Mutated before OutcomeUnchanged, even if a stale
prepared plan says NoChange. Copy UserActions from the fresh target outcomes in
target order, unchanged, to Result.ManualActions; never substitute preview actions
or rewrite structured JSON command guidance.

Tests must include exact binding mismatches (including another binding of the
same client), physical/locator/context/profile/delivery mismatch, outstanding
attempt/intent, each rejected unknown/negative dimension, missing/failed loaded
state with and without mutation, and fresh outcome action propagation. Exercise
both successful host callbacks that leave Prepared and failing callbacks, and
prove fresh core observation still executes on a repeat Prepared operation.
The actual Cline+Codex grouped lifecycle and foreign-file refusal remain the
acceptance path; user scope, native attempts, cancellation and recovery regressions
are preserved. No weakening of core guards, assertion meaning or evidence gates
is authorized by this reporting correction.

No production source has been changed for this plan revision.

## Independent final plan approvals

APM and Skillshare independently gave final R2 PLAN GO on frozen
bd721cd3ef1bed12b983bc11575d8afd2e0c800e after reading the full revised
plan and actual SDK/core seams. Both confirmed that their R1 refinements are
incorporated, including positive outcome enums, exact binding authority,
pending-attempt refusal, fail-closed loaded state, real mutation preservation,
fresh actions and callback behavior. Neither review executed a vendor client
or treated presentation completion as native activation. Implementation and
two frozen code reviews follow this approved plan.
