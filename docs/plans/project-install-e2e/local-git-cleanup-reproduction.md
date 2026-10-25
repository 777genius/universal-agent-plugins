# One bounded local-Git cleanup observation

On exact candidate `e81e9daf2fbce3acf36aee33eb954a68fcf18acc`, terminal Linux
run `40194870304`, job `121173439610`, fails the dependency reconciliation
`missing_edge` fixture. Its existing fixed diagnostic positively reports
`phase=fetch class=ordinary_supervisor_descendant_cleanup exit_status=unavailable`.
Core's separate normalization fixture fails before normalization without a
more specific label. Full Coverage passes. These observations do not identify
the exact cleanup cause or establish that every acquisition failure is shared.

The terminal resolver takes `acquireDependencyGit`'s `DirectAcquirer` branch:
`dependencyTestSource.AcquireGit` delegates to `Acquirer.AcquireGit`, which passes
`native=true` into `acquireGitTransport`. GitHub shorthand and the local
`URLForRepo` mapping do not change that branch. Its observed error is explicitly
`acquire direct Git source`. The separate Core `AcquireGitHub` path adds
`--filter=blob:none`; this experiment targets the positively classified terminal
fetch without that flag and does not pretend to reproduce both call paths.

The finite origin-category amendment is separate. This test-only slice observes
the original process error at an entirely controlled local-Git boundary, before
the public acquisition layer intentionally sanitizes it. A passing attempt is
not a fix or an explanation for the historical failure.

## Fixed experiment

- Add one Linux-only process-package test, explicitly opt-in through
  `AGENTPLUGINS_GIT_CLEANUP_DIAGNOSTIC=1`. Normal suites report this diagnostic
  as skipped, not as runtime qualification. In the opted-in test, the real
  `OS.OrdinaryPreflight` must succeed; unavailable containment fails the test.
  Do not substitute a runner, proc source, process group or native host claim.
- Create a private temporary HOME, source repository and work directories.
  The child environment is an explicit allowlist: the host PATH for locating
  Git, private HOME/XDG/TMP paths, disabled global/system Git configuration,
  noninteractive/no-credential settings and constant inert author identity.
  No original user profile, ambient Git/SSH options, network endpoint, token,
  SSH agent, package executable or vendor client enters the experiment.
- Run every Git command, including setup, with the ordinary production
  `OS.RunWithTreeExitGrace` path and its existing five-second natural-exit grace.
  Disable hooks, fsmonitor, automatic maintenance, GC and recursive submodules.
  Permit only the local file protocol. Setup creates one small, fixed local
  repository and captures its immutable commit; no source text enters a shell.
- Execute exactly 512 serial iterations, each initializing a fresh destination
  with the existing acquisition remote/settings and fetching that same immutable
  local commit. Match the positively observed native fetch graph: `fetch --quiet
  --depth=1 --no-tags --no-recurse-submodules origin <immutable commit>` after
  `-C <private destination>`; verify `FETCH_HEAD^{commit}` afterward. Each
  iteration must succeed naturally and preserve the expected revision. Stop at
  the first failure;
  this is a fixed process-race experiment, not retrying a failed command.
  Bound each command context to 15 seconds, the complete child context to five
  minutes and the test to seven. Context cancellation still uses unchanged
  production cleanup; every process failure is a failed observation.
- On failure emit fixed phase/iteration/class/status metadata. Only for the
  positively identified ordinary descendant-cleanup class from this controlled
  OS invocation, emit at most 4 KiB of its original process error. Its command,
  repository, files and complete child environment are fixture-owned. Do not
  print stdout, stderr, arguments, environment or general acquisition errors.
  All existing production and generic fixture loggers retain their prohibition
  on reading or formatting arbitrary error text. Success emits a single fixed
  completion count; no per-iteration output or additional artifacts.
  Admission uses the existing closed classifier's generic cleanup label or its
  recognized fixed reason suffixes; no arbitrary error text establishes a class.

## Publication and observation

Change only the new test, this plan and the existing temporary
`project-git-diagnostic.yml` workflow. Retain the same Ubuntu runner, Go pin,
read-only permissions and 15-minute job budget. Replace the already completed
prior diagnostic selections with one exact new test invocation, `-count=1`,
explicit opt-in and a seven-minute test deadline. Do not add retries, a matrix,
concurrency or a repeated full-suite invocation.

Publish reviewed plan/code first on the separate reproduction branch, which
does not trigger the focused workflow. After frozen independent code reviews,
advance the existing focused branch once to the reviewed combined source and
record its exact SHA, tree, runner, test count and result. Inspect the actual
named test result; a missing, skipped, canceled or timed-out observation is not
qualified. No local runtime execution is attempted on the already unsupported
executor. Inert compile and configured affected-package/workflow checks are
sufficient before the hosted observation.

Both independent plan rounds precede implementation. Both independent frozen
implementation reviews precede the one hosted observation. Any behavior change
based on its result requires its own evidence and reviewed plan. Existing full
Required/Core/Coverage/native gates remain mandatory for the final candidate.

## Runner-provider parity amendment

The first observation completed successfully on source
`14c35e9d35fcd976b5d74bcf4c02ff3986073b5e`, tree
`cfec35f7921e4547e2e6724ddbc74080a53f5619`: run `40198070979`, job
`121183139474`, all 512 iterations without a skip in 115.044 seconds. Its
actual host was Ubicloud Ubuntu 24.04, image `20261021.1.0`, selected by
`vars.CI_LINUX_RUNNER || 'ubuntu-latest'`. That result qualifies only this
bounded observation; it neither identifies nor corrects the historical cause.

The positively classified terminal failure, run `40194870304`, job
`121173439610`, instead used the explicit `ubuntu-24.04` selector on a
GitHub Actions runner image, version `20261012.334.1`. Core's separate,
unclassified normalization failure used Ubicloud and a different, filtered
`AcquireGitHub` fetch path. Do not combine these failures or infer a shared
cause from their host names.

Make one additional observation on the terminal failure's runner provider and
OS selector. This amendment supersedes only the previous instruction to retain
the workflow's runner expression: set `runs-on: ubuntu-24.04`. The test blob
`f579c6b68c5d625e9e87e4e51b288990ddfcd1a4`, exact invocation, child environment,
512-iteration count, first-error stop, containment, deadlines, permissions and
all other workflow settings remain unchanged. Do not add a matrix, retry,
fallback host, additional Ubicloud attempt or process behavior change.

The selector does not pin the historical GitHub image version. Record the
new observation's actual provider, image version, source SHA/tree and test
blob, plus the named test result and absence of skips. Preserve the first
Ubicloud result separately. A pass on either host, or a difference between the
two results, does not by itself establish the historical failure's cause.
A failure may supply a controlled original error for a separately reviewed
correction; a pass remains a noncausal observation. All final-candidate
Required/Core/Coverage/native gates remain mandatory.

Two independent plan-review rounds precede this one-line workflow change;
two independent frozen code reviews precede its one hosted invocation. Publish
the reviewed amendment and workflow on the reproduction branch, then advance
the focused branch once from `14c35e9d35fcd976b5d74bcf4c02ff3986073b5e` using an
expected-head lease. Verify YAML, the exact single-line workflow diff and the
unchanged test blob; repeat neither the already passed inert Go validation nor
any native execution on the unsupported local executor.
