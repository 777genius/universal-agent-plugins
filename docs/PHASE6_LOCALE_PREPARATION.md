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

Outside ownership: `website/tools/lib/journeys.mjs:journeyNav` still returns
English preview nav for generator-owned gateway/sidebar consumers. ROOT may
replace its two links with `/${locale}/use/` and `/${locale}/build/` and localized
labels now that these source counterparts exist; reconcile owning site-consumer
assertions. No generator, extractor, source-contract, shared SEO, English source,
landing, package manifest, workflow or platform file was edited here.

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
