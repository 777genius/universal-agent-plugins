# Public discovery, Skills and SEO: bounded E2E plan

Date: 2026-11-09
Base: `00bcdf486dabef2dc2a4e6cb239bae818a451ab0`
Status: Implementation in this PR; release and deployment are separate, verified checkpoints.

## Goal
Explain the **full** Universal Agent Plugins product to users, crawlers and AI agents without falsely representing client support, discovery counts, scan guarantees or unreleased authoring capabilities.

## Scope
1. Document the product's released installer, independent Agent Skills installer, native client boundaries, signed Directory, dependency lock/restore, safety and published SDK surfaces with verifiable links.
2. Clarify static first render (reviewed seed) vs signed *client-side* discovery augmentation. Do not turn a reviewed seed count into a purported live total or claim search bots run JavaScript.
3. Enrich the existing `llms.txt` and AI-catalog resource listing with concise evidence links, Skills discovery, SDK and limitations. Keep the outputs crawlable and avoid synthetic scores or fabricated installation metrics.
4. Add a stable, indexable, human-readable Skills **overview** page with useful static text and links to the client-loaded Skills catalog. Preserve non-indexing of query-specific Skill detail URLs and third-party descriptions.
5. Improve static page identity and SEO tests for canonical URLs, alternate locales, HTML route publication and evidence references; keep the current prerender/SSG infrastructure.
6. Document UAP/APM differences factually, including APM marketplaces and enterprise policies, rather than claiming APM has no catalog or UAP is universally superior.
7. Keep this PR strictly limited to product information, SEO and publication checks.

## Out of scope
- No changes to installer, registry signing, authorization or native install behavior; no invented catalog/scan/runtime support.
- No new SEO framework, analytics, AI-bot-specific cloaking or bot-only alternative content.
- No new enterprise policy engine.
- No release or production deployment without successful exact-head checks.
- Avoid editing files owned by open PRs #807 (auth/ChatGPT), #808 (Skills picker/VS Code) and #809 (catalog filters). Coordinate changes to shared documentation/translation files carefully.

## Acceptance
- HTML contains a truthful explanation that the reviewed catalog is the initial static render; additional community records arrive after signed browser-side discovery and can be unavailable.
- `llms.txt` clearly indexes installed Skills, product capabilities, SDK, lifecycle, native readiness evidence and catalog loading semantics.
- The static Skills overview has meaningful non-JS HTML, an indexable canonical and localized alternates, but query-specific Skill details remain noindex.
- No duplicate/contradictory public counts, no claim of live verified package execution on account of file installation.
- TypeScript/route/i18n/SEO tests pass; generated static artifact and browser E2E exercise new pages, crawl text and links; CI is green before merge.
- Review for inaccuracies and regressions; only merge after requirements are met.
