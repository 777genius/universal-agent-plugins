# Claude project native failure — bounded diagnostic capture

Pre-code supplement, 2026-10-23. The original twice-reviewed Claude project
adapter/native gate remains in force. This changes only missing failure
observability in the opt-in source native fixture, not its success criteria
or the production adapter/parser. Obtain two independent plan GO reviews before
source changes, then two independent implementation reviews before root replay.

## Actual failure and missing fact

Genuine disposable hosted macOS arm64 run40011624952, job120613172265, artifact
12528049546 executed Claude Code2.1.263 with measured binary SHA256
`ef5d2909c8af49f31ab6d5487e90316777bc2fac170adfe8160716caa8aaf4f9`.
Artifact ZIP SHA256 is
`c45098c244379dd4e0b62f2dfb7e235b415959428e19cd235cf083652fb5caf7`.
User command/plugin/runtime suites passed; the project test actually failed
at add with `the Claude Code plugin list output is not recognized`. Its project
prefix `uap-claude-native-evidence-58929984/` contains installer failure/version
but no raw native project listing, because add aborts before the later probe.
No project list shape or correct decoder change can be inferred from this
error alone. Keep the current fail-closed parser and actual failed stage.

## Narrow test-only correction

In the existing Claude project native fixture, when its installer runner first
returns an error, collect at most two fresh read-only native commands before
returning the same original failure: `plugin list --json`, once from its exact
project A cwd and once from its already-existing project B cwd. Use the same
verified genuine client path and explicit fixture environment/original
CLAUDE_CONFIG_DIR as ordinary native probes. No installer retry, native mutation,
reload, plugin details with a guessed ID, trust/account/model/MCP invocation,
new acquisition or hosted guard change is allowed. Successful installer paths
run no additional diagnostic commands. Both existing opt-in and unchanged real
disposable-host guard remain before fixture/acquisition/process creation.

Capture stdout and stderr separately with a fixed1MiB cap per stream while the
process runs, an independent10s context deadline per command, DEVNULL/empty
stdin, no shell and a short exec.WaitDelay (at most250ms) for inherited pipe
closure. Each capped writer cancels the child promptly on overflow and records
that overflow; do not use CombinedOutput or truncate unbounded output afterward.
Use fixed labels/argv and only known fixture roots. Record exact argv/cwd,
exit/error/deadline/limit/elapsed and byte/hash metadata with bounded error text;
preserve separate raw capped stdout/stderr logs under the existing evidence
root even on nonzero exit, timeout, malformed/unknown JSON or overflow. Do not
parse this diagnostic into installed/active evidence or suppress its error.
The existing context/WaitDelay primitive bounds this read-only diagnostic;
it does not claim OS isolation or general descendant/process sandboxing.

Run the existing foreign-byte check after diagnostics and preserve the original
installer failure/stage as failed. Additional diagnostic failure is separately
recorded, never a reason to mark the original lifecycle successful. Existing
writeEvidence hashes the captured logs/metadata in the ordinary uploaded
artifact. No secrets/account state exist in this fresh explicit fixture.

## Meaningful inert verification and next gate

Add focused tests for the capped stream primitive's exact boundary, overflow
cancellation and preservation without unbounded accumulation, separate streams,
bounded metadata/errors and finite diagnostic argv/cwd selection. Tests may
inject inert capture functions or use an inert test process; they may not launch
Claude, spoof hosted admission or weaken native success assertions. Retain
existing native opt-in/guard safety tests, syntax/format/diff checks and bounded
relevant repotests. No broad test rebuild is required while task storage is
constrained.

Commit the source checkpoint with both independent implementation GO reports;
root publishes/replays the exact hosted native suite. Independently inspect
that genuine artifact before proposing any production parser/scope change.
If a decoder correction is warranted, freeze a separate narrow evidence-based
plan and obtain both plan GO reviews before coding it. Neither successful
diagnostic collection nor existing user-suite success qualifies the failing
project lane, trust, MCP/runtime, another OS, all clients or release completion.
