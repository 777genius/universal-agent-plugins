# Cline Windows session cancellation

Exact base: #878 head 9e92815ddce3bf75ee21ac7faa5526a93fad9905.

Confirmed defect: composition checks context only before StartSession; Windows
waits without observing cancellation while main consumes Interrupt/SIGTERM.
A native child can therefore outlive cancellation indefinitely.

Preserve StartSession and existing managed helper callers. Add StartSessionContext
for the public session composition; pre-cancel refuses before spawn. Unix keeps
exec replacement after its preflight context check. Windows keeps suspended
creation, Job Object assignment and inherited stdio/environment. Its single Wait
runs once; cancellation terminates the owned job and joins Wait before return.
Return cancellation plus any termination/reaping errors without hiding failures.
No trust, approval, configuration, source receipt or native qualification changes.

Critique round 1 (scope/API): changing the shared helper globally could affect
installed helper semantics. Preserve legacy entrypoints using background context;
only explicit native session composition opts into context cancellation.

Critique round 2 (lifetime/races): killing only the immediate child leaves its
MCP descendants. Terminate the Job Object and reap exactly once. Completion wins
when already observed; cancellation before launch must produce no child effect.
Use a real Windows test child with a descendant and readiness receipts; cancel
only after all three generations are ready, verify all three process handles signal after return.
The existing portable-session-contracts workflow selects TestSession names.

Estimated change: 70–130 production/test lines plus focused test helper; 1–2 hours
including Linux focused checks, Windows cross-compile and review. Genuine Windows
runtime test executes in existing CI; a cross-compile is not execution evidence.

## Completed local review and validation

Root and independent reviewer accepted the production cancellation/Job ownership
logic. Review corrections: pre-cancel uses only a harmless named test child and
requires no spawn receipt; the real Windows injected termination-failure subcase
requires preservation of ERROR_ACCESS_DENIED as well as context cancellation;
external imports use repository grouping. The Windows fixture pins all three
real process generations alive before cancellation and requires signaled process
handles afterward. Failed TerminateJobObject falls back to last-handle close,
which enforces KILL_ON_JOB_CLOSE; successful close clears the exclusively owned
handle so deferred cleanup cannot close it twice. No speculative watchdog added.

Actual local results with Go 1.26.11 / golangci-lint 2.13.2:
- Linux TestSession contracts PASS (0.057 seconds).
- Composed CLI TestClineProjectSession PASS (5.131 seconds final repeat).
- Windows amd64 complete managedstdio test cross-compile PASS. This is not execution.
- Full make lint PASS: four whole-file size/architecture gates and four changed
  module gates report zero issues; unchanged 138-rule baseline guard passes.
- git diff --check PASS.

Actual Windows execution of both cancellation cases remains pending in the
existing portable-session-contracts CI lane. No native tools/call, release or
all-six-client qualification follows from these process contracts.

Final Windows-tagged changed-line lint PASS (zero issues), including final
resume helper PID bounds and checked thread-handle close. Final Windows test
cross-compile PASS. No lint suppression was added.

Integration baseline: root verified newer #878 df1eb27b6e70605acdc1edc06499995c0ce2c671
changes only clients/cline/session.go and session_test.go, with no overlap with
this patch. Copy only the eight owned paths onto that exact baseline; root owns
coherent final review/checks and expected-head publication. Remote scope unchanged.

Root repeated full make lint and Windows-tagged changed-line lint on the staged
df1eb integration tree: both PASS. The focused managedstdio, Cline client, CLI
application and composed CLI session suites all PASS. Independent final review
accepted the actual resumeSessionProcess extraction and its failure cleanup.
The required make test-core was attempted with its complete log retained: it
already reports unchanged Git/process-containment, AF_UNIX and ambient Codex
environment failures; it is not a PASS or a waived merge gate. Hosted checks
remain necessary on the final published commit.
