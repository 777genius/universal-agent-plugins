# Universal Agent Plugins site

Static Nuxt 3 frontend for the Universal Agent Plugins Directory.

Requirements: Node.js 22 or newer. The typed build adapter accepts the current
`../registry/index.json`, a deterministic review preview, or an exact published
signed snapshot without changing Vue components.

```bash
pnpm install --frozen-lockfile
pnpm lint
pnpm typecheck
pnpm test
pnpm test:a11y
NUXT_APP_BASE_URL=/universal-agent-plugins/ pnpm generate
NUXT_APP_BASE_URL=/universal-agent-plugins/ pnpm check:links
```

For site-only development and verification while the generated index is not
available, point Nuxt at the deliberately tiny fixture:

```bash
UAP_REGISTRY_PATH=tests/fixtures/registry.valid.json pnpm dev
```

Production publication passes `UAP_SIGNED_SNAPSHOT_PATH`. Pull-request review
deployments pass `UAP_DIRECTORY_PREVIEW_PATH`; the rendered site then carries a
prominent preview label and never presents unresolved data as production.

The site emits no analytics or tracking requests. See `NOTICE.md` and the icon
README files under `public/` for copied/adapted code and mark attribution.
