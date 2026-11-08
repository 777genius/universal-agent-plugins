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
