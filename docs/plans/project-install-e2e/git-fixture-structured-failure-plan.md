# Immutable Git fixtures: preserve closed failure categories

2026-10-25. This diagnostic amendment keeps command execution and failure
decisions unchanged. ROOT owns the ordinary-supervisor carrier and focused
workflow; CI/SDK owns the Git-exit carrier and fixture classification.

## Observed failure and information boundary

At `d6809d5438654ccdb47bd8d241b96b44e91515c8`, Coverage run `40187422585`,
job `121151062112`, fails `TestDependencyEnvironmentCLIEndToEnd` at
`deps_e2e_test.go:433`, during `deps update demo`. The existing diagnostic
proves only `phase=fetch class=other_git_or_process`. The earlier integration's
different graph fixture failed at the same public acquisition boundary.

The four typed classes currently available cannot distinguish a real Git exit
from process supervision failure. The Linux child serializes its own process
failure into `linuxOrdinarySupervisorResult.Error`; its receiver uses
`errors.New`, losing structure. A clean nonzero Git exit is also currently
formatted into an opaque error by `sourceacquisition.OSRunner`.

Do not inspect these error strings to recover a category: they may contain
arguments or streams. Existing logger tests deliberately panic if `Error()` is
called. The local preflight's missing locked-thread observation is a separate
observed host condition, not an explanation for the hosted fetch failure.

## A. Origin-owned ordinary-supervisor category

- Add one optional closed stage to the existing private supervisor result:
  request decoding, subreaper setup, ordinary preflight, command start, or
  descendant cleanup. Assign it only at those existing failure branches.
- On receipt of a nonempty existing Error, preserve its exact message and
  failure status. A private typed carrier retains only a recognized stage;
  unknown or missing stages remain failing and unclassified. Do not infer a
  stage from Error text or allow a category to replace the failure.
- A nonempty stage with an empty Error is contradictory and must fail status
  admission; it cannot establish successful execution or cleanup.
- Expose `process.OrdinarySupervisorFailureClass(error) string`, which uses
  structural matching and returns a fixed known label or the empty string.
  It must never call `Error()` or inspect command/output/environment text.
  The helper builds on every GOOS: only the Linux supervisor produces these
  carriers, and unrelated errors on any platform return the empty class.
- Preserve existing child exit handling, forced-cleanup causality, stable
  handles, status/EOF/trailing-data checks, limits, deadlines and fail-closed
  admission. Existing error strings and unwrap/Is behavior remain unchanged
  except the additive structural category.
- Add inert producer/receiver/category contracts for all stages, legacy missing
  stage, unknown stage, contradictory success and exact message preservation.
  Existing process safety tests remain intact.

## B. Positive Git exit category

- In only OSRunner's two existing clean nonzero-result branches, wrap the exact
  existing formatted error in a private carrier with
  `GitTransportExitCode() int`. Its Error delegates to the original formatted
  error; Unwrap retains that original error. Do not change the process-error,
  context, output-limit, success, environment or command branches.
- The distinctive method prevents a generic supervisor `exec.ExitError` from
  being mistaken for a positively observed clean Git exit. Keep the original
  integer exit code without imposing a new admission range or decision.
- Extend the existing test-only fixture classifiers using structural checks:
  retain current context/unsupported/stdout classes, recognize the process
  helper's fixed supervisor class, then recognize the distinctive Git-exit
  interface through `errors.As`. Emit only a fixed Git-exit label and integer
  status; no formatted error or streams. Preserve existing diagnostic caps,
  delegation count, command/context and exact returned bytes/error identity.
- Inert contracts cover code and unwrap preservation, native/nonnative message
  equivalence, and logger classification of an exit carrier whose Error panics.
  Existing secret/argv/output traps, unknown class and silence tests remain.

## Finite hosted observation and verification

After two independent plan rounds and frozen code reviews, add one narrow
current-source workflow using the same Linux runner selection and Go 1.26.10
as Required. Its sole test invocation selects exactly
`TestDependencyEnvironmentCLIEndToEnd` and
`TestInstallDependencyGraphLifecycle`, with `-count=1` and a bounded test/job
deadline. Keep ordinary process containment and all existing assertions.
No retries, repeat-until-pass loop, alternate process runner, credentials or
additional output/environment authority is permitted. The exact workflow delta
is reviewed with the implementation before one genuine hosted observation.

Run the existing relevant process and source-acquisition contracts, both inert
diagnostic suites and configured changed-source lint. Preserve exact head/tree,
job and finite output from the focused run. A passing attempt does not identify
the historical cause; a failing attempt can establish only the observed fixed
category and integer status. Any concrete behavior correction requires its own
reviewed evidence. Full final Required/Coverage/Core/native gates remain.

## R2 amendment: preserve the existing forced-cleanup failure category

`runExplicit` also fails when `termination.forcedMembers` is true, even if the
supervisor's Error is empty. ROOT will add `process.ErrOrdinaryForcedCleanup`
with the exact current text, `process left live descendants that required forced
cleanup`, and substitute it only for the constant `fmt.Errorf` in that existing
ordinary-runner branch. Preserve the surrounding joins, branch, status and timing;
leave the duplex runner unchanged. An inert contract verifies exact text and
`errors.Is` identity. CI/SDK adds an `errors.Is` fixture classification returning
the fixed `ordinary_forced_cleanup` label. Existing context/output precedence,
four-record caps, original delegate results and all no-Error-read traps remain.
