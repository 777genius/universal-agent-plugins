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
protocols, client behavior, or overlapping source/project/SDK E2E work. The only
workflow exception is the diagnosed Skills source resolution runner correction
recorded below; it changes the existing job's runner selection without changing
its tests, permissions, timeout, or concurrency. The
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
retries successfully through the same real command path. Each fixture opens a
pidfd for itself and transfers that already-bound handle to the test through
`SCM_RIGHTS`. The parent never reacquires a handle from a numeric PID or PID
file. One overall five-second readiness deadline covers both handoffs; the
actual deadline subtest has a separate fifteen-second context timeout. Missing
readiness fails rather than counting an early timeout as cancellation proof.
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


### Independent harness review correction

The fresh independent review identified a test-only identity race in the first
published harness: a three-second context timeout could allow npm to be reaped
before the parent opened a pidfd from its atomic PID file. A subsequently reused
PID could therefore name an unrelated task. Atomic file publication did not
bind the acquired descriptor to the original fixture. The timeout also preceded
the original five-second readiness allowance. Production code was unchanged.

The correction removes numeric PID publication and reacquisition entirely.
Fixtures open their own pidfds and pass them through Linux Unix datagram
`SCM_RIGHTS`; the receiver validates the role, exactly one descriptor, and
untruncated control data before registering signal cleanup. Received descriptors
are marked close-on-exec and closed on validation failure. Cleanup signals the
transferred stable handle before closing it. A random Linux abstract socket
address avoids Unix socket pathname-length failures in long test directories.
The sender uses `unix.Sendmsg` directly because Go 1.26.10 rejects
`WriteMsgUnix` on connected Unix datagram sockets, even with a nil destination.

A single five-second socket read deadline bounds both readiness handoffs, while
the real deadline context allows fifteen seconds from command launch. Completion
and cancellation may be observed simultaneously: the test rechecks `ctx.Err()`
and retains an already-consumed result rather than reporting a false early exit
or waiting for a second result.

- PASS, actual local execution with the race detector:
  `go test -count=1 -race -v -run '^TestLockedNPMFixtureTransferredPIDFDStopsItsOriginatingProcess$'
  ./install/integrationctl/agentplugins/providers` (1.099s, outside the restricted
  sandbox). This test starts a direct fixture process, receives its self-opened
  pidfd, signals that transferred handle, and confirms the originating command
  was stopped and reaped. It needs no proc-children supervisor.
- PASS, independently repeated by the fresh reviewer:
  `go test -count=1 -race -run 'TestLockedNPMFixtureTransferredPIDFDStopsItsOriginatingProcess|TestLockedNPMCommandOutcome|TestLockedRuntimeCanceledSuccessfulRunnerCannotCommitTarget'
  ./install/integrationctl/agentplugins/providers` (1.182s, outside the restricted
  sandbox).
- PASS: focused locked-runtime/outcome race tests (1.871s), changed-line provider
  lint, and the full-file provider size/architecture gates (zero issues).
- Full supervised cancellation/deadline proof continues to require observed
  non-skipped GitHub-hosted execution because this local host lacks the existing
  proc-children prerequisite. The independent reviewer rechecked the final
  handoff, cleanup ownership, and select behavior and reported no remaining code
  findings.


### Diagnosed Skills source resolution runner correction

The final-commit CI run exposed an existing runner capability mismatch in
[Skills source resolution run 41087510047, job 124172431116](https://github.com/777genius/universal-agent-plugins/actions/runs/41087510047/job/124172431116)
on npm commit `74ade1d3`. The source adapter tests and Docker runtime-envelope
probe passed. The subsequent real systemd deployment probe failed before running
its test: `/run/systemd/system` was absent, systemd was not PID 1, and `systemctl`
could not contact the system bus (`Host is down`).

The existing `source-and-runtime` job selected the generic `CI_LINUX_RUNNER`
repository variable for same-repository PRs and main pushes. In this run that
variable selected a Namespace container, whose container environment does not
provide the systemd VM required by the deployment probe. A PR-label fallback
would leave main pushes exposed to the same diagnosed failure.

The bounded correction selects the explicit GitHub-hosted `ubuntu-24.04` VM for
this existing job on every trigger, with a short systemd capability comment.
All existing source tests, Docker checks, deployment/systemd probe commands,
permissions, timeout, and concurrency remain unchanged. No workflow, flag,
capability framework, runtime-code change, or weaker probe was added. This is
the sole exception to the original no-workflow scope, justified by the observed
CI failure and the coordinator's explicit instruction to repair it.

Validation: PyYAML 6.0.3 parsed the workflow successfully. A complete parsed
mapping comparison against `HEAD` confirmed that only
`jobs.source-and-runtime.runs-on` changed; triggers, steps, permissions, timeout,
and concurrency are identical. `git diff --check` passed. `actionlint` is not
installed in PATH or the available cached tooling directory, so it was not run.
Runtime-code tests were not repeated because this correction changes only
runner selection. The unchanged real systemd probe must pass on the selected
GitHub-hosted VM before CI is claimed green.
