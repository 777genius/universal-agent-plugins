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
