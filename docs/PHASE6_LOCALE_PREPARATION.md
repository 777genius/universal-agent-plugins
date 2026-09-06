# Phase 6 D3 maintained locale preparation

Task: uap-authoring-docs-locale-fallback-20260906.
Base: `bb08ee00f2734baa04265a5afee115772a815cf2` (initial clean HEAD).
Status: bounded preparation; no release or public activation acceptance.

## Content and preservation

All 13 D1 journey routes now have RU/ES/FR/ZH counterparts: use/index,
use/install, use/manage; build/index, skill, mcp-remote, mcp-stdio, hybrid,
skills, layout, checks, handoff; legacy/v1/index. Their canonical IDs match
English. The 52 minimal pages explicitly identify unavailable translations,
link to the exact canonical English content, and state preparation and migration
unavailability in the local language. They copy no executable commands. English
continues to own exact prepared command facts and the pinned v1 reference.

All 192 previous localized pages remain at their original URLs. Original
frontmatter and entire bodies are preserved verbatim inside collapsed historical
details, labeled plugin-kit-ai v1, baseline 1.2.4. Current Use/Build links and
preparation/migration status precede that archive. This classifies entire old
journeys as historical rather than leaving runnable v1 onboarding under a current
Build banner. Old headings, explicit anchors, links, snippets and release records
remain intact; old acquisition commands inside the archive are historical text,
not current installation recommendations. The 1.2.4 label is the historical
command baseline, not a claim that every older release note describes 1.2.4.

Preservation inventory: existing localized guides, concepts, reference, API
indexes, home pages and releases: preserve in place, historical/support context.
No useful material, implementation, dependencies or tests were deleted or moved.
No YAML fallback or second supported engine is introduced. The archival text's
old future/migration claims are superseded by the visible migration-unavailable
notice. Capability-preservation and implementation-plan owner clarification apply.

## Navigation and fallback

RU/ES/FR/ZH nav links lead to the produced localized Use/Build pointers. Old
non-release navigation is explicitly labeled v1. English nav is unchanged.
The switcher uses D2 registry paths (bound only after page production), retains
all five locale choices, and labels missing translations as English with actual
English lang/hreflang. Unknown entities go to an explicitly labeled locale home;
no guessed translated suffix or fragment is emitted. Counterparts use their own
canonical URLs and shared canonical IDs; English pointers are ordinary links,
not redirects or false claims of complete translation.

Locale preference detection handles deployment base boundaries correctly.
Gateway detection honors browser preference order, including English, and ignores
invalid saved locales. Existing manual gateway and optional-storage behavior stay
intact. Locale switching opens the counterpart page top: translated heading IDs
are not assumed identical. Existing direct deep links remain in preserved bodies.

## Verification and integration handoff

Node 22.21.1 from the supplied read-only docs-build-tools ran the five-locale
quality checker and seven focused Node tests. Coverage: 61 source routes in each
of five locales; matching canonical IDs; 52 English pointers and their internal
links; 192 byte-preserved historical bodies/frontmatter; switcher destination
selection for present/missing/unknown entities; deployment bases, html aliases,
query/fragment locale detection; saved/browser preference order; localized nav
routes and destination-language bindings in both switcher variants.

These are source/behavior checks, not a generated-site or browser acceptance.
No model/dead-link check was altered or weakened. No full generation/build was
attempted: the historical Claude extraction blocker belongs to the other worker.
ROOT must regenerate registries and assembled pages, run unchanged model and
full dead-link checks, then verify rendered details/heading fragments, fallback
links, nav, switcher, gateway/storage, canonical/hreflang and html aliases in a
non-published browser preview. Existing generated registries are deliberately
untouched and do not yet prove these new routes were assembled.

Original D3 handoff seam (corrected): `journeyNav` is called by locale configs
and consumer tests, not the gateway or generator. The generator calls
`journeySidebar`. The correction below updates both real call paths. The initial
D3 patch did not edit the generator, extractor, source contract, shared SEO,
English source, landing, package manifest, workflow or platform files.

The existing D2 preparation-preview guard stays mandatory. D5, final engine and
native/wrapper provenance, published channels, Milestone A, Windows/macOS/native,
landing and public activation remain unproven and unchanged. No installs/network,
product commands/suites, real profiles, restricted reproducer, pushes, PRs,
publication or additional workers were used. Requested worker policy is
 gpt-6-astra low/default, no fast; this is not hosting telemetry attestation.

One exclusive `.git/index.lock` probe failed EROFS; no retry or commit attempt.
ROOT owns review and mechanical integration of the exact terminal patch.
Terminal patch, SHA-256, base and per-file Git/SHA-256 identities, test logs and
complete HANDOFF are in:
`/tmp/uap-authoring-docs-locale-fallback-20260906-artifacts/`.


## D3 bounded review corrections — 2026-09-06

Task: `uap-authoring-docs-locale-fix-20260906`.
Exact correction base: `7d80ef0652daacfd8d9a8ab56d1deff0b3dd0928`.
This section supersedes the initial source-only verification and outside-ownership
seam above. Preparation status and all release/platform holds remain unchanged.

`journeys.mjs` now owns the five nav route pairs and localized Use/Build/sidebar
labels, including journey item labels. All locale configs call `journeyNav`;
production `buildSidebar` calls `journeySidebar`. Mandatory consumer assertions
check actual source files, matching locale/canonical IDs, and corresponding
sidebar routes for all five languages, while retaining missing-generated-page
English fallback and produced-page registry checks. Existing browser navigation
expectations use the same five locale routes. Locale nav keeps all historical
links under a v1 dropdown: an owned browser observed the RU switcher at x=1443
outside a 1440px viewport with the former seven top-level items.

The seven locale tests are now in the mandatory `docs:test` runner alongside
all existing tests. A private Node preload adds a deliberate failing locale test
without changing repository test bytes; `pnpm docs:test` exits 1 with that exact
failure. The ordinary runner retains real adapter, repeated bounded assembly,
source identity, dirty-source and drift negative checks. Nothing bypasses the
preparation guard, model checks, VitePress dead-link checks or output checks.

`website/tools/quality/locale-dispositions.json` is the explicit source inventory.
The initial 52 pointers are `english-fallback`; only this disposition forbids
copied fenced commands and requires the precise English pointer. A new translated
Build tutorial, installer guide or release note uses frontmatter
`localeDisposition: current-translation` (or an explicit inventory entry), with
normal locale/canonical-ID and full-build link validation. It need not contain a
v1 notice and may contain current command fences. New pages are not automatically
classified as historical because of their route. Unknown dispositions fail.

The 192 `historical-snapshot` inventory entries additionally pin original complete
file, frontmatter and body SHA-256, derived from exact reviewed preserved bytes;
reviewed wrapper-file SHA-256 and original/reviewed commit identities are recorded
as provenance, not runtime Git dependencies. Missing or changed archive bytes
fail; the preservation test never silently skips. Original frontmatter, bodies,
headings, snippets, YAML assets and release records remain verbatim. An inventoried
archive can be adapted by setting its `currentDisposition` to `current-translation`
and adding current prose before the preserved disclosure. The original archival
membership and hash fields stay intact. This avoids requiring an edit to immutable
original frontmatter. Future removal still requires inventory and explicit owner
acceptance under the preservation contract.

Each archive wrapper now exposes its page title outside the disclosure and links
to the pinned English historical context. The local summary explicitly calls the
content a historical branch snapshot: 1.2.4 is the command baseline, not the exact
version of that snapshot. All original release version/date text stays intact.
Read-only Chromium diagnostics on ROOT's exact 7d80 build confirmed direct encoded
fragments reveal targets, but VitePress client navigation leaves them hidden in
all four locales; the original H1 is also hidden on ordinary entry. The locale
lifecycle component now installs a small fragment observer that opens all enclosing
details and focuses/scrolls the preserved target after client DOM insertion or
hash/history changes. It disposes its observer/listeners on unmount.

The same real browser uncovered a second gateway call path: the early head script
in `source/gateway/index.md` ran before the Vue component and preferred any RU/ES/
FR/ZH candidate over earlier English. It now applies saved-valid-code precedence
and ordered normalized browser languages, including English. The focused gateway
test executes that exact inline script as well as the shared route helper; the
browser exercises the real early redirect. Manual mode and optional storage stay.

Durable built-site coverage is in `locale-browser.mjs`, invoked by the existing
site-consumer browser smoke. It covers encoded direct/client fragments, both URL
spellings, reload/back/forward, keyboard disclosures, visible identity, both locale
switcher variants, real counterparts/English fallbacks/unknown homes, reciprocal
canonical/hreflang, and saved/invalid/blocked-storage/browser-order/manual gateway
behavior, mobile outline links and an explicit nested-disclosure browser fixture.
No current archived source contains a nested disclosure. Owned diagnostics reuse read-only 7d80 HTML with explicitly recorded
source overlays; they do not establish a final regenerated Vue/build acceptance.
ROOT must regenerate and run full generation/build/model/dead-link/output/browser
gates on the integrated patch.
D4 landing, D5 release, native/Windows/macOS, Milestone A and publication holds stay.

Exact terminal patch, identities, test logs, overlay limitations and final gate
inventory: `/tmp/uap-authoring-docs-locale-fix-20260906-artifacts/HANDOFF.md`.
Only one Git lock probe was made; it failed EROFS. No retry or commit was attempted.


Correction validation: mandatory `pnpm docs:test` passed all 23 tests with no
skips plus its three real bounded assemblies and negative source/drift checks.
The injected failing locale test produced exit 1 through that same entrypoint.
The source checker passed 61 routes × 5 locales. A delivery fixture with no Git
repository passed all seven locale tests and the checker after adding current
translated command tutorials and release pages (63 routes × 5 locales). Exact
origin audit verified all 192 original/reviewed/archive identities; in-memory
VitePress rendering retained all 1197 historical IDs and exposed all 192 titles.
The updated SFC compiles in memory. The existing consumer browser smoke and new
locale smoke passed an owned local diagnostic with source nav/site-data, early
gateway script, visible-title paragraphs and the exact fragment helper overlaid
on ROOT's read-only 7d80 output. This includes all 12 existing alias cases, both
prepared roots, five navs, desktop/mobile switchers and the targeted locale cases.
The overlay is diagnostic evidence; final SSR/hydration/bundling and complete
regenerated registries/build remain explicitly unproven until ROOT runs them.
