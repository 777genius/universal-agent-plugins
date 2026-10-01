# Gemini USER hook planning

`geminihooks` is a neutral, pure planner for named singleton command groups in
Gemini CLI 0.62.0 USER settings. It installs only requested native events. It
reads no filesystem, environment, authentication or discovery state, writes no
files, and imports neither the CLI nor the YAML/legacy installer engine.

```go
hooks := []geminihooks.HookSpec{
    {Event: "AfterAgent", Name: "example.finish",
     Argv: []string{"/opt/example tools/helper", "finish"}, Timeout: 2500},
    {Event: "Notification", Name: "example.permission", Matcher: "ToolPermission",
     Argv: []string{"/opt/example tools/helper", "permission"}, Timeout: 2500},
}
result, err := geminihooks.Plan(geminihooks.Request{
    Settings: existingBytes, Shell: geminihooks.Bash,
    Operation: geminihooks.Install, Hooks: hooks, Previous: previousReceipt,
})
```

The host supplies bytes (zero length means absent settings), a qualified native
shell and install/update/remove. `Hooks` is the complete desired owned set on
install/update; remove uses the prior receipt. Empty desired sets require remove.
Timeout is milliseconds; zero omits it, negative conflicts. Matcher is omitted
when empty. Errors wrap `ErrConflict`; conflicts return original bytes, nil
receipt and `Conflict=true`. No-op returns original bytes exactly. Successful
remove returns nil receipt and retains the settings document/event arrays.

A version-1 receipt stores only event/name/full-group canonical SHA-256 digests.
The host binds it to its explicit config path in its existing ledger. Receipt
order and JSON member order do not establish ownership. Event moves, duplicate
names, an extra hook, or changes to matcher/sequential/name/type/command/timeout/
env/unknown group or hook members conflict before mutation. Identical commands
without a receipt cannot be adopted. Collision checks include other native and
future event arrays. `VerifyOwned(bytes, receipt)` verifies only owned groups;
foreign edits, including opaque sibling values, do not revoke them. Duplicate
object keys or malformed grammar remain document ambiguity conflicts.

Key and selector comparisons retain native UTF-16 code units, including escaped
lone surrogates. Owned groups containing strings that Go JSON decoding would
replace with U+FFFD conflict before digest verification or mutation; this covers
keys and unknown nested values too. Literal U+FFFD and valid paired Unicode
remain supported. Distinct lone-surrogate keys/names in opaque foreign data
remain distinct, and their original bytes are preserved.

`HookSpec.Observer` explicitly opts an `AfterAgent` or `Notification` hook into
neutral observation. Other events conflict before mutation. Its zero value
keeps the exact generic command rendering and behavior. Observer argv uses the
same literal validation and quoting as `RenderArgv`; the planner wraps that
invocation in the existing native shell. All observer output is discarded,
then the shell emits exactly `{}\n` and exits zero, including a removed/missing
executable, nonzero exit status, or a thrown PowerShell invocation. Bash uses a
guarded subshell; PowerShell consumes all streams and catches invocation errors,
then exits before Gemini's appended native exit-code check. No additional
executable, delivery policy or application-specific command participates.

This opt-in addresses Gemini 0.62.0's cached hook lifecycle: deleting a helper
while Gemini remains alive leaves its cached command callable. Naked Bash
invocation then emits a diagnostic and exit 127; `hookRunner` falls back from
JSON parsing to `convertPlainTextToHookOutput`, mapping nonzero codes other than
1 to `decision: deny`. An AfterAgent denial can cause a retry with
`stop_hook_active: true`. An observer must never supply such a decision, context
or model message. Empty JSON supplies none of those fields. Receipts continue
to digest the actual wrapped command bytes, so opt-in changes require a normal
owned update; repeat, drift checks and remove use the same version-1 contract.
The native hook timeout (explicit `Timeout`, otherwise Gemini's default) still
bounds invocation; this wrapper does not add a timer or neutralize native timeout
or shell-start failures.

The planner preserves top-level policy, unknown values, foreign groups and their
comments in the document AST. Gemini 0.62.0 uses separate top-level
`hooksConfig.enabled/disabled/notifications` (`config.ts:1111`,
`settingsSchema.ts:2548`); these and security/trust settings are untouched. Legacy
reserved fields inside `hooks` also remain opaque. Ownership is not proof of
activation: the host must check effective merged settings/trust/disabled names,
retain the receipt, lock and reread bytes, perform CAS writes, then verify
readback. No policy is silently enabled.

`RenderArgv(Shell, []string)` supports explicit `Bash` and `PowerShell`. Native
0.62.0 `hookRunner` uses `getShellConfiguration`: Unix bash `-c`; Windows a
PowerShell ComSpec only, otherwise pwsh on PATH or powershell fallback, never
cmd for hooks. Use a caller-qualified executable path. Bash uses `exec --` and single-quoted
literals; PowerShell uses `&` plus single-quoted literals with doubled
apostrophes. Spaces, Unicode, apostrophe, backtick, ampersand, bang and percent
remain literal. NUL/CR/LF/invalid UTF-8 conflict. PowerShell empty arguments and
embedded double quotes conflict because the Windows PowerShell 5.1 fallback
cannot preserve them reliably.

Before shell execution, `settings.ts:806` runs `resolveEnvVarsInObject`. Tokens
matching its variable syntax (`$VAR`, `${VAR}`, `${VAR:-default}`, including
arbitrary braced names) produce an explicit unsupported interpolation conflict
in argv, hook names and matchers. The final command is checked too. Shell
quoting alone cannot prevent settings expansion. There is no owner-environment
resolution, stdin interpolation, fallback renderer or automatic shell choice.

Parsing selectively adapts the existing private `nativeconfig/document.go`
primitives using the already pinned hujson AST, leaving the legacy MCP codec
untouched. A public document/MCP framework is unnecessary. The 4 MiB input bound
matches the existing conformance MCP document ceiling. Before recursive parsing
or AST traversal, a private allocation-free lexical preflight caps structural
depth at 64 (including the root) and nodes at 65,536 (containers, object keys and
scalar values). Strings and comments contribute no internal delimiters or
nodes. Exceeding any budget returns `ErrConflict`, original bytes and no receipt;
`VerifyOwned` applies the same limits. Output is checked against those limits
too. These are supported document budgets, not a new parser or an activation
claim. Comments are accepted;
trailing commas are rejected because native settings use
`JSON.parse(stripJsonComments(content))` with default options (`settings.ts:787`).
Tests cover comment-bearing fixtures and trailing-comma rejection; they do not
claim exhaustive grammar parity with strip-json-comments.

Run from the module with Go 1.26.8, `go test -p 2 ./geminihooks
./adapters/nativeconfig`. Tests require Node >=22.18 to execute the unchanged
official 0.62.0 environment resolver fixture. Its supplied source provenance is
Gemini v0.62.0 commit `b460678f3db508407554afd604cc9d6635becb2a`,
`packages/cli/src/utils/envVarResolver.ts`; original Google Apache-2.0 header is
retained. The argv probe executes only a test executable under the native shell.
Linux tests execute bash after settings expansion; the negative control shows
naive quoting is changed by actual expansion. Windows CI cases execute available
pwsh.exe/powershell.exe and log exact argv; non-Windows hosts explicitly skip
those cases. Linux results do not qualify Windows, macOS or actual Gemini
activation. No actual agent, credentials or real project is exercised.

`go test ./geminihooks/...` also runs the observer contract fixtures. They record
exact argv through a real executable, exercise cached-command deletion and exits
0/2/127, assert full stdout `{}\n`, empty stderr and exit zero, and retain naked
command negative controls. Available Bash, pwsh and Windows PowerShell execute
directly; observer cases skip only unavailable shell executables. PowerShell
cases include the real hookRunner exit-code suffix and a thrown script invocation.
Unavailable Windows PowerShell execution remains pending native Windows CI
qualification; Linux Bash evidence cannot qualify Windows.
