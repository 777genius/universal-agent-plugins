# Installer terminal prompts

`agentplugins add <source>` in a human terminal selects compatible detected
clients, completes preflight, prints the complete plan, and asks to apply it.
The install answer defaults to **No**, including when only one client is found.
`--dry-run` stops at the plan. An explicit `--target` preserves command consent;
JSON and redirected stdin require explicit targets and never read prompt input.
Security overrides and activation/authentication attestations remain separate.

Use `--plain` for screen readers or line-oriented interaction without terminal
controls. Plain selection uses comma-separated numbers; Enter selects the shown
defaults. Confirm with a complete `y` or `yes` line. EOF, including an unfinished
`y`, is an error, not consent. Answers are bounded to 4096 bytes.

On suitable Unix terminals, Huh shows checkboxes: arrows move, Space toggles,
Enter submits. For confirmation, arrows or Space choose Yes/No, then Enter
submits. Printable `y`/`n` shortcuts do not submit; Tab does not submit. Esc,
Ctrl+C and Ctrl+D cancel. Bracketed paste does not supply checkbox/consent keys.
Queued complete answers remain available at the next question. No alternate
screen, mouse interaction, focus reporting or separate TTY is opened.

Rich mode requires actual terminal files on stdin, stdout and stderr, a known
non-dumb TERM, and an initial output size of at least 40 columns by 10 rows.
Otherwise Plain uses the visible output stream. If both outputs are redirected,
a required question fails without reading. `--no-color` and nonempty `NO_COLOR`
disable colors independently of rich controls. Use `--plain` for no escapes.

Windows currently selects Plain. Native console/ConPTY qualification is pending;
cross-compilation is not terminal evidence. Its line-reader cancellation uses a
dedicated thread and CancelSynchronousIo on inherited input, with no CONIN$
reopen or input-buffer flush. Unix Plain cancellation uses a per-question wakeup
reader; the caller's input descriptor stays open. Custom injected blocking
readers must supply their own cancellation; production uses terminal files.

Errors and cancellation exit through the existing installer error path (exit 1).
Explicit No exits 0. After materialization, declining activation/authentication
leaves the installation resumable; it does not claim that nothing changed.
A failed rich form is never automatically restarted in Plain.

The linked dependency additions/upgrades have their license texts in
`THIRD_PARTY_NOTICES.txt`. Include these notices with binary redistribution.
