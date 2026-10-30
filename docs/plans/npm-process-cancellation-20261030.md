# Locked npm runtime subprocess cancellation

Status: researched implementation plan, 2026-10-30. Scope is the existing locked
npm preparation path in `agentplugins/providers`. No release or merge is requested.

## Evidence and problem

`runLockedNPM` currently invokes `exec.CommandContext` directly, with stdout sent
to `io.Discard` and stderr copied into a 1,200-byte tail. Go's documented default
cancellation calls `Process.Kill` on that single process and leaves `WaitDelay`
unset. A helper retaining the stderr descriptor can therefore keep `Run` blocked
after the npm leader is killed, and can continue mutating the temporary runtime or
npm cache after the caller cancels. The current interruption test substitutes a
Go function for npm and cannot exercise this process or descriptor lifetime.

Primary reference: https://pkg.go.dev/os/exec#CommandContext and the `Cmd.Cancel`
and `Cmd.WaitDelay` documentation at https://pkg.go.dev/os/exec#Cmd.

The repository already provides `adapters/process.OS.Run`, which supervises the
leader and observable descendants, bounds inherited pipe/reap waits, retains
bounded stderr, and preserves typed cancellation and cleanup errors. Reuse this
implementation rather than adding npm-specific process groups, job objects, a
second runner, or a new timeout policy.

## Bounded change

1. Preserve the current ten-minute preparation context, npm executable selection,
   npm arguments, sanitized environment, cache validation, working directory, and
   no-stdin behavior.
2. Add an opt-in `DiscardStdout` field to the existing `ports.Command` value and
   honor it in the ordinary process runner by draining stdout to `io.Discard`.
   The zero value preserves all current callers. This retains npm's existing
   discarded-output behavior without unbounded capture or a new output limit
   that could terminate otherwise successful npm installs.
3. Execute npm through `process.OS.Run`. Treat nonzero `CommandResult.ExitCode`
   as failure; the shared runner intentionally returns natural nonzero statuses
   in the result. Preserve the final 1,200 bytes of bounded stderr in npm errors.
   Wrap the runner's actual error so `errors.Is` still identifies cancellation,
   deadline, and cleanup failures. Distinguish cancellation from timeout in the
   human diagnostic rather than labeling all cancellation as timeout.
4. Recheck the preparation context after the npm runner returns successfully and
   before validating/publishing the runtime, preventing a successful injected
   runner from committing a runtime after it has observed cancellation.
5. Replace implementation-only stderr-tail coverage with meaningful command
   outcome coverage and a real subprocess regression through `PrepareRuntime`.
   The fake npm starts a persistent helper that inherits stderr and writes a
   delayed marker. Synchronize on readiness, cancel, and assert bounded return,
   `errors.Is(context.Canceled)`, no committed target, no staging leftovers,
   stopped subprocesses, no delayed mutation, and a successful retry. Cover a
   deadline separately without waiting for the production timeout. Use existing
   platform process test conventions; no npm registry or real client is needed.

## Boundaries

Preserve VS Code's frozen implementation and dedicated tests. Preserve legacy
capabilities and the standard-first dependency graph. Do not change environment
CLI routing, adapter-boundary tests, release workflows, CI matrices, lock
protocols, client behavior, or overlapping source/project/SDK E2E work. The
process runner remains a trusted-command supervisor, not a hostile-process
sandbox. Platform containment is inherited from the existing implementation.

## Verification

Run focused provider tests, including the real subprocess cancellation/deadline
regression; focused process tests for discarded stdout and existing output
capture; the affected package tests with the race detector; then the repository's
required checks applicable to these packages. Inspect the patch and run
`git diff --check`. Record exact commands, outcomes, and environment limits below.
The root coordinator publishes this plan and opens the draft PR before code,
then publishes each coordinated commit immediately. No worker pushes or merges.

## Execution evidence

Pending implementation after plan publication.
