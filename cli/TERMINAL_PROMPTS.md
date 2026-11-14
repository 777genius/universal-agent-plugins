# Installer terminal prompts

`agentplugins add <source>` in a human terminal selects compatible detected
clients, completes preflight, prints the complete plan, and asks to apply it.
The install answer defaults to **Yes**, including when only one client is found.
Fresh Enter accepts that shown default; queued input never approves it.
`--dry-run` stops at the plan. An explicit `--target` preserves command consent;
JSON and redirected stdin require explicit targets and never read prompt input.
Security overrides and activation/authentication attestations remain separate.

Use `--plain` for screen readers or line-oriented interaction without cursor
controls. Color is independent: add `--color=never` for output without escapes. Plain selection uses comma-separated numbers; Enter selects the shown
defaults. Confirm with a complete `y` or `yes` line. EOF, including an unfinished
`y`, is an error, not consent. Answers are bounded to 4096 bytes.

On suitable Unix terminals, Huh shows checkboxes: arrows move, Space toggles,
Enter submits. For confirmation, arrows or Space choose Yes/No, then Enter
submits. Printable `y`/`n` shortcuts do not submit; Tab does not submit. Esc,
Ctrl+C and Ctrl+D cancel. Bracketed paste does not supply checkbox/consent keys.
Queued complete answers remain available at the next question, where a new
consent snapshot makes them decline or cancel; they cannot approve the next
question. Security and activation/authentication questions share that boundary. No alternate
screen, mouse interaction, focus reporting or separate TTY is opened.

Rich mode requires actual terminal files on stdin, stdout and stderr, a known
non-dumb TERM, and an initial output size of at least 40 columns by 10 rows.
Otherwise Plain uses the visible output stream. If both outputs are redirected,
a required question fails without reading.

`--color=auto|never|always` defaults to `auto`. Auto checks each output stream
separately: redirected files/pipes and `TERM=dumb` receive no color. A nonempty
`NO_COLOR` (including `0` or whitespace) disables color unless an explicit color
flag is present. Explicit `--color=auto` still checks the destination and TERM;
`--color=always` forces color for human output, including redirected output.
`--no-color` aliases `--color=never`. Repeated identical choices are accepted;
contradictory explicit choices are rejected in either order, before command
execution. `--no-color=false` is rejected; use `--color=auto` instead.

Success uses green, warnings yellow, errors red, labels cyan, and secondary text
muted gray. Colons and values remain ordinary. Colors supplement the wording;
they are never required to understand status or consent. Selected human add,
plan, result, installation status and binding rows use these roles. Help remains
unstyled. JSON output, help and errors in JSON mode contain no ANSI, even with
`--color=always`. Rich prompts with `--color=never` retain cursor controls;
`--plain --color=never` disables both color and cursor controls.

These are implementation semantics, not native terminal release qualification.
The older terminal evidence harness assumes that Plain always has no escapes;
its Plain/tiny/redirect checks need an explicit `--color=never` when qualifying
cursor-free output. Colored Plain needs separate semantic assertions.

Windows currently selects Plain. Native console/ConPTY qualification is pending;
cross-compilation is not terminal evidence. Its line-reader cancellation uses a
validated inherited console input and a dedicated thread reading an owned
CONIN$ object with matching mode. Cancellation uses CancelIoEx on that object,
joins the reader, then closes only the owned handle. Redirected streams use
CancelSynchronousIo on the dedicated thread. No pipe-to-console fallback or
input-buffer flush is performed. Unix Plain cancellation uses a per-question wakeup
reader; the caller's input descriptor stays open. Custom injected blocking
readers must supply their own cancellation; production uses terminal files.

Errors and cancellation exit through the existing installer error path (exit 1).
Explicit No exits 0. After materialization, declining activation/authentication
leaves the installation resumable; it does not claim that nothing changed.
A failed rich form is never automatically restarted in Plain.

The linked dependency additions/upgrades have their license texts in
`THIRD_PARTY_NOTICES.txt`. Include these notices with binary redistribution.

The additive public `installerui.NewTerminal` takes borrowed prompt input and
visible output files; data stdout is independent. Auto mode can use rich UI
with redirected data stdout when the prompt pair is suitable. UAP keeps its
stricter three-stream rich policy. No terminal is opened by the library.

Use `Terminal.SelectMany` for checkbox selection and `Terminal.Confirm` for
consent. `Terminal.PlainUI` supplies cancellable legacy action/unit menus;
its `UI.Confirm` retains the old line API and has no consent snapshot. Calls on
one Terminal, including PlainUI, must be sequential. Provide signal-aware
contexts; the caller may close its descriptors after all calls return.

The new facade validates requests before I/O: 1–64 options, unique opaque ASCII
IDs and defaults, bounded valid UTF-8 display text, and at most 128 full summary
rows / 64 KiB. Hosts must visibly escape authority paths before passing them
as summary rows; labels may be abbreviated, summary rows are never clipped.
Rich summaries scroll with Up/Down, PgUp/PgDn, Home/End. Plain invalid selection
retries at most three complete answers. `all` / `none` are standalone selection
commands; an empty accepted subset is possible when MinSelected is zero.
Cancellation, incomplete EOF, output/resize/restore errors never accept partial
results. A queued consent snapshot above 4096 bytes/events fails closed.
