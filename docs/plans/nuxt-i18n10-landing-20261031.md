# Nuxt i18n 9 → 10 landing migration (2026-10-31)

Issue: [#613](https://github.com/777genius/universal-agent-plugins/issues/613). The isolated Dependabot update [#574](https://github.com/777genius/universal-agent-plugins/pull/574) is **not** mergeable: Nuxt i18n v9's transitive `@intlify/shared@10` conflicts with Vue I18n v11.4.14. This plan deliberately excludes dependency rollup [#612](https://github.com/777genius/universal-agent-plugins/pull/612).

## Decision and boundaries

1. Migrate `@nuxtjs/i18n` from `^9.5.6` to the verified stable v10 line (`10.6.0` at the time of planning), **together with** `vue-i18n@11.4.14`. Use a real pnpm **12.3.4** lockfile regeneration; do not hand-edit registry resolutions, integrity hashes, override all `@intlify` packages, or bypass frozen-install checks.
2. Follow the [official 9→10 guide](https://i18n.nuxtjs.org/docs/guide/migrating): remove `restructureDir: false`, `lazy`, and `bundle.optimizeTranslationDirective`. Keep `strategy: 'prefix_except_default'`, `defaultLocale: 'en'`, `trailingSlash: true`, and `detectBrowserLanguage: false` as published URL contracts. In v10 locale files load lazily by default.
3. Move only the **nine top-level translation dictionaries** from `landing/locales/*.json` to `landing/i18n/locales/*.json` and the Vue I18n configuration from `landing/i18n.config.ts` to `landing/i18n/i18n.config.ts`. Default `restructureDir: 'i18n'` resolves `langDir: 'locales'` and `vueI18n: './i18n.config.ts'` without path traversal. Leave feature-owned `features/*/locales` and content overlays in place.
4. Update each repository consumer of those moved dictionaries, fixture and tests; retain the same JSON bytes, route list, slavic plural rules and user-facing translated text. For locale metadata, use `language` (Vue I18n/Nuxt i18n v10 option) and update SEO projections if the existing `iso` internal field is renamed.
5. Update the disposable real Nuxt candidate fixture for v10 semantics and preserve its failure/retry/route checks. This is not a production feature or flag.

## Verification / no-merge gates

- Lock acquisition must run on an actual npm-connected, valid Node 22.21.1 / pnpm 12.3.4 environment and produce **real** `landing/pnpm-lock.yaml`. Generate using `pnpm install --lockfile-only --ignore-scripts`, then verify `pnpm install --frozen-lockfile` (normal dependency lifecycle policy) without mixing lockfile versions. Before any committed lockfile, a temporary, branch-restricted GitHub Action may upload the generated file as an artifact for byte-exact retrieval and a connector commit. **Never print credentials or publish a writable PR workflow**. Remove the temporary lock-generation workflow before ready-for-review.
- Verify the resolved dependency graph contains no incompatible split-major `vue-i18n`/`@intlify` imports; run `pnpm exec nuxt prepare`, `pnpm exec vue-tsc --noEmit`, `pnpm run lint`, `pnpm run test:registry`, `pnpm run check:i18n`, `pnpm run generate`. Keep the signed registry mirror verification (no mocked or unsigned feeds).
- Run all existing browser tests including desktop/mobile and 9-language routing/SEO/head, no extra skipped cases; exercise both deployment base paths `/` and `/universal-agent-plugins/` via the combined Pages artifact checks. Real SkillSpector / signed registry smoke must still work; do not weaken any security gates.
- Verify explicit legacy/public page path consistency, canonical/hreflang, Slavic plurals, directionality (Arabic), locale switch and pending navigation, return after refresh, and policy on no browser autodetection.
- Independent security review and code review on **exact latest SHA**; fix every actionable finding; require all CI (including `landing-test` and both `combined-pages-validation` matrix cells) green, unblocked by queues or stale passing runs. Only then squash-merge the migration PR. Close #574 with a linked explanation after a successful merge; no release in this task.

## Self-review / failure risks

- **Lock correctness**: cannot resolve a new major version by editing YAML importers manually. An unchanged or manually reconstructed lock is a hard stop, even if source looks correct.
- **Root restructuring**: failure to move files and every direct test import will silently turn translations into English, break SSR, or fail unit tests. Preserve original blob hashes for all nine dictionaries, then audit references and static output.
- **Route/SEO drift**: default-browser redirects or changed trailing slashes can harm indexed pages; preserve existing options and compare both base URLs.
- **Security boundary**: existing signed input is mandatory. Never use `pull_request_target` for this branch, never grant token write permissions to run arbitrary PR scripts.
- **Scope**: do not reformat unrelated landing modules, mutate #612, force a Dependabot merge, or claim a passing browser E2E from unexecuted tests.

## Status

Research and scoped change preparation started. **Not production-ready or merged until all gates above have evidence.**
