# Cursor Agent: exact project MCP tool permissions

This is an opt-in, publisher-declared **Cursor Agent CLI** project permission,
not Cursor Editor workspace trust or global bypass. Project Skill installation
and selected MCP-server fingerprint approval already exist independently.

Add this exact extension to the package manifest:

~~~json
{
  "extensions": {
    "com.universal-agent-plugins.cursor-mcp-approval": {
      "version": 1,
      "mcp_auto_approve": {
        "selected-server": ["inspect_runtime"]
      }
    }
  }
}
~~~

Select that MCP server and this extension with a project installation.
During an explicit, confirmed add, UAP shows every exact
"Mcp(selected-server:inspect_runtime)" rule. It projects this list into the
owned package, verifies the native server receipt, existing
disablement and file authority, then conditionally adds only these named
rules to the selected project's ".cursor/cli.json"
"permissions.allow". Existing "permissions.deny" and "permissions.ask" are
preserved; a matching or broad deny/ask is a refusal, not automatically
removed. No force/yolo, generic wildcard MCP approval, or Editor
"mcpAllowlist" mutation is performed.

The Cursor Agent CLI's named tool permissions are not tied to an immutable MCP
server fingerprint. **They persist after removing the plugin**, as native
project trust does for other clients. Revoke them with the Cursor CLI/project
permissions editor when no longer wanted. Later update, repair, dependency
sync and read-only verification do not regrant a revoked permission. Only
another explicit, confirmed installation can authorize it again.

The server fingerprint approval is proven for exactly two Cursor Agent
versions: `2026.09.18-9a7762b` (original pinned native spike) and
`2026.10.22-27dcf5f` (independently audited full genuine native MCP
lifecycle in [PR #521](https://github.com/777genius/universal-agent-plugins/pull/521),
[run 41046185239](https://github.com/777genius/universal-agent-plugins/actions/runs/41046185239)).
The latter tested the publisher-pinned CLI and exact approval-store
fingerprints across A/B installations and server version updates, not an LLM
tool call. UAP never calls `mcp enable` itself because that command can
override user disablement. The current E2E requires native `mcp list`
to report `ready` **without** running that manual command.

The native server's independent fingerprint approval is still version-gated;
unsupported client versions, login requirements, Editor admin policy and
unknown project trust do not become Active by installing permission files.
The CLI project must be the actual root Cursor Agent loads, not a nested
folder under a different Git repository. After installation the user must
still authorize Cursor itself if its separate workspace/account policy
requires it.

The extension accepts only v1, 1–32 selected servers and 1–64 exact tool
names per server. Names are visible ASCII letters, digits, underscore, dash
or dot. Invalid/duplicate JSON keys, wildcard/glob names, unselected servers,
foreign native configuration and malformed permissions are refused.

## Evidence and boundaries

Focused tests cover extension admission, selected-server membership, managed
projection, byte-preserving unchanged repeat, foreign policy preservation,
explicit deny/ask, consent vs repair/verify-only, and owned-server drift.
Genuine Cursor Editor activation, signed-in model/tool execution and
workspace trust are separate qualification gates. No test that only reads
files may claim those native runtime outcomes.
