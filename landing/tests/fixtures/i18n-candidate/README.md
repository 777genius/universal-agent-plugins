Disposable real Nuxt 3 / i18n 9 candidate fixture; all three codes load the existing
English reference dictionary. Nothing here participates in production routes or
publication. `prepare.mjs` copies the current switcher, adapter, preference store
and helpers byte-for-byte into a fresh temporary app, substitutes only the locale
metadata module, and links the already installed landing dependencies. It does
not install packages or change the production config or dictionaries.

From `landing`, orchestrator commands (run each base separately):

```sh
candidate_dir=$(node tests/fixtures/i18n-candidate/prepare.mjs)
UAP_CANDIDATE_BASE=/ node node_modules/@playwright/test/cli.js test --config "$candidate_dir/playwright.config.ts"
UAP_CANDIDATE_BASE=/universal-agent-plugins/ node node_modules/@playwright/test/cli.js test --config "$candidate_dir/playwright.config.ts"
```

Playwright starts the real Nuxt dev server with the installed binaries. Real
lazy locale factories make a test-only request which Playwright rejects with 503
(no fetch retries) or holds pending. i18n's own load wrapper swallows that
rejection; the production adapter must detect its empty direct dictionary before
navigation. No mocked `navigateTo`, locale assignment or `setLocale` is used.
The fixture also exercises Nuxt `abortNavigation`, completed Back/Forward,
query arrays/hash, cookie base, keyboard/current choice/Escape/focus, and pending
controls. A global fixture route guard aborts before the locale middleware.
Production static artifact refresh/payload checks remain in `tests/browser`.
