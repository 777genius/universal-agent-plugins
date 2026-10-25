# Group result error classification extraction — R1

The bounded full-file installer architecture lint on the assembled source reports
only finishGroup: 47 statements (limit40) and cyclomatic complexity24 (limit20).
The pure exact project completion helper and read-only handoff predicate pass
these guards. Do not alter budgets, exclusions, error meaning or project checks.

Extract exactly the existing `if err != nil` body in finishGroup into a private
`failedGroupResult(result Result, err error) (Result, error)` function in the same
file. The caller remains at the exact same point after wrapLifecycleError, state
load, live result attachment and fresh attempt/actions attachment, and returns
the helper only when that already-wrapped error is nonnil. It precedes all
project-success, host-pending and no-change classification exactly as today.

The helper preserves current field updates and order: start Incomplete with the
original error message; ErrPlanChanged selects Conflict/plan_changed; the
existing update-separately marker or ErrProjectionRefreshRequired selects
Conflict/update_required; ErrProjectionPinChangeRequired is checked last and
selects Conflict/pin_change_required. These remain independent ordered ifs,
not an exclusive switch that would alter multi-error precedence. Return the
same error object. Retain all already observed Mutated, binding, target and
fresh guidance fields by passing/returning the full result value. No IO, state,
context, callbacks or observations move into this helper.

No new generic error framework or public API is introduced. The existing project
completion and user pending paths are unchanged. Run the current complete
TestFinishProjectGroup/TestFinishGroupPreservesActualMutationFacts and real
project group lifecycle tests, plus the existing conflict/error regressions.
If current tests do not pin the ordered conflicting-error contract, add one
small result-level table for exact Reason/Outcome/error identity and preserved
mutation/targets/actions, rather than tests of helper layout. Re-run bounded
installer full-file architecture lint and changed-source correctness lint.
Two independent plan rounds precede the extraction and two frozen code reviews
follow; this plan does not claim the current lint failure is already fixed.

## R2 — exact wrapped identity and overlapping classification

APM and Claude independently gave R1 PLAN GO on exact3bd3f4a2181eefb7c1130c4477148c0fb9882fb2
after reading the whole current finishGroup and this plan. The helper receives
and returns the same already-wrapped nonnil error object; finishGroup's existing
wrapLifecycleError may wrap its original input before any state read, and that
behavior is preserved. No nil-helper call is introduced. No load-error branch,
attachment, project completion, pending logic or reporting call moves.

Add a small actual finishGroup result table using joined errors: PlanChanged
alone selects plan_changed; PlanChanged plus ProjectionRefreshRequired selects
update_required; either plus ProjectionPinChangeRequired selects pin_change_required.
The existing textual update marker keeps its same position. Assert the same
wrapped identity/cause, known mutation (never NoChange), live target/failure facts
and freshly attached project manual guidance survive every classification.
This pins the externally reported precedence, not the private function layout.
No production extraction has happened. Obtain both final R2 plan GOs first.
