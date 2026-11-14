# codex-package-prod

Reference Codex package repo for the official `codex-package` lane.

This example covers the official Codex plugin bundle only. It does not include `plugin/launcher.yaml` or repo-local notify integration.
It demonstrates first-class package metadata in `plugin/targets/codex-package/package.yaml`, structured prompt UX in `plugin/targets/codex-package/interface.json`, an optional app manifest in `plugin/targets/codex-package/app.json`, and shared MCP wiring from `plugin/mcp/servers.yaml`.

Included MCP servers:

- `linear` (remote, `https://mcp.linear.app/mcp`)
- `supabase` (remote, `https://mcp.supabase.com/mcp`)
- `playwright` (stdio, `npx @playwright/mcp@0.0.70`)

Runtime auth is configured via `LINEAR_API_KEY` and `SUPABASE_ACCESS_TOKEN`/`SUPABASE_PROJECT_REF` placeholders. If unset, auth is deferred to user OAuth flow or the MCP server may report a disabled state.

## Workflow

```bash
plugin-kit-ai normalize .
plugin-kit-ai generate .
plugin-kit-ai generate --check .
plugin-kit-ai validate . --platform codex-package --strict
```

Use [../../../docs/CODEX_TARGET_BOUNDARY.md](../../../docs/CODEX_TARGET_BOUNDARY.md) if you need to decide between this package lane and the repo-local `codex-runtime` lane.
