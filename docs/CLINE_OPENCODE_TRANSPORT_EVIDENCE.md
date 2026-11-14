# Cline cwd and OpenCode declared transport

This records adapter behavior and source evidence, separately from native runtime
verification. Client discovery checks presence, not version; no minimum version
is inferred from these release snapshots.

- Cline VS Code extension v4.1.17 accepts nested `transport.cwd` in
  `apps/vscode/src/services/mcp/schemas.ts` and passes it to the stdio transport in
  `McpHub.ts`. Cline CLI cli-v3.0.61 passes cwd to spawn in
  `sdk/packages/core/src/extensions/mcp/client.ts`. UAP retains its nested shape,
  including normalized default `${PLUGIN_ROOT}` cwd and explicit cwd. These are
  source-inspected surfaces, not claims that native execution was tested here.
- OpenCode v1.18.29 has no native remote transport selector in
  `packages/core/src/v1/config/mcp.ts`. Its `connectRemote` in
  `packages/opencode/src/mcp/index.ts` starts with Streamable HTTP. Consequently
  portable Streamable HTTP remains supported and declared SSE is unsupported for
  this adapter. A fallback to SSE does not preserve an SSE-first declaration.
  Other clients retain their independent transport capabilities. URLs and headers
  are copied literally; UAP adds no invented native selector.

Portable SSE is still valid Agent Plugins 1.0.0 input. A client capability refusal
is not a package validation error. Component selection must omit unsupported SSE
from OpenCode native configuration and receipts while preserving usable siblings.

Local tests cover projection, native receipt updates from historical no-cwd Cline
entries, cwd ownership drift, and receipt-based removal. They use temporary paths
and do not start Cline or OpenCode. No native runtime, activation, OAuth, or release
verification is claimed by these tests.

Historical Cline artifacts use their recorded digest. Adding cwd changes the
projection digest: exact repair must refuse a historical rebuild mismatch without
changing state or native configuration. A controlled update is the migration
path. Historical intact receipts remain usable for removal. Reverting the renderer
must not discard receipts or erase cwd fields through unowned cleanup.

Sources: [Cline extension](https://github.com/cline/cline/tree/v4.1.17),
[Cline CLI](https://github.com/cline/cline/tree/cli-v3.0.61),
[OpenCode](https://github.com/anomalyco/opencode/tree/v1.18.29),
[Agent Plugins 1.0.0 section 7.2.1](https://github.com/agentplugins/agent-plugins-spec/blob/ff8ab5e392cc87bd88d87c060815a87490e51003/spec/1.0.0.md#721-remote-transport).
