# Cursor and OpenCode Parity Audit

Checked on `2026-04-08`.

## Cursor

- Supported by `plugin-kit-ai`: repo-local `.cursor/mcp.json` generation, project-root `.cursor/rules/**`, authored `plugin/targets/cursor-workspace/AGENTS.md` merged into generated root `AGENTS.md`, and `--include-user-scope` import from `~/.cursor/mcp.json`
- Intentionally out of scope: nested non-root `.cursor/rules/**`, GUI-only/global rule authoring, JSONC guarantees for `.cursor/mcp.json`, and VS Code extension packaging
- Official references:
  - [Cursor Rules](https://cursor.com/docs/context/rules)
  - [Cursor MCP](https://cursor.com/docs/context/mcp)

## OpenCode

- Supported by `plugin-kit-ai`: `plugin` refs including tuple-form `[name, options]`, shared `mcp`, skills, commands, agents, `default_agent`, `instructions`, `permission`, themes, stable local plugin code plus shared `package.json`, beta standalone tools, JSON/JSONC import, and explicit `--include-user-scope` import from `~/.config/opencode`
- Sanctioned passthrough: `plugin/targets/opencode/config.extra.json` remains the fallback for wider product config such as `server`, `watcher`, `snapshot`, provider toggles, and other non-first-class settings
- Deprecated import aliases preserved:
  - top-level `mode` imports as `agent`
  - agent-level `maxSteps` imports as `steps`
  - agent-level `tools` imports without being re-generated in deprecated shape
- Official references:
  - [OpenCode Config](https://opencode.ai/docs/config/)
  - [OpenCode Plugins](https://opencode.ai/docs/plugins/)
  - [OpenCode Schema](https://opencode.ai/config.json)
