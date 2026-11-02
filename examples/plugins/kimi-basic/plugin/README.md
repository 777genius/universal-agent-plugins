# kimi-basic

Reference Kimi Code packaged-plugin example for `plugin-kit-ai`.

This example demonstrates the current packaged Kimi subset:

- portable `plugin/skills/**` generated into root `skills/**`
- portable `plugin/mcp/servers.yaml` inlined into `.kimi-plugin/plugin.json#mcpServers`
- Kimi does not resolve MCP file refs, so no `.mcp.json` sidecar is generated
- root `CLAUDE.md` and `AGENTS.md` remain boundary docs that point back to `plugin/`

Validate it with:

```bash
plugin-kit-ai generate --check .
plugin-kit-ai validate . --platform kimi --strict
```
