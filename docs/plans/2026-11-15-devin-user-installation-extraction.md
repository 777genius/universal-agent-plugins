# Devin User installation extraction

## Scope and dependency

This slice adds the measured exact Devin **3000.12.57** version to User/Global
file installation. Existing User 3000.11.205 support remains. Project continues
to require exact 3000.11.205, the original selected primary `devin` executable,
and the existing Project authority. The client ID remains `windsurf`.

The parent is `a1d55d6970a48fd32bd3cd3db32b9fe185262fe4`, tree
`abc6d7517db7ff881acb5110bd84823c359f9f27`. Its reviewed Antigravity extraction
provides the generic, non-serialized `ClientNativeConfigRoot` handoff needed by
saved User/SDK installation. This child changes no shared installer or CLI
production path. Merge the parent before this slice; final Required and Core
checks must bind the resulting child source and actual main base.

Exactly six paths belong to this change: the three Windsurf adapter files
`global_profile.go`, `global_lifecycle_test.go`, and
`global_version_refusal_test.go`; `installer/devin_global_host_test.go`; the
existing Core workflow; and this plan.

The three adapter files are exact selected-source blobs from
`a610eadf5436dd3fb2b29fbb069615028b46ad00`:

| File | Blob |
| --- | --- |
| `global_profile.go` | `16a30a1eb7d5c71591213a4c807dfcf581224563` |
| `global_lifecycle_test.go` | `8f2983f0a3d8b6f58961265ab0203d8c902879d1` |
| `global_version_refusal_test.go` | `16fc88ed6c4b80502219e91c7a7fdb5c4354dc9a` |

## Behavior and ownership

User version admission accepts exactly 3000.11.205 or 3000.12.57. A missing
observed version returns `VerificationIndeterminate` before native-config or
process effects. Wrong client/executable, adjacent or decorated versions, and
invalid or aliased roots still refuse. Project version admission is unchanged.

`NativeConfigRoot` carries the independently observed User profile; `ConfigRoot`
still identifies legacy Cascade authority. Existing recorded Cascade and
package-only receipts retain their route. Mixed or retargeted receipts refuse.
No implicit migration, model invocation, authentication, or MCP protocol call
is added. File installation continues to report Prepared/installation_verified.

The SDK test is based on source blob
`2cc58853a2393bb31f9a2342fd126785446525a0`. Its original non-opted Antigravity
fixture is replaced with real Cline because the parent now intentionally opts
Antigravity in. The negative requires `ErrInvalidRequest` and the exact existing
native-root admission reason. The original private/frozen-root, state, foreign
profile, recorded-root, and zero-process assertions remain.

## Three-platform validation

The existing Core `project-contracts` matrix already runs Project methods on
Ubuntu 24.04, macOS 15, and Windows 2025 with Go 1.26.11. A new bounded source-only
step selects 15 current User/legacy top-level tests: six Global adapter methods,
two original-root SDK methods, three User CLI version methods, three recorded
receipt-route methods, and exact owned removal. The preceding command, all
jobs/triggers/runner choices, and Project coverage remain unchanged. The new
step uses Bash fail-fast and `-p=2 -count=1 -timeout=3m -v` across the three
existing packages so actual RUN/PASS names can be checked.

The tests use real private files and state with rejecting process controls;
they do not claim a native model or MCP session. Existing SDK helpers are reused.
No broad metadata collector, logger experiment, research workflow, or composed
CLI fixture expansion is imported.

## Retained genuine Linux evidence

[Run 43640883052](https://github.com/777genius/universal-agent-plugins/actions/runs/43640883052),
[job 132613737968](https://github.com/777genius/universal-agent-plugins/actions/runs/43640883052/job/132613737968),
completed successfully on source `0633402a9c992fd941caebd74a88a0fbe07220bd`,
tree `3de63b31367d544dd93654f7b17a8eee4c9beddd`. Artifact 14136868649 has outer
SHA-256 `6ecccf5f600856b808889e8759b1656686b8ecb618292d965bf6dcfb60406377` and
inner SHA-256 `8c2298b1ec809653593cf64b9af759121b5b6dbb4d116a5a444e0fb788312153`.
Both CRC checks, 79-member closure, and all 78 command-stream byte/hash joins
were replayed. All 39 command records have exit 0 and no capture error.

The original Linux vendor pin was
`https://static.devin.ai/cli/3000.12.57/devin-3000.12.57-x86_64-unknown-linux.tar.gz`,
archive SHA-256 `0946784ef886d716e82a8026f76933bf0a6e4f0a804a23722bb62047e8c07d4a`;
the extracted executable SHA-256 was
`3f3fb99f472d92f7852be3775ac2b66b5d60baf26f3441b02d54c56b160bc30d`.
These are original acquisition-receipt facts; no vendor download was repeated.

The catalogue contains seven Git/source commands, 27 genuine Devin version/help
or MCP/Skills inventory calls, and five UAP operations: User add without saving,
doctor, repair of a deliberately removed owned Skill, doctor, and remove.
Five public response/state/effect joins preserve the IID, source/package,
original native root, binding objects, and DATA receipt. Foreign native rows
remain equal to baseline. After removal, MCP/Skills stdout from both Project
observation directories is byte-identical to baseline, bindings are absent,
and the original DATA remains retained.

The bounded replay receipt SHA-256 is
`e91c5015694d0cef50c73abccea7f88cc43232e6865612a206755fb1b4e1abce`.
This is historical Linux evidence, not execution of this extracted source or
Windows/macOS native qualification. The original report's false MCP runtime,
restart/removal, Project lifecycle, and admission flags remain false. They do
not block this file-installation slice.

## Review and remaining gates

The source R1 was approved before authoring. Independent R2 closed the six-path
allowlist, three exact adapter copies, corrected SDK negative, current 15-test
selector, all prior workflow bytes, and unchanged shared dependencies.

Actual local execution passed the full Windsurf adapter package (11 top-level
methods, 33 named cases), User plus Project SDK tests (5 methods, 21 cases), and
all three User CLI version methods. None failed or skipped.

A scratch Go overlay restored only the original `global_profile.go`. It
compiled and failed exactly six named cases: the new 3000.12.57 lifecycle and
its parent; the two missing-version modes and their parent; and new-version SDK
preparation. Eleven other cases still passed. The SDK failure occurs at the old
version gate; it is not a separate counterfactual for frozen-root protection.
Independent review verified the original JSON events, hashes and intended
semantic assertions. Candidate source remained unchanged.

Required full lint/core and ordinary final-source three-OS results remain
validation gates. Queued, cancelled, skipped, or historical checks are not
final acceptance. No new native runtime qualification is inferred from these
file-installation tests.
