# P1 — Two missing genuine native MCP calls: Cursor and Qoder

## Baseline and boundaries
Current project evidence matrix already qualifies genuine `tools/call` for Claude, Codex, OpenCode V2, Gemini, Copilot, Kimi, Cline and Pi under their **named** scripted/no-provider-auth conditions. Do not rerun or reimplement their tests. Cursor currently has native `initialize` and `tools/list`, **not** `tools/call`. Qoder has native stdio/HTTP configuration/list observation, **not** `tools/call`.

## Scoped implementation
1. Reuse `scripts/project_native_runtime.py`, existing per-client collectors, pinned binaries and project A/B disposable roots. First identify actual native-client command/trust support and observable tool invocation; no fabricated protocol frames.
2. Cursor: selected trusted project, genuine editor/CLI process, independently anchored local stdio MCP `initialize`/`tools/list`/`tools/call` with per-run nonce receipt, foreign configuration and A/B isolation. If native editor/headless path cannot issue `tools/call`, preserve a truthful negative status and evidence instead of green.
3. Qoder: selected genuine Qoder session/project and owned stdio/HTTP config, pinned native binary, actual client-driven tool call, exact provenance/nonce, project isolation, remove/restart non-discovery, no cached foreign import mistaken for current UAP selection. No direct Python MCP simulator counted as native call.
4. Add narrow negative tests for denied trust/auth, stale source, foreign ownership, unsupported transports, cancellation/cleanup. Keep bounded OS/runtime and no vendor credentials.
5. CI exact HEAD: both named native routes must PASS on qualifying OSs before support matrix claims. Review and merge only proven routes; document incomplete platform/transport honestly.

## Linked existing independent work (do not duplicate)
VS Code trusted Project remains [draft #530](https://github.com/777genius/universal-agent-plugins/pull/530), genuine hosted trust/UID/cgroup gate. Kiro auth refusal remains [draft #686](https://github.com/777genius/universal-agent-plugins/pull/686), requires legitimate disposable native authentication. No fake E2E bypass. Separate approvals/evidence apply.


## Observed collector contract and native execution seam (2026-11-08)

This is an implementation constraint, **not** an E2E success claim. Read the real pinned repository collectors at the exact Draft HEAD:

- `scripts/project_native_cursor_cycle.py` line 21 explicitly limits `SCOPE` to `initialize` / `tools-list` and says **"no tools-call"**. Its `admit` branch requires `mcp-tools-call=false`, even if other flags are true. The existing frozen Cursor cycle must continue to pass unchanged; add an independently evidence-bound **new native-call lane**, not an edit which silently upgrades old evidence.
- `scripts/project_native_qoder_cycle.py` line 252 builds a Skill resource list with `mcpServers=[]`; this genuine Qoder *Skills* lifecycle is not an MCP `tools/call` probe. Keep the existing finished Skills evidence intact and add a separate selected-client MCP proof, never reinterpret the prior Skills collector.
- Cursor official CLI docs: `agent mcp list-tools <identifier>` proves **only advertised tools**; the next authenticated **agent session** must be started in project A, e.g. `agent -p` with a request that requires an exact nonce/operation, and the project-owned MCP server must independently record the real `tools/call`. Ref: https://prod.cursor.com/docs/cli/mcp. A CLI listing or an assistant's text claiming a call is insufficient; no broad `--approve-mcps` shortcut and no unattended approval of arbitrary servers.
- Qoder official CLI: select the one disposable server with `--allowed-mcp-server-names`; `--strict-mcp-config` plus a one-off explicitly supplied `--mcp-config` can isolate the process from user/global MCP configurations, but ensure this tests a server actually installed by the selected UAP project rather than a replacement injected by the harness. Require a genuine Qoder session and per-run nonce receipt; Qoder's `mcp_list/mcp_get/mcp_call` lazy helpers only qualify when they were invoked by that client. Refs: https://docs.qoder.com/cli/mcp-reference and https://docs.qoder.com/cli/cli-reference.
- Both vendors require the applicable *native agent login/provider authorization* to run a model turn. Detect inability to authorize and retain `mcp-tools-call=false` with a reason; do **not** fabricate a native call from a direct JSON-RPC Python test, grant blanket Project MCP trust, switch to another client's credentials, or label an install/list-only run complete.
- Independent observable proof must bind `{client, native_binary_pin, project_root, selected_server, tool_name, nonce, request_args, response_nonce, timestamp, run_id}` to a server-side receipt generated during the native client session. Test before/after removal, Project B and outside-directory negatives; fail on stale receipt, wrong nonce or server, unexpected file/profile mutation. Strip tokens, prompts and vendor responses from published artifacts. No new migration or user-facing feature flags.
