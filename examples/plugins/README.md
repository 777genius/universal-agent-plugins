# Production Plugin Examples

> **Historical plugin-kit-ai v1 managed examples; baseline 1.2.4.**
> Commands, stability labels, and support claims below describe the preserved v1
> workflow. These are not root `plugin.json` starters for the standard MVP.
> Project migration is not available in v2 yet. Maintain legacy projects using
> the v1 1.2.4 command set. See [historical context](../../website/source/en/legacy/v1/index.md)
> and the [prepared, unreleased Build guide](../../website/source/en/build/index.md).
> Public activation remains gated; no v2 npm availability is implied.

All six checked-in examples are v1 managed projects with `plugin/plugin.yaml`.
Their generated native outputs remain useful client-specific references:

| Examples | Native output / scope |
| --- | --- |
| `claude-basic-prod` | `.claude-plugin/plugin.json`, MCP configuration and hooks |
| `codex-basic-prod` | `.codex/config.toml`, launcher/notify runtime |
| `codex-package-prod` | `.codex-plugin/plugin.json`, Skills, MCP and app metadata |
| `cursor-basic` | `.cursor-plugin/plugin.json`, Skills and MCP |
| `gemini-extension-package` | `gemini-extension.json`, extension commands/settings/hooks |
| `opencode-basic` | `opencode.json`, workspace configuration and mirrored Skills |

Native packages/configuration are not root Agent Plugins 1.0 manifests.
Skills and MCP components may be reusable; hooks, app bindings, themes, and
client settings retain their client-specific boundaries. The external Context7
link below is a historical reference, not a locally verified standard package.

These examples are reference implementations for the historical v1 production plugin workflow.

- [`context7` in universal-plugins-for-ai-agents](https://github.com/777genius/universal-plugins-for-ai-agents/tree/main/plugins/context7): canonical multi-target MCP-first example with `plugin/` as the only authored root, package-only Claude, official Codex package output, Gemini extension packaging, and workspace-config output for OpenCode and Cursor
- [claude-basic-prod](./claude-basic-prod): Claude plugin repo with `plugin/plugin.yaml`, generated native artifacts, and deterministic local smoke path
- [codex-basic-prod](./codex-basic-prod): Codex runtime lane repo with `plugin/plugin.yaml`, generated `.codex/config.toml`, deterministic local notify smoke path, and repo-local MCP passthrough example
- [codex-package-prod](./codex-package-prod): official Codex package lane with `plugin/plugin.yaml`, generated `.codex-plugin/plugin.json`, optional `.app.json`, shared `.mcp.json`, and skills-first bundle output
- [gemini-extension-package](./gemini-extension-package): Gemini CLI extension repo with `plugin/plugin.yaml`, generated `gemini-extension.json`, shared MCP, and packaging-only validation coverage
- [cursor-basic](./cursor-basic): Cursor packaged plugin repo with `plugin/plugin.yaml`, portable `plugin/skills/**`, generated `.cursor-plugin/plugin.json`, and shared `.mcp.json`
- [opencode-basic](./opencode-basic): OpenCode workspace-config repo with `plugin/plugin.yaml`, generated `opencode.json`, shared MCP, and mirrored portable skills

Use them together with [../../docs/PRODUCTION.md](../../docs/PRODUCTION.md).
For copy-first Go/Python/Node starter repos, see [../starters/README.md](../starters/README.md).
For deeper repo-local Python/Node entrance references, including the checked-in helper-layer examples, see [../local/README.md](../local/README.md).

These reference repos document the historical v1 stable production path where Go is the recommended default because it yields the most self-contained plugin delivery story.
Canonical authoring uses `plugin/plugin.yaml`, `plugin/mcp/servers.yaml`, and `plugin/targets/<platform>/...`; committed native Claude/Codex/Gemini/Cursor/OpenCode files in the plugin root are generated managed artifacts.
Gemini, Cursor, and OpenCode remain packaging/config-only in this reference set. Gemini's Go hook lane is documented through the generated scaffold README, `plugin-kit-ai inspect`, `plugin-kit-ai capabilities --mode runtime --platform gemini`, the deterministic `make test-gemini-runtime` runtime gate, and the dedicated `make test-gemini-runtime-live` smoke path rather than a checked-in production example repo. Executable `python` and `node` plugins are stable supported repo-local local-runtime lanes and are covered through scaffold/runtime docs plus polyglot smoke tests rather than checked-in production example repos. Those interpreted lanes still require Python or Node to be installed on the machine running the plugin. Launcher-based `shell` authoring remains `public-beta`.
