# Pinned Grok native MCP call through a local protocol driver

Scope: four new test/CI/documentation files; no production or #878 automation
changes. Reuse the merged `grok_project_protocol_e2e.Journey`, acquisition pin,
ordinary UAP CLI, controlled MCP probe, ownership checks and explicit disposable
Project trust fixture. Existing direct-client evidence is not reclassified.

## Native admission

The official Grok custom-model guide documents a per-model OpenAI-compatible
`base_url`, `api_backend = "chat_completions"` and headless `-p` / `-m` selection.
The permission guide documents `dontAsk` and an exact `MCPTool(server__tool)`
allow rule. These current documents are candidate contracts, not proof that the
pinned 1.0.70 executable implements them. Require its original help and actual
loopback requests. Unknown API/auth/tool naming fails closed and stops review.

No account, vendor credential, actual model or inference service is used. A
bounded deterministic server implements only the documented local provider
protocol. Any inert local SDK key is accepted only by that owned loopback server;
it is not vendor authentication. No login/auth file, broad approval, yolo,
always-approve, plugin-dir auto-trust or alternate MCP configuration is allowed.

## Proof

Run the real pinned native Grok process against the UAP-installed Project A
configuration. The local driver uses native `search_tool` discovery, then selects only the exact
qualified `uap_probe__inspect_runtime` via native `use_tool`, supplying the fixture
nonce and marker.
Only Grok may connect to the MCP process. The driver never opens an MCP socket
or launches the server directly. Require a new ordered same-session
initialize/list/call receipt, exact nonce, revision, plugin/data roots, current
directory and marker; require correlated native tool output at the provider.

Exercise installed, updated and missing-package-repaired states by overriding
only Journey.protocol. Preserve the existing repeat-add identity, partial-edit
repair refusal, user/foreign protections, A/B isolation and retained-data
checks. Add genuine Project B and post-removal native sessions which must not
advertise/call the removed owned tool, while the held state remains unchanged.

The provider is loopback-only with bounded request count, body, socket timeout,
stage deadline and finite responses. Unsupported paths/protocols fail. It emits
one native discovery followed by one owned invocation, then terminal text only
after the exact native result. Auxiliary requests never qualify that invocation.
Retain source/tree/run, binary/acquisition digests, bounded native captures,
request/response digests and original MCP events. Linux x64 only; no other
platform, HTTP MCP, vendor model or default activation claim.

## Review and acceptance

1. Inert negative contracts: malformed/oversized requests, wrong model/tool,
   duplicate matching tools, wrong nonce/result/root, repeated calls and missing
   evidence refuse. Streaming and non-streaming local protocol are tested.
2. Finite pinned native API admission before product acceptance. A failed spike
   stays unqualified; no invented expected native transcript or relaxed flags.
3. Actual isolated hosted source-built UAP/native lifecycle and negatives,
   independent original artifact hash/CRC/event review, applicable ordinary CI.
4. SHA-guarded merge and genuine natural post-merge execution.

Files: `scripts/grok_noauth_call_e2e.py`,
`scripts/test_grok_noauth_call.py`, `.github/workflows/grok-noauth-call.yml`, and
this plan. Approximate 600–800 new lines; protocol/admission risk 4/10. Reuse
existing helpers rather than adding a client adapter, framework or installer.

References:
- https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/11-custom-models.md
- https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/22-permissions-and-safety.md
- Merged #869 source and its precisely scoped no-model evidence.

## Measured pinned protocol admission

Local finite admission on the verified 1.0.70 binary observed deferred MCP:
`search_tool` discovers qualified names and schemas; `use_tool` invokes exactly
`uap_probe__inspect_runtime`. The test requires complete native discovery before
requesting this one tool, then the actual correlated native result and server
receipt. Auxiliary native session-title and no-tools summary requests receive
inert terminal text, are bounded separately, and never qualify an MCP call.
Documented default, session-summary and prompt-suggestion models are pinned to
the owned local endpoint; telemetry/title refresh are disabled in disposable
configuration. No vendor model or actual inference is involved.

The pinned binary also probes exact `GET /` on the owned provider. This receives
an explicit recorded 404 denial, with no query/body/service/auth response. All
other unknown routes, including `/v1/responses`, remain fatal. Earlier strict
admission failures remain failed originals; they are not relabeled successful.
The positive local Python-MCP admission, when run, is protocol-only. Product
acceptance requires the hosted source-built Go fixture and production App route.

Operation authority is bound to the original native provider replay of the exact
`use_tool` nonce/marker/operation arguments. The existing Go events do not log
arguments or an operation fact; they independently bind the server process,
nonce, roots and resulting marker. Require initial marker absence then creation,
and unchanged bytes/inode/size/mtime/ctime for subsequent native reads. Never
infer an operation field absent from the shared fixture. Check protected state
before replacing provider endpoint configuration, so prior native mutations
cannot be overwritten and adopted as a new baseline. Preserve original bounded
MCP JSONL bytes/hash and provider request/response bytes/hash for independent joins.

## Integration with the measured cwd correction

The first real hosted call in run 43259198583 reached the installed Go server but
failed: Grok 1.0.70 ignored the emitted cwd and launched it from Project A.
The subsequent 473d11f/9e0333d observations bind that native behavior; they do not
qualify the portable plugin-root default. Preserve those original observations.
The dedicated product correction in #886 uses the existing managed stdio launcher
to enforce the declared working directory without granting activation or trust.

This lane must be integrated with that correction before final acceptance. It
retains the actual installed MCP config before each attempt, validates the emitted
launcher against the source-built App hash and exact encoded child arguments,
then obtains the expected cwd from the inherited finite author fixture. Genuine
native calls must cover omitted cwd, plugin-relative cwd and plugin-data cwd,
alongside update, repair and retained-data checks. Project cwd and plugin root
remain separate authorities; the oracle is not weakened to match a host defect.

Project B receives normal explicit trust in the same disposable profile as A.
Before each absent-source native session, genuine inspect must report the exact
trusted selected project and no owned plugin. Untrusted suppression cannot stand
in for source absence. No real profile or installer trust policy is changed.

Provider acceptance occurs only after all HTTP workers have drained and joined.
Malformed schemas and unexpected parsed-data shapes latch a refusal; later valid
requests cannot recover a failed session. Exactly three primary requests are
required for discovery/call/result, or two for trusted absence; the existing
bounded metadata/auxiliary requests remain separate. Inert regressions cover
malformed-schema retries, late worker failure and trusted negative selection.
Source HEAD/tree/clean state and compiled executable hashes must remain unchanged.
## Delegated hosted process containment — 2026-11-14 follow-up

The genuine `6b523784b4c56d42c39c733b3f77f0cf79de9615` no-auth run
`43553250630`, job `132320243826`, artifact `14098460458` remains a failure.
Its original 104,052-byte `evidence.json` has SHA256
`04a44b04463365a4451c3c6e7780ee0338302a3106f121e77b04014e2e71fa3c`.
The pinned native `mcp disable` writer created the initially absent lock; the
first `repair-enable-owned-full-id` then exited 1 because the ordinary hosted
service could not create its delegated child cgroup. This happened before ACP
startup. The 14 original commands and 28 raw stream joins do not qualify a
native tool call or the new service path.

R1 approved a seven-path CI correction using the existing finite Cline systemd
lifecycle. Its Cline host and complete launch-vector functions remain verbatim;
the exact unit namespace, environment, compiled test selection and 180/165-second
limits remain. Only two new literal cases select the existing Grok protocol or
no-auth script. The same ordinary UID/GID, fixed root-owned system executables,
`env -i`, held source/interpreter/installer/probe inputs and strict manager and
cgroup receipts apply. No arbitrary native argv, caller unit name, host-cgroup
ownership change, fallback, native action or production enablement guard is added.
The existing disposable-host opt-in is explicit only in the two Ubuntu jobs.

One original 900-second monotonic deadline includes preflight, service, bounded
capture, closure and publication. The main capture leaves the existing 15-second
cleanup reserve; the jobs keep their 20-minute limits. No stage resets the clock.
The child writes only a private evidence directory. A complete parent capture,
positive unit/cgroup closure, unchanged held inputs, original source/tree,
strict exit/qualification joins and both original stream digests precede any
publication. Forced stop/kill, residual descendants, transport error, unknown
members, replacement or expiry refuse admission.

Early independent R2 found a provisional-output race: an append-only
`publication=admitted` could survive a later sink substitution or finalization
failure. The correction rejoins the exact held/named sink and its complete bytes
after writing. Refusal revokes only the parent's exact tail while its original
authority remains; foreign replacement or append is preserved. The workflow's
commit boundary is the launcher's successful process outcome, together with the
literal publication output. A stale string after any late failure cannot authorize
upload. Parent exit 0 means completed publication only. The separately emitted
`native_passed` verdict is checked by a mandatory final workflow step, so a clean
native exit 1 can retain its original negative bytes while the job still fails.
The native scripts, acquisition, lifecycle assertions and public Grok refusals
are unchanged. Future native acceptance requires genuine new-commit originals.

The [GitHub runner source](https://github.com/actions/runner/blob/main/src/Runner.Worker/FileCommandManager.cs),
`FileCommandManager.InitializeFiles` and `SetOutputFileCommand`, confirms the newly created, initially empty per-step
output file; the parent holds and checks that original file before the service,
and the child environment never receives `GITHUB_OUTPUT`. Standard workflow step
outcome also covers failure after the last filesystem check or interpreter
finalization; this is not a claim that a later external filesystem writer is
prevented by a non-atomic path observation.

Local evidence is inert: the actual finite manager body covers clean 0/1,
transport failure, forced stop/kill, residual descendants, malformed/foreign
manager state, collision and the original deadline. The complete Cline vector
and 165-second main capture are compared literally. Actual parent control-flow
counterexamples cover input/ancestor/sink replacement, foreign append, late budget
exit and descriptor-close failure. Both extracted final workflow shell verdicts
refuse native false, stale grants, malformed values and unsuccessful launcher
outcomes. An unchanged original 6b negative-evidence replay preserves all 28 stream
joins and `passed=false`; it does not manufacture a delegated service receipt.

The implementation includes a roughly 400-line parent adapter, a focused shared
lifecycle extraction and about 500 inert-test lines, concentrated in the reused
finite kernel; complexity 5/10 and confidence 7/10 before genuine hosted execution.
Earlier local runtime-suite attempts retained missing sparse-checkout fixture
errors. Exact Git fixture bytes were restored without changing any pre-existing
test body. Final local verification passed: 133 existing/shared runtime tests (32.321 s),
47 protocol/publication tests (0.402 s), and 18 no-auth driver tests (0.843 s).
All pre-existing test bodies remain AST-identical. YAML and four Python AST
checks passed, preserving permissions, concurrency and job limits. Independent
R2 of the frozen composition and genuine hosted execution remain required; the
earlier incomplete-fixture attempts are retained as failures.
