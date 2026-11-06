# Plan: faster Pages artifact validation and Playwright sharding

Date: 2026-11-06. Scope: Landing CI only. Status: two critical reviews incorporated; execute in two small, independently qualified PRs. No release or client runtime changes.

## Evidence from exact GitHub Actions jobs

- PR #735 job `landing-test` ran 825 seconds total; full Playwright 415 cases / 4 workers: **413 pass, 2 skip, 660 seconds** (subsequent postmerge: 645 seconds).
- The two independent `combined-pages-validation` jobs took **730s** and **696s**; their `Prepare and validate landing` steps were **390s** and **377s** respectively, while browser integration took **224s** and **209s**.
- Within preparation, `pnpm run test:artifact` reports **268,668ms**. Its negative mutation cases repeatedly invoke `checkPagesArtifact` against a 468-HTML/31-plugin fixture on both deployment bases.
- Current `checkHtmlReferences` rebuilds the route/namespace sets for every page and scans the entire route list for each link. `checkPagesArtifact` searches all expanded routes and recomputes SEO for each sitemap URL. These are avoidable within-call repeated computations, not useful coverage.
- Two Pages bases are semantically distinct; signed snapshots, full artifact bytes, docs anchors, SEO, route scoping and fail-closed mutation tests must stay verified at both bases.

## PR 1 — indexes, not skipped validation

1. Build a **fresh reference index within each `checkPagesArtifact` invocation**, from the current actual route list and file inventory; pass it to `checkHtmlReferences`. The helper's standalone callers still build the same index on demand.
2. Replace repeated linear route searches with `Set.has`. Populate SEO-by-canonical once from verified route results and use it for sitemap assertions; reject duplicate canonical identities.
3. Replace the per-HTML full registry-description scan with a fresh description-name lookup, testing the actual payload strings against the same allowed projection descriptions. Preserve the existing refusal semantics for duplicate plugin descriptions.
4. Preserve all real `readFile`, full signed mirror/docs byte equality checks, payload scoping, output/ref anchors, invalid mutated files, same test count, and release gates. **No cache across invocations or mutable fixtures.**
5. Measure `test:artifact` and full two-base Pages jobs against the above baseline on exact PR head. Restore any check that becomes weaker; never excuse a red CI job as a performance win.

## PR 2 — shared verified build, two browser shards

1. Keep one verified signed registry mirror, `nuxt prepare`, typecheck, registry tests, lint, i18n and `nuxt generate` in one preparation job. Upload the staged static site **only inside the same GitHub Actions run**, with immutable run-scoped artifact identity. This job must fail closed.
2. Run two browser shard jobs on separate runners, installing the same pinned dependencies and Chromium. Each downloads the exact prepared static site, runs `playwright test --shard=N/2`, and uploads its report/traces. On **docs-only** safe PRs, one smoke shard is enough. Unknown/landing changes, push, scheduled and manual runs always select the full suite.
3. Keep a single **`landing-test` aggregate status** for existing branch protection; it fails if preparation, either shard, report consolidation or required visual screenshots fail. Merge blob reports to prove the full suite rather than accepting partial green shards. Preserve Marketing Kit screenshots and per-shard failure traces, even for a failing shard.
4. Keep two independent combined Pages validation bases, unchanged in behavior. Do not share a site built for one base as evidence for another; those jobs validate their own final deployed artifact.
5. Benchmark real total Landing wall time (critical path, excluding queue), runner-minutes and exact pass/skip counts. Target ~7–9 minutes only as a hypothesis; if artifact transfer outweighs sharding, prefer measured simpler CI.

## Critical reviews — incorporated before coding

**Review A, security and correctness.** A global cache of successful validation would miss the negative tests that mutate files; rejected. A route/namespace map is valid only within one invocation and must derive from the *current* on-disk tree. No reduction in strict signed-byte comparison, anchor checks or allowed-URL rules. Canonical duplicates must be rejected rather than silently picking one.

**Review B, CI and reliability.** Naive sharding would build the site twice, increase runner contention, break required `landing-test`, and lose screenshots. Revised to a run-scoped prepared artifact, two isolated shards, aggregate gate with explicit failure propagation, and merged reports and screenshots. Preserve docs-only fail-open selection and nightly full testing. Avoid new libraries/feature flags and no `continue-on-error`.

**Review C, performance skepticism.** More workers on an already 4-core runner risk CPU contention. Don't do 8 workers or 4 CI shards without measurements. Do PR 1 independently before PR 2; compare step-level time, screenshot evidence and all three Pages/landing checks. No fake claims from queued, cancelled or skipped jobs.

## Completion criteria

- PR 1: exact-head `Landing` full browser and both Pages bases green, `Required`, CodeQL and other applicable CI green; two critical review passes; guarded squash merge; verify postmerge main.
- PR 2: shared artifact provenance and script checks, docs-only smoke, full actual 413+ browser E2E and two Pages bases, one failing shard propagates to the `landing-test` gate; full required CI green, guarded squash merge and postmerge main verification.
- Record per-step durations and all changed files; don't blindly rerun CI. Preserve 2 known skips as skips. Avoid changes to installers, registry security policy, generated artifacts and VS Code.
