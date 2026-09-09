Disposable real Nuxt 3 / i18n 9 candidate fixture; all three codes load the existing
English reference dictionary. Nothing here participates in production routes or
publication. `prepare.mjs` copies the current switcher, adapter, preference store
and helpers byte-for-byte into a fresh temporary app, substitutes only the locale
metadata module, and links the already installed landing dependencies. It does
not install packages or change the production config or dictionaries.

From `landing`, on the hosting runner only (requires its installed browser):

```sh
candidate_dir=$(node tests/fixtures/i18n-candidate/prepare.mjs)
UAP_CANDIDATE_BASE=/ node "$candidate_dir/build.mjs"
UAP_CANDIDATE_BASE=/ node node_modules/@playwright/test/cli.js test --config "$candidate_dir/playwright.config.ts"
UAP_CANDIDATE_BASE=/universal-agent-plugins/ node "$candidate_dir/build.mjs"
UAP_CANDIDATE_BASE=/universal-agent-plugins/ node node_modules/@playwright/test/cli.js test --config "$candidate_dir/playwright.config.ts"
```

Build once per base, then serve the isolated Nitro production SSR output; no
Vite dev compilation occurs during requests. Playwright refuses a missing or
mismatched build marker, starts a fresh server, and runs one worker with no
retries. Test and readiness timeouts remain 30s and 60s. Preparation only copies
files and links installed dependencies; do not build, serve or launch browsers
in the provider sandbox.

The installed nuxt-icon component awaits `loadIcon` in SSR setup, which otherwise
makes even the first document depend on the public Iconify API. A fixture-only
plugin seeds its icon state with three decorative local glyphs before rendering.
The copied switcher, real Icon component, Vuetify and i18n remain in use. These
glyphs are not flag artwork validation. This removes an observed source-level
SSR network dependency; it does not claim to prove the cause of the dev hang.

Real lazy locale factories make a test-only request which Playwright rejects with 503
(no fetch retries) or holds pending. i18n's own load wrapper swallows that
rejection; the production adapter must detect its empty direct dictionary before
navigation. No mocked `navigateTo`, locale assignment or `setLocale` is used.
The fixture also exercises Nuxt `abortNavigation`, completed Back/Forward,
query arrays/hash, cookie base, keyboard/current choice/Escape/focus, and pending
controls. A global fixture route guard aborts before the locale middleware.
Production static artifact refresh/payload checks remain in `tests/browser`.

The rejection test also observes i18n 9's own `Failed locale loading:` console
error and a new factory request on explicit retry. Installed i18n 9.5.6 catches
factory errors and supplies an empty dictionary; its production cache only stores
non-function dictionaries, so the async candidate factories remain retryable.
Tab from the final menu item exits to the next control, Abort next navigation;
Escape returns focus to the activator. All five scenarios remain required at
both bases. Hosted build/browser results are required before calling this
harness runtime-verified.
