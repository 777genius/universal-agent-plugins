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
