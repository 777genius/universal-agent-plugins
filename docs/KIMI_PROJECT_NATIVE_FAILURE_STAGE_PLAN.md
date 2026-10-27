# Kimi installed collectors: finite failure-stage diagnostics

## Fresh actual failure and controlled source

PR524 current published source a4a211d60595abbb8592e76e27846ba6c349a6d0,
tree975a0faff2689474fe9b6d07c663af0fec78933a, has runtime40452608070.
Both jobs now pass public installation and the corrected receipt/schema proof.
Component122018618870 completes user/A/B/C adds, then observe-installed exits1
with zero stdout and only "Kimi installed API observation failed; private capture
quarantined". Single-tool122018618873 completes A add, then native-attempt exits1
with zero stdout and only "Kimi single-tool observation failed; private native
streams quarantined". Neither capture reveals the failing callback operation,
exception category or nested service cleanup result. No HTTP shape or specific
root cause can be inferred from those generic diagnostics.

Native component12711270709 outer SHA256
b898629adf03caef42df54afaa4a394b4ccaf63bcbe59bb7300c3f4e44579e10;
native call12711270602 outer SHA256
12fbc3c6083f530393aee3196c7af3e493d29b7fcd5717414fe0e77938b25abc.
Matching frontends12711445369/12711380421 were independently checked with the
native archives for exact outer hashes, bounded canonical unique ordinary ZIP
inventories, CRCs, exact committed source/tree and empty patch list. All five
native qualification flags and vendor/model flags remain false. Core40452608159
and Terminal40452608141 now pass; they are not native-runtime qualification.

Local work/kimi-a4-diagnostics baseline7e10e7da7a883c8c371d837e37a99b331fbc3b54
has exactly the current published tree975a0fa. It was reconstructed from the
earlier exact tree31a95 plus all six immutable official current-file blobs;
each full git-blob hash and the complete resulting tree were asserted. Preserve
all six externally published schema/Windows/noctx files byte-for-byte. The old
local schema/noctx plan/code sequence through5ac10bf remains on its original
branch and is not transplanted onto this baseline.

## Existing capability inventory: preserve, do not republish blindly

The earlier local4406458a finite ResponseSchemaRefusal capability is already
byte-identical in this current published tree: kimi_attempt.py SHA256
326d4daf8682ad3a85f5251b551cc354a21d330ae4ac092d36c9905558d8e0ef,
test_project_native_kimi_attempt.py SHA256
8cac1d82093c87e73ba0439325888b3014a251c1d174b8f64236af5e99f69649.
It catches only registered later session/prompt/approval/snapshot/history shape
refusals, stops later tool actions, obtains the same graceful shutdown/reaping
and emits only the existing admitted redacted closure with failure status.
Authority/permission/readiness/transport/cleanup errors remain quarantined.
Preserve that implementation and its meaningful inert supervisor tests. Do not
broaden its catch, expose quarantined HTTP or treat this new generic failure as
a known ResponseSchemaRefusal.

## Diagnostic-only implementation after two independent plan GOs

Scope is the two existing collectors scripts/project_native_collectors/kimi.py
and kimi_attempt.py, their two existing test files scripts/test_project_native_kimi.py
and scripts/test_project_native_kimi_attempt.py, and this plan/validation record.
No Go, shared service/frontend, provider, native grammar or authority file changes.

Add one small Kimi-only failure tracker in kimi.py, reused by the existing
kimi_attempt import. It stores only a registered stage, registered phase, a
finite safe exception category and boolean reached status. Its report must be
constructed from an explicit closed schema, never an arbitrary dict/object.
Do not include exception str/repr/traceback/class name, native output, request
or response bytes/keys, URL/path, bearer/key, nonce, session/call/workspace ID,
stdout/stderr, environment, filenames or user-supplied request.stage.

The finite stage inventory covers main host-admission, request-read,
request-validate, authority, immutable-closure, context, service-entry, ready,
meta, health, unauthorized, workspace-a/b/c, trust-before/set/after-a/b/c,
trust-untrusted-c, catalog-a/b/c/user, mcp-outside, mcp-a/b/c, skills-a/b/c,
test-a/b, test-refused-c, probe-events-a/b/c, distinct-workspaces, pre-call,
create-session, prompt, approvals, approve-once, snapshot, history,
preserved-authority, shutdown, service-cleanup, serialization and emission.
Only the stages actually used by each collector are instrumented. The repeated
approval/snapshot indexes remain private and map to their single registered
stage. Phase is one of enter, request, envelope, validate, complete. Unknown
stages/phases cannot be rendered; registration errors fail closed.

Before an existing API operation mark its finite stage/request; after the same
bounded service.http returns mark envelope, and before returning its validated
envelope mark validate so the existing caller's strict object/value checks are
attributed to that operation. Mark readiness/authority and the existing
non-HTTP checks at their actual boundaries. A wrapper around each existing
observer callback records only the safe category of an uncaught exception
before the unchanged Service swallows it; it rethrows that same exception.
If the callback returns normally, mark service-cleanup before the unchanged
Context.service completes supervision and immutable rechecks. Do not catch a
failure and continue to another HTTP, approval, prompt or tool action.

Safe exception category comes from a finite type mapping (validation, type,
key, OS, timeout, other), with no dynamic type/module names or message. The
original failure/exception and return-code behavior remains unchanged.
Callback-entered is true only if that wrapper was actually entered. A recorded
service result is reported only from the actual existing Context summary
command, not inferred from a generic failure.

On the existing generic entrypoint exception branch, emit one bounded line of
static-prefix JSON to stderr (maximum2KiB), followed by the unchanged generic
quarantine failure. Report the finite stage/phase/category, observer-entered,
service-recorded and these five cleanup values copied only when present as
exact bools in the actual Service result: shutdown_acknowledged, sigterm_sent,
forced_kill, group_reaped, port_closed. Missing/unreported values are null,
with cleanup_reported=false; never turn unknown into success. Do not serialize
any other service or Context field. No new stdout envelope or artifact member
is added for generic failure. Existing stdout quarantine and the original
closed evidence/secret checks remain unchanged.

Do not emit a new failure report on success or alter the already supported
ResponseSchemaRefusal path. Preserve its failure return1 and redacted graceful
capture, existing API budget/reserved shutdown slot, original hosted guard,
150/60/65-second budgets, process signals/reaping, endpoint and credential
authority, request inventory, trust/permission/manual approval, source/pins,
cleanup/admission and all qualification flags. This slice identifies a failure;
it does not accept or repair it.

## Meaningful verification and publication

Extend the existing inert tests with failures containing planted bearer/key,
path, nonce, session and arbitrary exception text. Exercise the actual tracked
collector callbacks and their existing inert service supervisor where possible,
then render the failure line and require the exact closed fields/enums, finite
size/no raw linebreak, no private value/exception text/HTTP body and failure
status. Cover pre-service failure (no cleanup claim), readiness, HTTP transport,
strict envelope/validator, permission/authority and post-callback cleanup
failure. Compare cleanup booleans with the actual retained supervisor record,
including false and unavailable values. Verify the first failed operation is
the last attempted action; no extra API, grant, prompt or tool is introduced.
Unknown stage/category inputs and malformed cleanup dictionaries cannot cause
arbitrary data to enter stderr.

Keep and run all existing Kimi collector/attempt and shared service inert
contracts, including actual inert Python child cleanup/quarantine tests and the
previous finite-schema-refusal graceful capture tests. These are not native
Kimi/model/provider observations and require no Go slot. Run Python syntax and
whitespace checks and independent code review before root-owned publication to
the existing PR524. No vendor binary executes locally, no host spoofing, new PR,
release or VS Code work is included.

Fresh genuine hosted source-bound component and call captures must identify
the actual failed stage/category and truthful cleanup status before any later
behavioral or grammar amendment is planned. Do not relabel the existing failed
runs, assume a response schema mismatch or promote any native qualification.

