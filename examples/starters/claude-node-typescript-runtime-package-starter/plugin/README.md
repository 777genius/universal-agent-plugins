# claude-node-typescript-runtime-package-starter

Copy-first starter for Node/TypeScript teams that want the stable Claude default hook subset with built output under `dist/main.js` and a pinned shared `plugin-kit-ai-runtime` dependency.

## Who It Is For

- Teams wiring a local Claude plugin into an existing repo
- Node/TypeScript users who want the canonical `npm` starter path with a shared helper dependency
- Users who want the stable default hook subset instead of the extended hook surface

## Prerequisites

- `plugin-kit-ai` installed
- Node.js `20+` installed on the machine that will run the plugin
- `npm`
- Claude local plugin runtime lane

## Runtime

- Platform: `claude`
- Runtime: `node` with TypeScript
- Entrypoint: `./bin/claude-node-typescript-runtime-package-starter`
- Execution mode: `launcher`
- Status: `public-stable`, repo-local interpreted subset

## First Run

```bash
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
plugin-kit-ai validate . --platform claude --strict
```

This starter keeps one canonical Node story:

- `npm`
- `plugin/main.ts`
- `package.json` pinned to `plugin-kit-ai-runtime@1.1.0`
- `dist/main.js`

This starter begins on the shared package path directly instead of vendoring `plugin/plugin-runtime.ts`.

```ts
import { ClaudeApp, allow } from "plugin-kit-ai-runtime";
```

`plugin-kit-ai bootstrap .` runs `npm install` and `npm run build`.
If you prefer `pnpm`, `yarn`, or `bun`, keep using the stable runtime lane, but this starter stays opinionated on `npm`.
If you want downstream users to avoid installing Node at all, prefer the Go starter instead.

## Local Smoke

```bash
printf '%s' '{"session_id":"starter-session","transcript_path":"/tmp/t.jsonl","cwd":".","permission_mode":"default","hook_event_name":"Stop","stop_hook_active":false,"last_assistant_message":"ok"}' | ./bin/claude-node-typescript-runtime-package-starter Stop
```

## Stable Default

- `Stop`
- `PreToolUse`
- `UserPromptSubmit`

The scaffold wires only the public-stable Claude hook subset by default.
Treat `plugin-kit-ai validate --strict` as the CI-grade readiness gate for this plugin.
Use `plugin-kit-ai init <name> --platform claude --runtime node --claude-extended-hooks` only when you intentionally want the full runtime-supported hook set scaffolded.

## Target Files

- `plugin/targets/claude/hooks/hooks.json`: authored Claude hook routing
- `hooks/hooks.json`: generated managed Claude hook file
- `package.json`: pinned shared-helper dependency manifest
- `plugin/main.ts`: runtime entry importing `plugin-kit-ai-runtime`
- Optional first-class Claude breadth via `--extras`:
  - `plugin/targets/claude/settings.json` -> generated `settings.json`
  - `plugin/targets/claude/lsp.json` -> generated `.lsp.json`
  - `plugin/targets/claude/user-config.json` -> generated `plugin.json.userConfig`
  - `plugin/targets/claude/manifest.extra.json` -> manifest passthrough for non-managed keys only

Keep stdout reserved for Claude responses and write diagnostics to stderr only.

## Ship It

This starter already includes `.github/workflows/bundle-release.yml`.

```bash
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
plugin-kit-ai validate . --platform claude --strict
plugin-kit-ai bundle publish . --platform claude --repo owner/repo --tag v1.0.0
plugin-kit-ai bundle fetch owner/repo --tag v1.0.0 --platform claude --runtime node --dest ./handoff-plugin
```
