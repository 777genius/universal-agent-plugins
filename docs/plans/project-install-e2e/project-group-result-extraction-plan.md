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
