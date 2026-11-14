# Codex MCP transport evidence

UAP treats declared MCP SSE as unsupported for Codex. Existing stdio and
Streamable HTTP adapter support is unchanged. This is a client delivery decision,
not an additional portable package validation rule.

## Source evidence

The reviewed Codex source is `rust-v0.153.4`, commit
`3d2ee51ca2d5db578f328aa75e20aa22c0197c9a`, checked on 2026-09-07:

- The [native Agent Plugins parser](https://github.com/openai/codex/blob/3d2ee51ca2d5db578f328aa75e20aa22c0197c9a/codex-rs/codex-mcp/src/agent_plugin_config.rs)
  explicitly rejects declared SSE.
- The [legacy plugin configuration](https://github.com/openai/codex/blob/3d2ee51ca2d5db578f328aa75e20aa22c0197c9a/codex-rs/codex-mcp/src/plugin_config.rs)
  normalizes transport fields; its URL route uses
  [Streamable HTTP](https://github.com/openai/codex/blob/3d2ee51ca2d5db578f328aa75e20aa22c0197c9a/codex-rs/config/src/mcp_types.rs).
  This does not establish SSE-first behavior.
- [Manifest selection](https://github.com/openai/codex/blob/3d2ee51ca2d5db578f328aa75e20aa22c0197c9a/codex-rs/utils/plugins/src/plugin_namespace.rs)
  prefers root `plugin.json` over the compatibility manifest. UAP preserves the
  standard manifest and selects delivered servers in both `mcp.json` and
  `.mcp.json`.

This source snapshot does not establish a minimum supported Codex version or
prove a native handshake, tool call, skill execution, cache refresh, or data
persistence. Adapter tests and native runtime evidence remain separate.

## Delivery and existing installations

Healthy skills, stdio servers, and Streamable HTTP servers remain eligible when
SSE is excluded. An SSE-only delivery refuses without installation writes.
Direct local and immutable Git installation remain independent of the Directory.

For an existing installation that recorded an SSE component, same-source Add and
exact historical Repair refuse rather than silently dropping that component.
A confirmed Update may withdraw the now-unsupported SSE component when healthy
delivery remains. An all-unsupported Update refuses safely. A temporarily
unavailable stdio executable does not authorize destructive withdrawal.
