# codex-node-typescript-starter

Copy-first starter for Node/TypeScript teams that want the stable `codex-runtime` Notify path with built output under `dist/main.js`.

## Who It Is For

- Teams wiring a local Codex plugin into an existing repo
- Node/TypeScript users who want the canonical `npm` starter path
- Users who want the stable interpreted subset, not the more self-contained Go production lane

## Prerequisites

- `plugin-kit-ai` installed
- Node.js `20+` installed on the machine that will run the plugin
- `npm`
- Codex local runtime lane

## Runtime

- Platform: `codex-runtime`
- Runtime: `node` with TypeScript
- Entrypoint: `./bin/codex-node-typescript-starter`
- Execution mode: `launcher`
- Status: `public-stable`, repo-local interpreted subset

## First Run

```bash
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
plugin-kit-ai validate . --platform codex-runtime --strict
```

This starter keeps one canonical Node story:

- `npm`
- `plugin/main.ts`
- `plugin/plugin-runtime.ts`
- `dist/main.js`

The local helper file mirrors the shared `plugin-kit-ai-runtime` package when you later want to move this API into a reusable dependency.

`plugin-kit-ai bootstrap .` runs `npm install` and `npm run build`.
If you prefer `pnpm`, `yarn`, or `bun`, keep using the stable runtime lane, but this starter stays opinionated on `npm`.
If you want downstream users to avoid installing Node at all, prefer the Go starter instead.

## Local Smoke

```bash
./bin/codex-node-typescript-starter notify '{"client":"codex-tui"}'
```

## Stable Default

- `Notify`

Treat `plugin-kit-ai validate --strict` as the CI-grade readiness gate for this runtime lane.
This starter is for repo-local integration, not the official packaged Codex bundle lane.

## Target Files

- `plugin/launcher.yaml`: runtime and entrypoint for local Notify integration
- `plugin/targets/codex-runtime/package.yaml`: authored Codex runtime metadata
- `.codex/config.toml`: generated managed Codex config
- `plugin/plugin-runtime.ts`: official helper API for `onNotify`

## Ship It

This starter already includes `.github/workflows/bundle-release.yml`.

```bash
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
plugin-kit-ai validate . --platform codex-runtime --strict
plugin-kit-ai bundle publish . --platform codex-runtime --repo owner/repo --tag v1.0.0
plugin-kit-ai bundle fetch owner/repo --tag v1.0.0 --platform codex-runtime --runtime node --dest ./handoff-plugin
```
