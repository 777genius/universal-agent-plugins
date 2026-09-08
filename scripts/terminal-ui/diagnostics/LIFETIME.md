# Temporary bounded Windows lifetime matrix

Source base: `268e6d105229d250625628d59a0c694d657bbe1f`.
This is diagnostic evidence, not a production repair or release qualification.
The workflow applies the same `lifetime.patch` to a fresh exact-base checkout
for every row, then links the selected policy into the test executable. The
driver uses a restricted environment, so policy selection is not an environment
variable. `request.json`, the build command, binary hash and preflight output
identify the row. No PowerShell owner/harness implementation changes are made.
The stored fixture has zero context to avoid nested-patch whitespace warnings;
the workflow checks the exact SHA before applying it with `--unidiff-zero`.
All rows also carry the separately reviewed capture correction: process exit
does not end marker waiting while the output reader still drains, and the
original deadline stays fixed. A stopped reader is rescanned before failure.
This fixes a deterministic race in the supplied yes-lifecycle evidence; it does
not accept missing/late markers or qualify that historical run. The shared
harness source in the worker checkout remains untouched.

| Policy | Console file object | Canceled issuing thread | Question |
| --- | --- | --- | --- |
| baseline | Duplicate of inherited input | Return to Go pool | Does the failure reproduce with shared instrumentation? |
| retired-noninitial | Duplicate of inherited input | Exclude startup thread, retire, verify native exit | Does actual issuing-thread termination clear the stale host request? |
| fresh-input | New `CONIN$` open after inherited console validation | Return to Go pool | Does final close of independently opened input state clear the stale request? |
| fresh-input-retired | New `CONIN$` open | Exclude startup thread, retire, verify native exit | Are both lifetimes required? |

The original inherited-handle variant failed the same started-cancel/reuse case.
That rules out duplicate-close *alone* as the cause; it does not test a new file
object. The older retired-thread fixture is retained as historical evidence. Its
first canceled request did not enter ReadConsole and its infinite wait never
completed. Go 1.25.13 `runtime.mexit` parks the startup OS thread (`m0`) forever
when a goroutine exits locked to it. The old trace did not record thread IDs, so
it cannot establish that this exception caused that particular hang, or rule out
a wait-access issue. It also cannot refute the thread-lifetime hypothesis.

Before any console read, the new preflight duplicates a live noninitial thread
with the same access flags as the reader, checks `GetThreadId`, expects
`WAIT_TIMEOUT` from a zero-duration live wait, then releases the goroutine while
locked and expects `WAIT_OBJECT_0` within 250 ms. It also exercises a context
canceled before `ReadLine`. Preflight runs independently without a console and
again inside each disposable ConPTY, emitting `DIAGNOSTIC_PREFLIGHT_OK`.

The startup thread ID is captured during Go package initialization. A reader
that lands there holds it locked while a second locked thread accepts the work.
Retirement applies only when a native console call was entered and the context
was canceled; a pre-read cancellation does not retire an idle thread. Thread
handles remain owned until the reader joins and the finite exit wait completes.
Trace records contain current thread IDs, native entry/return, native-entered
and retired flags, wait entry/result, and separate console-close entry/return.
Optional object comparisons are informative only if the duplicate baseline
provides a positive same-object control on that Windows implementation.

Every original control still performs 30 exchanges, with the original six-delay
sweep, one-second cancellation and two-second reuse limits. Watchdogs turn a
stuck operation into an explicit failing process with stacks instead of waiting
for the outer harness deadline. The unchanged qualification resource census is
also called after six warmup exchanges and each following six-exchange batch,
with GC finalizers disabled. No bound is relaxed and no stuck read is retried.

The additional `queued-boundaries` control submits three legitimate answers
together: Unicode including a combining mark and a surrogate pair, Ctrl+Z/CR,
then the next line. All three must survive separate read/close/open lifetimes.
It repeats ten groups (30 reads). Subsequent readiness markers receive no extra
input; this cannot rescue a blocked read. All controls still require exact
console-owner snapshots, final owner readback/echo and no forced cleanup.

Interpretation:

- A failing preflight is a probe failure. Do not draw a cancellation conclusion.
- Baseline must reproduce a cancellation that actually entered ReadConsole.
  If it passes, inspect scheduling and tracing effects before selecting a repair.
- `fresh-input` alone passing favors a small owned-open/final-close repair.
- `retired-noninitial` alone passing favors a thread-lifetime repair; simply
  returning locked is insufficient because of the startup-thread exception.
- Only the combined policy passing indicates a lifetime interaction.
- If both independent policies pass, prefer the smaller supported repair after
  qualification. If all fail, obtain driver-to-host cancellation dispatch
  evidence rather than switching APIs without a discriminating explanation.

Never promote this diagnostic overlay. Strip native tracing and all temporary
probes/workflow from a selected production candidate. Then run the unmodified
30-cancel resource qualification, native EOF/Unicode/queued-owner tests and the
full native CLI matrix including `yes-lifecycle`, with its original restoration
deadline. A late RESTORE_OK, diagnostic success, installer release evidence or
PowerShell-only success does not substitute for those checks.
