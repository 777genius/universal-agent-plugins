# Codex Target Boundary

Use this guide when choosing between `codex-runtime`, `codex-package`, or both in the same repo.

## Summary

- `codex-runtime` is the repo-local notify/runtime lane
- `codex-package` is the official Codex plugin bundle lane
- they are separate on purpose and should not be treated as interchangeable

## Choose `codex-runtime` When

- your plugin runs from the repo through `plugin/launcher.yaml`
- you need the stable `Notify` runtime path
- your source of truth is repo-local runtime wiring plus authored metadata under `plugin/targets/codex-runtime/...`
- you want managed `.codex/config.toml`

Canonical authored inputs:

- `plugin/plugin.yaml`
- `plugin/launcher.yaml`
- `plugin/targets/codex-runtime/package.yaml`
- optional `plugin/targets/codex-runtime/config.extra.toml`

Managed output:

- `.codex/config.toml`

## Choose `codex-package` When

- you are building the official Codex plugin bundle
- you need packaged metadata, interface UX, optional app assets, or portable MCP wiring
- there is no repo-local launcher/runtime contract in this lane

Canonical authored inputs:

- `plugin/plugin.yaml`
- optional `plugin/mcp/servers.yaml`
- `plugin/targets/codex-package/package.yaml`
- optional `plugin/targets/codex-package/interface.json`
- optional `plugin/targets/codex-package/app.json`
- optional `plugin/targets/codex-package/manifest.extra.json`

Managed outputs:

- `.codex-plugin/plugin.json`
- optional `.app.json`
- optional `.mcp.json`

Bundle layout rules:

- `.codex-plugin/` must contain only `plugin.json`
- `.app.json` must exist only when `.codex-plugin/plugin.json` references `./.app.json`
- `.mcp.json` must exist only when `.codex-plugin/plugin.json` references `./.mcp.json`
- do not move sidecars under `.codex-plugin/`; Codex expects them at the plugin root

## Use Both When

- the repo needs both a repo-local runtime plugin and an official Codex package
- the runtime lane and package lane must evolve together from the same authored repo

In that case, keep each lane in its own target subtree and validate them separately:

```bash
plugin-kit-ai validate . --platform codex-runtime --strict
plugin-kit-ai validate . --platform codex-package --strict
```

## Import Guidance

- use `plugin-kit-ai import --from codex-runtime` when you only have repo-local `.codex/config.toml`
- use `plugin-kit-ai import --from codex-package` when you only have `.codex-plugin/plugin.json` and optional `.app.json` / `.mcp.json`
- run the `codex-runtime` and `codex-package` imports separately when a repo contains both native lanes

## Anti-Patterns

- do not use `codex-runtime` as a substitute for the official package bundle
- do not put runtime-only assumptions into `codex-package`
- do not treat `config.extra.toml` as package metadata
- do not treat `manifest.extra.json` as a place for canonical fields already covered by `package.yaml` or `interface.json`
