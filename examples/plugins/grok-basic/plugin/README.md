# grok-basic

Reference Grok packaged-plugin example for `plugin-kit-ai`.

This example demonstrates the current packaged Grok subset:

- portable `plugin/skills/**` generated into root `skills/**`
- portable `plugin/mcp/servers.yaml` generated into wrapped `.mcp.json`
- generated `.grok-plugin/plugin.json` references the managed wrapped `.mcp.json`
- root `CLAUDE.md` and `AGENTS.md` remain boundary docs that point back to `plugin/`

Validate it with:

```bash
plugin-kit-ai generate --check .
plugin-kit-ai validate . --platform grok --strict
```
