# Gemini installed native stderr: exact measured terminal prefix

## Source-bound observation

Current published PR526 sourcec887f66df1893aff40deda0012d869afa74af7b3 has
tree d3f155960cb42efafe31d6a8ef15ecbb63a72572, identical to reviewed local
4eced02fbb7e490f1be8662202311b680fe18426. Preserve both newly published
provider-part/wire-ID and research refresh corrections and their history.

Earlier genuine run40405537328/job121905922854 at eef6c202/tree3f17100d
captured the native installed attempt's stderr in closed artifact12698460531
(outer SHA2560aba508ca5cbaebc1054a04654d6afc6b534ecdc298b91497a14857cecb03cc0).
Its first342 bytes are exactly these three newline-terminated ASCII lines:

```text
Warning: Basic terminal detected (TERM=dumb). Visual rendering will be limited. For the best experience, use a terminal emulator with truecolor support.
Warning: 256-color support not detected. Using a terminal with at least 256-color support is recommended for a better visual experience.
Ripgrep is not available. Falling back to GrepTool.
```

SHA256 of exactly those342 bytes is
0309ffcd9f9558b019117eab516a089bfe80be8c1a596be48e3794ecf695a953.
The complete1952-byte failed stderr has SHA256
2f4e54c8fc557813d6b32779523c8daed46e8484b1afee9ae5556049b21e51c0
and contains additional startup cleanup metrics and an API error/stack. That
full failed stream is not an admitted diagnostic and must remain refused.

Pinned google-gemini/gemini-cli@6006c14a4decfe4b891c6263ad6ad5992e404b5b,
packages/core/src/utils/compatibility.ts blob47ef078831255119ded71fa972929408dabee801
defines isDumbTerminal for TERM=dumb/vt100 and emits the exact basic-terminal
and256-color warning messages in that order. The entire immutable file was
fetched and its full git-blob hash recomputed; provenance is retained under
/tmp/uap-shell-fresh-oct27/pinned-gemini/compatibility-audit.json.
The UAP fixture's unchanged nativePlatformEnvironment supplies literal
TERM=dumb; f.env also supplies NO_COLOR=1. This plan does not choose a new
terminal, install ripgrep or change that environment to hide the warnings.

## Finite guard amendment after two independent plan GOs

Change only repotests/agentplugins_gemini_project_native_attempt_test.go and
repotests/agentplugins_gemini_project_native_contract_test.go, plus this plan.

Factor the existing native diagnostic guard into a small exact-byte helper.
Keep its currently admitted empty stderr and exact ripgrep-only string.
Admit exactly one additional whole byte string: the measured342-byte three-line
tuple above. Do not use HasPrefix, trimming, substring/regexp matching, line
filtering, arbitrary terminal values or combinations of warnings.

The helper is called only at the same existing point, after the unchanged
f.command has independently required exit0, no process/capture/context error,
bounded stdout, no omitted stderr and strict descendant cleanup. Preserve that
call ordering and the fatal diagnostic refusal. A nonzero exit, provider/API
error, native/probe/stream/session/sentinel mismatch or failed cleanup cannot
be converted into success by recognizing these benign bytes.

Inert contracts independently state and hash the observed342-byte literal and
verify exactly the three admitted strings. Reject truncated/CRLF/reordered/
duplicated/extra-newline/extra-prefix variants, either terminal warning alone,
the two terminal lines without the ripgrep fallback, TERM=vt100 or changed
messages, appended startup metrics, API/stack/authentication diagnostics,
embedded NUL/ANSI controls and the recorded failed stream's additional lines.
No diagnostic is removed or normalized in the retained native artifact.

Scope excludes all production code, provider and research grammar, pins,
native arguments/environment, source/installed identity, auth/trust/permission,
original budgets, process supervision/cleanup, output closure and qualification
flags. Useful source remains preserved; VS Code is excluded. This does not
establish that the newly corrected native attempt completes.

## Validation, review and remaining genuine proof

Use only the root-granted exclusive Go/lint slot for affected inert native
diagnostic/provider/stream contracts and both actual affected all-enabled and
whole-file size/architecture lint. Replay the retained failed1952-byte stderr
offline through the helper and require refusal; any temporary replay test must
be removed before commit. Run gofmt and whitespace checks and obtain independent
code reviews before root-owned publication to existing PR526. No vendor binary,
new PR or release is included.

Fresh genuine source-bound execution must still pass the exact provider
exchange, native stream/session/sentinel, probe/installed authority joins and
cleanup. The full applicable project lifecycle remains a separate mandatory
proof. All unproved qualification, vendor-authentication and real-model flags
remain false; the old failed trace is never relabelled.

## Completed local validation

Both independent plan and code reviews approved the final two-file correction.
The exact helper resides alongside the existing native stream helper in the
contract file, retaining the attempt file's prior size. The affected diagnostic,
provider, stream, identity and artifact contracts passed (0.015s), including a
temporary offline replay of the exact retained1952-byte stderr by full hash.
That actual failed stream was refused while its exact measured342-byte tuple
was admitted; the temporary replay test was then removed. Actual affected
all-enabled lint and unfiltered whole-file size/complexity/architecture lint
each reported zero issues. Formatting and diff whitespace checks passed.
Checks used the granted exclusive slot, pinned Go1.26.10 with Go1.22 source
compatibility, private shared-memory temporary/lint directories and existing
bounded module/build caches. No vendor, installer, hosted guard or model ran
locally. The fresh genuine hosted result remains a required separate proof.
