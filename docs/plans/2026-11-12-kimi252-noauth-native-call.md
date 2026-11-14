# Kimi 2.5.2 native MCP call without vendor authentication

Status: plan/source admission only. No new native call or production-App
qualification is claimed. Preserve #880's actual native initialize/list and
trust-revocation proof as its own narrower checkpoint.

## Scope and ownership

Five new test/CI/plan files only; no shared fixture or product edits:
- scripts/kimi252_noauth_provider.py (200–280 lines)
- scripts/kimi252_noauth_call_e2e.py (300–420 lines)
- scripts/test_kimi252_noauth_call.py (180–260 lines)
- .github/workflows/kimi252-noauth-call.yml (90–120 lines)
- this plan (80–110 lines)

Reuse current kimi_252_native_web_mcp_probe's strict native bearer reader,
no-proxy/no-redirect management transport and genuine web startup. Reuse existing
kimi_stable_project_mcp_smoke's owned file/config/CLI checks and production CLI
composition; do not reinterpret its preparation cases as native runtime proof.
Use the existing source-built Go MCP probe for actual hosted acceptance, without
changing its default behavior. No #878, #832, VS Code, release or Registry work.

## Immutable vendor admission

Official tag @moonshot-ai/kimi-code@2.5.2 resolves through annotated tag
910c84bf61ff887f574cf2dc5b36bf78f03ccc60 to source commit
1c142a007c10e881602690def96057c23a2baa12. Official Linux x64 tar.gz asset is
58,177,868 bytes, SHA256
6614a511048219d53996454cac8594c9541bbb6cf6e8f0a081d4e878be511ce2.
The Darwin arm64 asset is 54,792,334 bytes, SHA256
974500f5360f21c00fb73ab978b7d362f44d6acfdb7110b975b8d7a44ff0da9c.
Verify archive/member/binary identity and actual version before running. Pin
source references separately from the executed release binary, never infer
byte-equivalence from the version string alone.

Official configuration supports a custom OpenAI Chat Completions provider with
an explicit base_url and API key. Use only a bound, test-owned 127.0.0.1 provider
with inert local protocol key; no vendor login, account, service or real model.
Current docs are candidates; inspect the pinned config and protocol source and
actual release's /openapi.json and /asyncapi.json before admitting API shapes.
Unknown protocols, schema/tool names or approval behavior fail closed, retaining
original bounded diagnostics. No speculative compatibility parser or fallback.

## Native invocation and consent

Prefer the genuine web REST session/prompt interface so a pending exact owned
MCP tool can be approved once through Kimi's normal interaction API. Keep native
permission mode manual. Match session, agent, tool-call/interaction IDs, exact
owned server/tool and nonce arguments before sending the one-shot approval;
never persist session-wide approval, use yolo/auto, disable auth, bypass trust,
or substitute alternate MCP config. If the pinned API cannot supply this
specific consent, record the gap rather than globally approve tools.

Reuse #880's explicit disposable Project A workspace registration/trust grant,
with pending cleanup set before the mutation. Revoke in finally and require
fresh native trust state plus effective catalog suppression before acceptance.
The local server-issued management bearer is private/test-owned, never vendor
authentication; do not capture its banner, token bytes, headers or WebSocket
subprotocol. Prefer REST polling if the pinned API exposes sufficient pending
interaction/result state; otherwise review a bounded genuine WebSocket client.

The deterministic provider selects only the actual advertised owned MCP tool,
sends the fresh nonce/marker and requires correlated native result. The driver
never calls MCP directly. Require actual same-process/session initialize/list/
call original probe receipts, pinned revision, cwd/root/data, marker and result.
Auxiliary provider requests never qualify the owned call.

## Product acceptance

Public source-built UAP performs add/repeat/update/missing-package repair/remove
against the exact selected Project A. Preserve original User/Project B/foreign
config and ownership/data receipts. Require initial marker absent then created;
subsequent read-stage marker bytes/stat unchanged; preserve retained data after
removal. Fresh B and removed-A native sessions cannot discover or call the owned
server. Preserve existing supported lifecycle tests rather than duplicate them.

Local finite native + Python MCP fixture may qualify protocol admission only,
with source/binary/digests/closed environment recorded honestly. Actual App and
Go fixture/native acceptance requires clean exact-source hosted Linux x64 and
Darwin arm64, independent original artifact/hash/CRC/event review, applicable
CI, SHA-guarded merge and natural path-scoped main execution. Windows and other
transports remain unclaimed unless separately admitted and actually executed.

Retain bounded original native API observations (excluding bearer), provider
request/response bytes+digests, raw probe JSONL+digest, source/tree/run/platform,
commands/exits and cleanup. Preserve failed originals without reclassification.

Risk: 5/10 until pinned API/one-shot approval admission; approximately 850–1190
new lines. Stop scope growth if a new framework or shared/product mutation is
needed; request independent source/protocol review before implementation publish.

References:
- https://github.com/MoonshotAI/kimi-code/tree/1c142a007c10e881602690def96057c23a2baa12
- https://moonshotai.github.io/kimi-code/en/configuration/providers
- https://moonshotai.github.io/kimi-code/en/reference/server-api.html
- Existing scripts/kimi_252_native_web_mcp_probe.py and kimi_stable_project_mcp_smoke.py

## Measured 2.5.2 protocol and implementation admission

The verified release binary completed five local exact-adapter protocol stages:
declined owned call (zero call/marker), one approved native call, a subsequent
native read preserving marker bytes/stat, Project B absence and removed-source
absence. Native manual mode, one-shot approval and final empty permission rules
were observed. Positive and read stages join exact provider invocation replay,
wall-time-framed native result and original same-process MCP initialize/list/call.
This local direct-config Python fixture is protocol admission only; it does not
qualify production App installation, the Go fixture or hosted OS execution.

Pinned `POST /sessions` accepts `agent_config` in its schema but the create
handler ignores it. `mcp_servers` is also not applied by session-config updates.
Do not rely on either to select a model, restrict server loading or grant
permission. Apply the measured model/manual overrides on `POST /prompts`;
observe actual native manual-mode frames and the complete offered tool schema.
Keep preexisting foreign/User MCP fixtures explicitly disabled using the pinned
`enabled=false` server field, so their preserved entries never start foreign
services. Preserve their original bytes and source ownership.

The native tool is `mcp__uap-kimi-stable-mcp-proof__inspect_runtime`. Normal
approval carries exact session/main-agent/call/interaction IDs and generic
input-display detail with nonce/operation/marker. Validate all before answering
`approved` or `rejected`, omit `scope`, and require final permission_rules empty.
The inline Node24 standard WebSocket bridge only subscribes/observes; it never
approves. Its private bearer arrives on stdin, not argv/env/artifact; origin,
frame/byte/deadline bounds and bearer exclusion fail closed. Reuse #880's bearer
and trust cleanup independently of model-provider protocol.

Original failed create/config and closed-PATH Node observations remain retained.
The actual live 2.5.2 OpenAPI/AsyncAPI and source-specific local captures remain
separate intermediate admission evidence. Full hosted production CLI lifecycle,
original artifacts, independent source/protocol review and postmerge are still
mandatory before final native acceptance.

Independent review corrected the absence oracle: Project B and removed-source
sessions must receive the same ordinary explicit test-owned workspace trust as
positive stages, then prove the trusted native catalog and offered tools omit
the owned server. Untrusted suppression alone is not removal/source-absence
proof. Trust is always revoked and freshly verified, even after this negative
admission fails. Each stage retains the original strict-read Project MCP config
bytes/hash before any native attempt and requires them unchanged after cleanup;
missing config is recorded explicitly. Earlier untrusted negative observations
remain narrower protocol evidence, not this corrected negative acceptance.

## Shared probe compatibility correction

The operation receipt must not widen every existing native result schema.
Independent integration review found closed raw-facts admission in Cline,
Gemini, Copilot and earlier Kimi lanes. The shared Go probe therefore reports
the executed operation only when the owned MCP fixture explicitly sets
`UAP_TEST_REPORT_OPERATION=1`. This new Kimi journey selects that receipt;
legacy, empty, disabled and malformed values keep the existing result fields.
The provider still requires the actual executed operation, with no relaxed
fact or process checks. No production behavior or other client's validator
changes are needed.

An explicit hosted Go test builds and executes the actual probe. It checks
legacy and opted-in write/read results, unchanged read-marker identity and
timestamps, and refusal of invalid operations and nonces without tool effects.
The previously successful hosted Kimi artifacts remain evidence for their
original source; the corrected final source still requires its natural hosted
Linux/macOS native run before acceptance.
