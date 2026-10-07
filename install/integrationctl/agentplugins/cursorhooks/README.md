# Cursor user-hook planning

`cursorhooks` is a pure public library for one fixed `stop` observer. It accepts
bytes, a trusted previous receipt, a fixed specification and an explicitly
qualified shell contract. It returns desired bytes and a receipt; it performs
no filesystem, environment, policy, consent or client operations.

This UC2 slice is source support. It does not enable Cursor in the installer,
SDK or Notifications, or qualify an installed notification integration.

```go
result, err := cursorhooks.Plan(cursorhooks.Request{
    Document: document,
    Operation: cursorhooks.Install,
    Specs: []cursorhooks.HookSpec{{
        Executable: "/TEST owned/bin/agent-notifications",
        Selector: "/TEST owned/control/hooks-binding.json",
    }},
    Shell: cursorhooks.LinuxUserShell32212,
    ExecutableVerified: true, // Host's current trusted no-follow checks.
})
```

The complete native entry is:

```json
{"type":"command","command":"<literal fixed argv>","timeout":5,"failClosed":false}
```

The invocation is `<executable> cursor-event stop --binding <selector>`. It is
one flat entry in `hooks.stop`, with numeric document `version:1`. The timeout
is native seconds. The library supplies no hook name, group, matcher, permission,
model, environment, `loop_limit`, follow-up or neutral-response shell wrapper.
The observation executable owns bounded stdin/stdout and neutral `{}`/exit 0.

## Ownership and lifecycle

`Install` without a receipt creates ownership only if the exact desired command
string is absent from every event. An identical foreign command is a conflict,
even with different fields. `Install` with a receipt repeats the same fixed
specification; a changed specification requires `Update`.

`VerifyOwned(document, receipt)` requires one unique exact full entry in `stop`.
Duplicates, cross-event command collisions, moving the entry and editing any
entry field conflict. Array position is not an identity. Unknown fields added
to the owned entry participate in its full digest and cannot be overwritten.
The trusted receipt also binds the fixed prior specification to its entry
digest; inventing a receipt around edited config is not an adoption workflow.

`Update` replaces that verified entry at its current position, checking the new
command for collisions first. `Remove` removes only that entry. Both allow
legitimate foreign edits. Remove requires the receipt even if the executable
has already disappeared; it does not require current executable attestation.
It leaves the document and empty containers in place. A second removal with an
old receipt conflicts; the host records completed removal in its own ledger.

`Repair` uses only the receipted fixed prior spec (omit `Specs`, or supply exactly
that spec). The owned command must be absent, or already intact, with its
verified stripped-document remainder unchanged. A missing command plus any
foreign drift returns `ErrAbsenceUnproven` wrapped by `ErrConflict`; a changed
command cannot become a deletion. Repair also refuses drift when the owner is
intact. It cannot filter out receipts and reinstall. No additional recovery
authority is needed for this slice or exposed by this API.

Receipts contain version, `stop`, the own executable/selector/shell and two
SHA-256 digests. They contain no foreign document bytes or transient snapshots.
The entry hash covers every field. Object keys are sorted, arrays retain order,
strings use native UTF-16 units, and number tokens remain exact. Whitespace and
equivalent string escapes do not affect hashes. Number lexeme changes are
conservatively treated as drift. The remainder excludes the owned entry and
normalizes only an empty `stop` array and empty `hooks` object. Foreign empty
events/objects remain significant. Whole-file absence can be repaired only if
the prior proof had no foreign remainder.

A successful repeat/update refreshes the remainder digest, including a byte
no-op with legitimate foreign edits. Persist the returned receipt under the
host's transaction fence; `NoOp` applies to document bytes, not receipt state.
Receipts are trusted host state, not authentication or effective-policy proof.

## Grammar, limits and preservation

Input/output is strict UTF-8 JSON, at most 4 MiB, depth 64 including the root,
and 65,536 nodes counting containers, keys and scalars. An allocation-free
lexical budget pass runs before recursive AST parsing/traversal. Duplicate keys
are rejected everywhere, including escaped/native UTF-16 equivalent keys.
Native numeric spellings of version 1 are accepted and preserved. Foreign
numbers are never converted to floating point. Foreign lone surrogate escapes,
composed/decomposed Unicode and original string/number lexemes survive edits.

Hooks must be an object and every event must be an array. Future foreign event
and entry semantics remain opaque. The private AST operations selectively adapt
the existing nativeconfig/hujson primitives; there is no new shared config
framework, JSONC engine, Gemini schema or YAML fallback.

No-op/refusal returns the original bytes exactly. Input is never mutated. On
oversized refusal the returned `Desired` borrows the input to avoid an oversized
copy; other results have independent bytes. Errors expose fixed reasons, never
foreign values. `errors.Is` recognizes conflict, unsupported representation and
unproven absence separately.

## Shell evidence and host obligations

`RenderArgv` supports only `LinuxUserShell32212`: the explicitly selected local
Linux user-hook single-quote grammar, with apostrophes encoded by adjacent
single/double quoted segments. It emits a literal command without `exec`, shell
option changes or failure/output guards. Tokens are at most 4,096 bytes, argv
at most 32 tokens/16,384 bytes. Empty ordinary args are supported; NUL, CR/LF,
invalid UTF-8, dollar and backtick tokens are unsupported. Executable and
selector paths must be canonical absolute Linux paths, not `/` or `//` aliases.

The supplied Cursor IDE 3.22.12 Linux spike observed `workspaceOpen`, config
discovery, ordinary adversarial argv and cwd `TEST HOME/.cursor`. It did not
observe native `stop`, a model session, CLI hooks, Notifications delivery,
restart or any other platform/version. The pinned hook service source shows
plugin-root replacements in the `claude-plugin` branch, and user commands
passed to `executeHookDirect` with a separately built native environment.
The complete backend preprocessing source is unavailable in this workspace;
retrieving the pinned bundle was denied by the network allowlist. Dollar-like
forms therefore remain refused even though the narrow native spike recorded
one dollar literal. Do not broaden placeholders or literal forms by assuming
quoting disables native preprocessing.

The regression recorder executes the JSON-decoded command under actual
`/bin/sh` in fresh TEST scratch with a minimal environment. Its broader token
matrix is target-shell proof only. It does not establish the exact native
backend shell identity or broaden the supplied native qualification. The caller
must qualify the selected backend before activation. `WindowsUnqualified` and
all other contracts explicitly return `ErrUnsupported`; Linux roundtrips and
cross-builds cannot qualify Windows. Real Windows native CI remains required.

The host must verify a fixed regular owned executable, execute permissions,
bounded confined selector and no-follow canonical parents before setting
`ExecutableVerified`; pure functions cannot verify those filesystem facts.
Bind receipts to the selected physical profile/path/binding/generation, check
effective restrictions and independent consent, and retain disabled state.
Native cwd is the user's `.cursor`, so every invocation path is absolute.

For writes, reuse the existing nativeconfig `Kernel.BeginExactFile` writer
lease and reread snapshot. Notifications retains its sole hook-file writer
under that lease; it must not also call `ExactFile.Apply` for the same write.
When the kernel itself is the selected sole writer, `ExactFile.Apply` provides
the existing exact-byte CAS seam. Metadata preservation/refusal, readback,
rollback and cross-file receipt fences remain host/kernel responsibilities.
No shared edit or wrapper is needed for pure UC2. The current acquisition API
has no context argument; a required bounded-context lease belongs to the kernel
owner before downstream lifecycle use. This slice does not prove ACL/xattr
support or remove the non-cooperating writer race.

## Focused verification

Public-package tests exercise fixed native grammar, unique full-entry
ownership, foreign lexemes/order, byte-exact refusal/no-op, narrow repair,
legitimate foreign edits, Unicode identities and all resource limits. Every
test states its observable red condition. A fresh executable recorder tests
JSON transport through the actual target shell; no test launches Cursor,
provisions anything, authenticates, touches a real profile or runs a model.

Run from the agentplugins module with the explicitly provided toolchain and
offline isolated Go caches: `go test -p 2 -count=1 -timeout 90s ./cursorhooks`.
Set `CURSORHOOKS_TEST_ROOT` to an existing absolute own `.research/tmp/` TEST
directory, and `TMPDIR`, `GOTMPDIR`, `HOME` and `USERPROFILE` to fresh TEST paths.
The shell recorder fails if the TEST scratch boundary is missing. Non-Linux
runs skip only the Linux recorder and retain explicit Windows refusal checks.
