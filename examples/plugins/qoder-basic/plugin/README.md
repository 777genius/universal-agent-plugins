# qoder-basic

Reference Qoder packaged-plugin example for `plugin-kit-ai`.

This example demonstrates the current packaged Qoder subset:

- portable `plugin/skills/**` generated into root `skills/**`
- portable `plugin/mcp/servers.yaml` generated into shared `.mcp.json`
- generated `.qoder-plugin/plugin.json` references the managed shared `.mcp.json`
- target-native hooks and commands under `plugin/targets/qoder/**` mirrored into root `hooks/**` and `commands/**`
- the Go SDK exposes typed `SessionStart`, `UserPromptSubmit`, `PostToolUse`, and `Stop` handlers through `sdk/qoder`; this example's hooks use the bundled shell runtime
- root `CLAUDE.md` and `AGENTS.md` remain boundary docs that point back to `plugin/`

Validate it with:

```bash
plugin-kit-ai generate --check .
plugin-kit-ai validate . --platform qoder --strict
```
