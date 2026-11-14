# cursor-basic

Reference Cursor packaged-plugin example for `plugin-kit-ai`.

This example demonstrates the current packaged Cursor subset:

- portable `plugin/skills/**` generated into root `skills/**`
- portable `plugin/mcp/servers.yaml` generated into shared `.mcp.json`
- generated `.cursor-plugin/plugin.json` references the managed shared `.mcp.json`
- root `CLAUDE.md` and `AGENTS.md` remain boundary docs that point back to `plugin/`

Validate it with:

```bash
plugin-kit-ai generate --check .
plugin-kit-ai validate . --platform cursor --strict
```
