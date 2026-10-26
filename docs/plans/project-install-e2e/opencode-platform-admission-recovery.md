# OpenCode platform admission and exact public progress: recovered candidate

This source reconstruction follows the previously twice-reviewed platform plan
31393f78 / final 1e5b742701ded5ad3de3fe62b00e9e0c026a8870. It needs two fresh
code approvals on its new tree. No old whole-candidate approval is inherited:
0ecab091's whole approval was withdrawn when its repeat-add stderr expectation
was found unreachable in the actual public command dispatch.

## Actual observations and limits

Genuine run 40145166981, source 46b4678ea66d7804311f98270b70ebd3cceb928a,
failed before project discovery qualification. The macOS OpenCode job
121024834151 passed all four existing user lifecycle gates, then stopped on the
first public UAP user add because successful public progress was treated as
dirty stderr. Its bounded project artifact retained false qualification flags.

Windows OpenCode job 121024834126 and Codex job 121024834145 failed in the
matrix safety tests before native execution. Their observed Python traceback was
3.12.10: importing the POSIX resource helper failed, and the positive project ZIP
admission test returned false without retaining its cause. The Windows metadata
explanation below is source-backed, not a locally reproduced Windows diagnosis.

No vendor binary is acquired or run by this patch's local tests. No workflow
selector, native argv, guard, user gate, capture limit, model/MCP/trust contract,
or project ownership assertion is relaxed. Hosted Windows/macOS success remains
a subsequent requirement.

## Platform model

The matrix records the actual ambient Python implementation, version and
executable. An upstream source tag is evidence about implementation semantics;
it does not pin the hosted interpreter.

CPython v3.12.10 path stat copies Windows birth time into ctime, whereas held-fd
stat exposes ChangeTime. The publisher retains the complete identity before and
after within each domain, including each domain's ctime. Cross-domain matching
requires positive volume/file identity and birth time, identical mode/link
count/size/mtime/attributes, and absence of reparse attributes/tags. Missing or
zero identities refuse. It never compares unlike ctime meanings or discards
same-domain change detection.

Primary source:
- https://github.com/python/cpython/blob/v3.12.10/Modules/posixmodule.c
- https://github.com/python/cpython/blob/v3.12.10/Python/fileutils.c
- https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilew
- https://docs.python.org/3.12/library/msvcrt.html#msvcrt.open_osfhandle

Windows opens use typed pointer-width CreateFileW, read-only existing-only,
noninheritable handles with OPEN_REPARSE_POINT. The descriptor transfer requests
binary mode. Invalid handles never reach transfer/close, failed transfer closes
the raw handle exactly once, and successful transfer gives the descriptor sole
ownership. Win32 failures remain typed; there is no ordinary-open fallback.

The publisher admits bounded immutable in-memory bodies and verified stream
hashes. It rechecks the original root/member path metadata after ZIP assembly
and before publication within the original four-second budget. It does not
claim continuous writer exclusion. Subsequent writes cannot change the bytes
already captured in the admitted ZIP.

Only the six POSIX capture tests are skipped on non-POSIX systems, before
importing their POSIX-only helper. A pure import regression verifies that scope.
All other safety/admission/native gates remain enabled.

## Exact public progress contract

Capture/context/pipes errors refuse before any semantic or progress parsing.
Only ordinary public UAP commands admit these exact bytes. Native and Git stderr
stay strict. Unknown labels, extra bytes, missing lines, CRLF substitutions and
warnings refuse.

| Labels | Exact stderr, each displayed line ends in LF |
| --- | --- |
| add-user | Resolving and validating one Agent Plugin package for every selected target... / Installing uap-opencode-user-control for 1 selected clients... |
| add-A, add-B, add-same-user, add-same-A, add-json | Resolving and validating one Agent Plugin package for every selected target... / Installing uap-commands-fixture for 1 selected clients... |
| repeat-add-A | Resolving and validating one Agent Plugin package for every selected target... / Applying transactional client package... |
| unchanged-update-A, update-A | Resolving and validating one updated Agent Plugin package for every selected target... |
| intact-repair-A, missing-repair-A | Resolving and validating each unique exact installed package revision once... |
| remove-A, remove-json | Empty |
| repeat-remove-A | Separate existing exact absence contract: pure unwrapped ExitError, exit 1, empty stdout and one of the two source-established fixed refusal lines; owned state/files unchanged |

The first/repeat add distinction is measured by
TestOpenCodeProjectPublicRepeatProgressSource through the public Cobra command
and production scoped composer, planner, stager, kernel and saved state. It uses
only an inert compiled TEST --version process, an explicit disposable profile
and inert security admission. The first add takes the group Installing path.
The existing same-scope binding makes repeat add enter runAddLoaded before that
line and take Applying transactional client package. The measured repeat
reports mutated=false, keeps Prepared / installation_verified, and preserves
the complete original user tree and project bytes/modes, including state,
managed package and native skill/MCP configuration. Only the expected process
mutation-lock payload changes; its presence/type/mode remain checked.

Fresh measurement after reconstruction: PASS 0.988s. The earlier exploratory
measurement was PASS 1.439s; an intermediate stronger whole-tree comparison
correctly exposed the process-lock payload refresh and was narrowed explicitly
to preserve all persistent content rather than falsely asserting lock bytes
were immutable.

## Local verification and review gate

- Five platform tests cover separate path/fd ctime stability, every positive
  cross-domain field, root/member/held reparse refusal, binary CRLF/Ctrl-Z bytes,
  post-ZIP mutation refusal, typed flags/handle transfer and failed-transfer
  ownership. Together with five existing envelope tests: 10 PASS 0.120s.
- Public source first/repeat proof: PASS 0.988s; no vendor execution.
- Focused inert parser/capture/loopback recorder suite: PASS 0.209s. The first
  sandboxed invocation refused the loopback socket before the test body; the
  same finite suite passed under the already authorized execution environment.
- Configured changed-source repotests lint was attempted with the unchanged
  90-second bound twice. Both ended with exit 4 / timeout after reporting zero
  issues, so neither is a lint pass. The first used an accidental read-only
  default cache; the second used the explicit shared writable cache and had no
  cache warnings. Full exact-source lint remains a hosted gate. Budgets and
  checks are unchanged.

Two fresh code GOs and parent-controlled publication are required before this
candidate reaches an executable hosted workflow. No native qualification is
promoted by these local checks.
