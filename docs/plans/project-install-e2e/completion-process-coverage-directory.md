# Keep completion stderr exact under Go coverage instrumentation

Core run `40201578593`, job `121193687884`, at source
`6a1d53f4b69f6761dd62039bd36404dad7119a04` reports
`TestReleaseCompletionProcessStderr` case zero with a successful exit and
54 stderr bytes in its informational instrumented pass. The fixture's source
blob is `9a45d418515bbba326a48e313cd6f247d4d96e2b`, unchanged from main.

The child re-executes only its own Go test executable. Its explicit environment
omits `GOCOVERDIR`. The pinned local Go 1.26.10 producer,
`src/internal/coverage/cfile/emit.go:133`, writes a fixed 54-byte warning when
that variable is empty in an instrumented executable. The log reports the byte
count rather than the raw stderr; deterministic instrumented red/green checks
will verify the fixture correction. No vendor executable, account, credential
or native containment prerequisite is involved in this own-test child.

## Exact correction

Allocate one separate `t.TempDir()` beside the existing private fixture home.
Set `GOCOVERDIR` to that coverage directory in the existing explicit child-env
allowlist. Do not inherit the caller's coverage directory or whole environment,
reuse the profile home for coverage files, filter stderr, change any completion
argument/protocol, change child dispatch, or relax the zero-stderr, token/debug
file and output assertions. Uninstrumented execution keeps the same assertions.

The dedicated Coverage workflow currently excludes this test, with a comment
covering it and the unrelated version-probe fixture; this exclusion appeared in
commit `d50723354067e0bdada0e59b0db54dcbb7e190e8`. After the instrumented fix is
verified, remove only the completion exclusion from its CLI coverage command.
Move/narrow the comment to the remaining version-probe exclusions and preserve
those exclusions, instrumentation, package selections, timeouts and all other
workflow settings. The corrected completion test must run in that coverage gate.

## Review and bounded validation

Root and the second reviewer perform two independent plan rounds before edits,
then independently review the frozen source/workflow diff. Coordinate the one
Go build slot and disk headroom before running the unchanged exact test with
`-cover -count=1`: it must reproduce the instrumented failure without source
edits. Then make only the planned test/env and workflow edits. Run the same
instrumented test, the exact test with atomic coverage, and the uninstrumented
test; require every completion case to pass with the original assertions.
Run affected-source configured lint, YAML parsing and exact workflow-diff checks.
Do not add a mirror test, disable coverage, change Core's informational status,
execute a vendor client, bypass a native gate, or repeat broad suites locally.
Publish the narrow reviewed fix promptly; final normal CI remains required.
