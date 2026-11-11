# Catalog release presentation, responsive filters, and Pi Project installs

Date: 2026-11-11. Base: `784ade332a75c58e9849204f3ca9164a8bdd7827`. Owner goal: one reviewed pull request through merge and production E2E, without replacing signed Directory admission.

## Current evidence and defects

1. `main` already references the new white, rounded Hermes `hermes.webp`; `website/production-source.json` is pinned to an earlier commit (`812cbfc0`). A working build of `main` does not prove that the production Pages site contains the WebP. Also review the already-open browser-test adjustment #865; do not duplicate unrelated work or claim an old fixture passes.
2. `landing/assets/styles/registry.scss` forces **two** columns at viewport 981–1520 px and only separates the source/owner pair at ≥1521 px. At desktop widths near the supplied screenshot, the eight filters should have **three** columns; two belong to narrower widths, one to mobile. Preserve compact search, sort, keyboard, RTL, and focus behavior.
3. Pi supports eligible native User/Project extensions, Skills and stdio/Streamable HTTP MCP in the CLI, but a signed Directory package (e.g. Sentry) may not grant Pi as a `target`. The picker correctly refuses to invent a signed target. Owner wants Pi to be usable across all **technically compatible** catalog packages where a verifiable install route exists. Not every package is component/transport compatible; do not claim otherwise.
4. Hermes/Antigravity picker availability is governed by actual active signed release and published CLI capability, not by logos or whether the adapter exists in source. **Do not bypass release gates**. Antigravity IDE activation E2E remains deferred as #864, still `Prepared`.

## Minimal implementation

### A. Responsive filter grid and visual tests
- Modify only `landing/assets/styles/registry.scss`: five columns on wide desktop, three on medium desktop (search spans the row; sort and filters flow normally), two on small desktop/tablet, one on narrow mobile; make source and owner separate grid items where three columns are available.
- Add a real-browser assertion at 1024 and 1280 CSS px (three columns), 900 and 768 px (two), and 390 px (one), including Retina-width screenshots. Check actual computed grid columns, search width, no horizontal overflow, both source/owner controls, with English and one RTL viewport.

### B. Pi explicit pinned-source Project route (no signature bypass)
- Keep `resolveDistribution`, signed evidence decoding, v1/v2 grants, release eligibility, package identity, and actual CLI unchanged.
- Add a narrowly scoped, explicit **Project-only Pi alternate** for a reviewed, currently installable catalog package whose active signed release identifies a valid immutable GitHub repository + full 40-char revision + safe package path, **all** advertised package components fit Pi's generated Project capability matrix, and no signed Pi selection exists. Reuse existing `installCommands` to generate `github:OWNER/REPO@FULL_SHA//PACKAGE_PATH --target pi --scope project`.
- The exact Git selector is an *independently consented direct source*, **not** a new signed Pi grant. Present it as such in both catalog card and detail picker, and require explicit Pi selection; no automatic Pi inference, no mixed signed/direct multi-target install, no silent fallback on feed failures, revoked/expired/preview/unsigned/retired packages, missing exact SHA, unsafe path, missing component, or v2 ungranted closure. If authorization cannot be evidenced, remain disabled.
- Reuse `usePluginInstallSelection` for the picker eligibility state on both surfaces. Avoid a new resolver, generic adapter, persistence mechanism or feature flag. Keep management/lifecycle commands coherent for exact Git installs and existing saved dependency rules. Add concise i18n copy (all nine locales), clearly distinguishing signed target from exact-source path.
- Add positive Sentry-like MCP Pi Project unit+browser tests and negative unsupported extension/transport/source/revocation/preview/mixed-target tests. Do not claim live Pi tools/call from a UI test. If a concrete CLI route cannot be validated, stop short of enabling a misleading command.

### C. Publish the actual Hermes WebP
- Preserve the official user-provided image; do not restore the superseded Hermes wing or duplicate assets. Check README, site data and public paths.
- After all source and tests are stable, commit the page-source candidate, record its immutable SHA/tree, and update `website/production-source.json` in a **later commit of this same branch** to that ancestor. Merge by normal merge (not squash/rebase) so the pin remains an ancestor, and only when production source checks and released-CLI capability guard allow publishing. Never point the pin at an unaudited or future commit.
- Verify the deployed site uses the pinned WebP (HTTP content type and image pixels or screenshot), filter columns and Pi exact-source instructions. A published WebP is not evidence of Hermes/Antigravity installability.

## Review and acceptance

1. **Architecture/security review:** ensure signed Directory authorization, v2 closure, source ownership, direct-source explicit choice and native Project trust are never silently broadened. Reject unsupported components and unsigned/expired data.
2. **Product/UX review:** three-column medium layout, two-column narrow layout, mobile, RTL, tab/keyboard behavior, honest Pi fallback and unavailable client messaging. Correct findings before proceeding.
3. **Regression review:** exact-source CLI flags, project vs user scope, automatic vs manual selection, card/detail parity, saved preferences, readme/client icon parity, upstream source special cases. Keep unknown/error cases fail-closed.
4. **Final verification:** check unit/i18n/lint/TypeScript, real Playwright screenshots at desktop/tablet/mobile, normal GitHub PR-head CI (Required, Landing/browser, Docs, Pages where relevant), review threads; merge only after required exact-HEAD gates. Check post-merge main and production Pages provenance and live UI on the *published* SHA. Queued/skipped checks or a successfully built but un-deployed site are not a PASS.

### Scope and budgets

No CLI adapter, registry signing, GitHub environment, npm release, generated capabilities, or other clients' behavior change. No credential or trust bypass. Defer Antigravity IDE activation (#864). Approximate change: 90–180 production lines plus 100–200 focused tests and translations. Risk / complexity / expected reliability before tests: **6/10 / 5/10 / 7/10**. The Pi route and production pin are the high-risk gates; prefer a truthful disabled option over an unverifiable 'works for every plugin' promise.

### Review refinements

- **Round 1 (safety):** explicitly distinguish signed release eligibility from independently pinned Git acquisition, forbid implicit auto/multi-client fallback.
- **Round 2 (capability):** require *every* package component to have Pi Project support, not just one; exact MCP transport acceptance stays a CLI responsibility and must not be overstated.
- **Round 3 (deployment):** pin to an earlier reviewed commit on the same merge-preserved branch; merge method and published-source verification are essential.
- **Round 4 (independent CodeRabbit provenance review, #866):** a signed product's newest source and its Pi-selected eligible release can differ. Return the exact release source alongside the immutable selector; bind the card's Git link and SkillSpector subject to that revision. If the existing security assessment belongs to another source/tree, withhold it for that installation rather than misattribute the newer scan. Assert this with an active newer ChatGPT-app-only release and an older Codex-supported pinned source; link the matching Git revision in the install detail.
