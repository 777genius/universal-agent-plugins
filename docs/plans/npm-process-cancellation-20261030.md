# Locked npm runtime subprocess cancellation

Status: implemented; native Linux CI proof pending, 2026-10-30. Scope is the existing locked
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
   A small typed npm error retains the full runner cause for `errors.Is` and
   `errors.As`, while rendering at most 300 bytes of cause summary plus a
   truncation label and the 1,200-byte stderr tail (under 1,600 bytes total).
   This also bounds cancellation/cleanup errors that already embed the shared
   runner's larger diagnostic buffer. Distinguish cancellation from timeout in the
   human diagnostic rather than labeling all cancellation as timeout.
4. Recheck the preparation context immediately after the npm runner returns
   successfully and again directly before committing the target, preventing a
   successful injected runner from committing after observed cancellation. This
   does not claim atomic cancellation of an already-started filesystem commit.
5. Replace implementation-only stderr-tail coverage with meaningful command
   outcome coverage and a real subprocess regression through `PrepareRuntime`.
   The fake npm starts a persistent helper that inherits stderr and writes a
   delayed marker. Synchronize on readiness, cancel, and assert bounded return,
   `errors.Is(context.Canceled)`, no committed target, no staging leftovers,
   stopped subprocesses, no delayed mutation, and a successful retry. Cover a
   deadline separately without waiting for the production timeout. Use existing
   Linux process test conventions and pidfds for stable fixture identities;
   existing runner tests retain Darwin/Windows lifecycle coverage. No npm
   registry or real client is needed.

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

The plan was published before production edits as draft PR #589, plan commit
`bb8881242db103af584382125aea95a91bd79abd`. The implementation uses the existing
ordinary process runner, retains npm's ten-minute preparation timeout and
argument/environment contract, and adds only an opt-in stdout discard field.
No subprocess framework, release policy, workflow, or CI matrix was added.

Validation used Go 1.26.10, with cached dependencies in `/tmp/uap-audit-gomodcache`.

- PASS: `go test -count=1 -race -run 'TestLockedRuntime|TestLockedNPMCommand'
  ./install/integrationctl/agentplugins/providers` (final rerun: 1.448s). This exercises the
  existing locked-runtime lifecycle tests, canceled-successful runner refusal,
  exact cancellation/deadline cause preservation including joined cleanup
  failure, natural nonzero status, and bounded diagnostic-tail behavior.
- PASS: changed-line golangci-lint against base `ae068426706d5f2c9d624ae94c6504ac8b7f553e`
  for `providers`, `adapters/process`, and `ports`: zero issues. Full-file size
  and architecture gates (`revive,funlen,gocyclo,gocognit,dupl,depguard`) also
  report zero issues for those packages.
- PASS: `go test -c` for both providers and the process adapter on
  `GOOS=darwin GOARCH=arm64` and `GOOS=windows GOARCH=amd64`. These are compile
  checks, not runtime execution on those systems.
- PASS: `git diff --check`.
- BLOCKED LOCALLY: real subprocess execution. The pre-existing
  `TestOSRunnerPreservesCommandOutputAndExitCode` fails inside the restricted
  sandbox with `current locked OS thread disappeared during ordinary
  containment preflight`. Running the same baseline outside that sandbox
  reaches the existing explicit capability skip: a positively live proc task
  lacks its `children` interface. The new native cancellation/deadline cases,
  real-command outcome test, and portable process discard tests likewise
  explicitly SKIP outside the sandbox for this missing prerequisite. On
  `RUNNER_ENVIRONMENT=github-hosted`, only the new Linux npm regressions turn
  unavailable containment or pidfd prerequisites into a failure, so green
  GitHub-hosted Required execution cannot hide a skip of this regression. These
  skips are not cancellation proof; observed non-skipped Linux CI execution
  remains required. No production guard was weakened.
- PRE-EXISTING BROADER FAILURE: before implementation, running the full providers
  package failed in existing Codex identity/activation/golden tests with
  `malformed or duplicate Codex environment entry` (5.178s). Focused npm tests
  pass; this patch does not change Codex environment parsing.

The Linux subprocess regression executes the production `PrepareRuntime` path
with a real fake-npm executable and a persistent helper inheriting stderr. It
requires readiness before cancellation, checks typed cancellation/deadline and
bounded return, observes both identities through pidfds, checks no target or
staging residue, triggers a delayed cache mutation that must not occur, then
retries successfully through the same real command path. Fixture PID readiness
is published atomically and cleanup uses pidfds rather than recyclable PIDs.
Portable process tests additionally verify discarded large stdout retains
nonzero status, stderr diagnostics, typed cancellation, and cleanup attestation.


Windows compatibility was inspected against cached Go 1.26.10 source:
`CommandContext` delegates to `Command(name, args...)` and adds only its context
and cancellation callback. The shared runner uses the same command constructor
and argv; Windows containment adds suspended creation, Job attachment, and
resume without rewriting command lines or adding a shell. Both old and new npm
paths select `npm.cmd` without an explicit `cmd.exe /c` wrapper. Any native batch
launch limitation of that pre-existing choice is outside this cancellation
correction; Windows compilation is not presented as runtime proof.
