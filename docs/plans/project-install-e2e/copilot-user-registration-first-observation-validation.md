# Copilot user-registration first observation: exact implementation validation

## Reviewed source and prerequisite

The accepted plan commit is `24ffbce31311eecbbb32cc3857f7b12c7870de53`,
following exact successful readiness source
`189e00d364ccde805c9ffc6738e61fc58ae277fc` on program base `a412ba5e`.
Root R1 and native-finish R2 approved plan SHA256
`84b24bf24240ea52cfed5211dc23463874a2a03066449cbf39c57e57d5b52072` before code.
The prerequisite's two separate source-bound raw reviews are recorded in that
accepted plan. Neither review turns its five lifecycle/MCP or account/model
flags true.

Code commit `a2f312ee27f6d15202edbfd12d8fad4c01381d2c`, exact reviewed tree
`ca9013594bd728ece3d0bb47e3192187b130fd42`, changes ten code files. Root and
independent CLI reviewers read the complete source, shared guards and accepted
plan and gave exact-tree CODE GO without remaining findings. No code changed
after those reviews. This record does not attribute the author's test execution
to either static reviewer.

The old readiness entrypoint remains callable with its original opt-in. Its
entire function body and both existing readiness semantic-reader files are
byte-identical to the successful source. Preparation is a private extraction
with the original Git/config/version/package sequence. Only the explicit new
mode uses the closed four-command matrix, user/outside scope and failed-only
finalization. The canonical Copilot selector selects that new diagnostic;
there is still exactly one automatic Copilot row.

## Actual gates

The following checks passed on the unchanged reviewed code with Go 1.26.10,
golangci-lint 2.13.2 and serialized local validation:

- Focused `go test -p 1 ./repotests -run '^TestCopilot' -count=1`: PASS, 0.290s.
  Existing inert loopback protocol tests used local loopback permission; no
  vendor or public installer executable ran.
- `PYTHONPATH=scripts python3 -m unittest test_project_native_copilot_user
  test_project_native_copilot_readiness test_project_native_runtime`: 42 PASS,
  7.184s. An earlier invocation without the required module path failed to
  import the modules; it did not run the contracts or alter source.
- Actual all-enabled full-root lint with
  `--new-from-merge-base=a412ba5e137fdcb7e004393229b323211ed746e1`: zero issues.
- Unfiltered whole-root revive/funlen/gocyclo/gocognit/dupl/depguard: zero issues.
- Four Python files parsed; gofmt and staged whitespace checks were clean.

Separately, the actual current189e successful native ZIP replays through the
updated shared reader: 21 members, runtime/collection complete, test passed,
all five/account/model qualifications false. Its inner SHA256 remains
`4cb870b0a2c80ac1be98a36db2b58868f6ba8af9187d52092f7a3d6042a42de7`.
The older genuine failed fixture and its manifest remain unchanged. No old
capture was rewritten into the new mode.

## Boundaries demonstrated and remaining

The new real-file controls cover unreadable private bodies without content
reads, unknown names and values absent from exported metadata, exact root-
inclusive 1024-entry and depth32 bounds, aliases/FIFO/hardlinks, same-byte inode
replacement and mode facts, coherent parent replacement, original-context
exhaustion, and deferred observation through Goexit. Projection checks cover
full counts, deterministic unique hash order, true omissions, and the complete
serialized object capped at 2048 bytes independently of at most16 records.
Command/reader controls refuse partial or out-of-order triples, wrong scope,
cwd, flags, deadlines, source/scanner pins, unexpected provider/native/installed
members, false completion, qualification promotion, private diagnostic values
and a later review-barrier label masking an earlier process failure.

This mode performs no project install, native chat/session, provider or SDK RPC.
It creates no account, login, trust or direct registry state. Original immutable
and config-digest guards, ordinary descendant supervision and original
150-second work/180-second total/30-second command budgets remain required.
Unknown file bodies are never opened for reading by the metadata walk.

No user-registration native observation is claimed by these inert checks.
The next genuine hosted capture is intentionally failed and must be reviewed
at its actual source-bound boundary. All five, vendor-account and real-model
flags remain false. A full lifecycle, user/project catalog precedence and
zero-tool provider grammar still require their own source or measured evidence.
Root alone owns publication and fresh-head reconciliation; no release is made.
