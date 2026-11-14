# opencode-basic

Reference OpenCode workspace-config example for `plugin-kit-ai`.

This example demonstrates the current finished OpenCode workspace/config lane:

- `plugin/targets/opencode/package.yaml` for `opencode.json.plugin`
- `plugin/targets/opencode/commands/` for `.opencode/commands/`
- `plugin/targets/opencode/agents/` for `.opencode/agents/`
- `plugin/targets/opencode/themes/` for `.opencode/themes/`
- `plugin/targets/opencode/tools/` for first-class beta `.opencode/tools/`
- `plugin/targets/opencode/plugins/` for `.opencode/plugins/`
- `plugin/targets/opencode/package.json` for shared `.opencode/package.json` dependency metadata used by both standalone tools and plugin code
- portable `plugin/mcp/servers.yaml` for `opencode.json.mcp`
- portable `skills/` validated against the shared `SKILL.md` contract and mirrored into `.opencode/skills/`
- native import support for `opencode.json`, `opencode.jsonc`, project workspace directories, local plugin code/package metadata, and explicit `--include-user-scope`

Plugin specifics in this example:

- `plugin/targets/opencode/plugins/example.js` uses the canonical official-style named async plugin export and doubles as the loader smoke fixture
- `plugin/targets/opencode/tools/echo.ts` shows first-class beta standalone tool authoring using `@opencode-ai/plugin`
- `plugin/targets/opencode/plugins/custom-tool.js` shows beta custom-tool support through plugin code using the same shared helper dependency
- `plugin/targets/opencode/package.json` is the canonical authored source for shared tool/plugin dependencies
- `make test-opencode-tools-live` is the dedicated opt-in live evidence path for standalone tools; `make test-opencode-live` remains the stable local-plugin-loading evidence path

Validate it with:

```bash
plugin-kit-ai generate --check .
plugin-kit-ai validate . --platform opencode --strict
```
