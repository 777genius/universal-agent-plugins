# Plan: faster Landing Playwright without removing regression coverage

Status: reviewed design; validate run durations and quality on real GitHub Actions.
Date: 2026-11-06. Target: `ci/landing-playwright-fast-20261106`.

## Baseline and constraints

At PR #724 head `451b8294`, real `landing-test` browser logs report **415 tests using 2 workers; 413 passed, 2 skipped, 24.5 minutes** (browser step alone). The current `.github/workflows/landing.yml` runs the entire suite for any matching PR, including root README and docs-only changes; it also runs two independent combined Pages bases for PRs. These two Pages checks are an important full artifact gate and must remain. The workflow already cancels superseded PR runs; do not manually rerun same SHA. Other checks and production Pages deployment remain unchanged.

## Proposed minimal changes

1. In `landing/playwright.config.ts`, use up to four workers on CI, keeping current behavior locally. Do not shard, duplicate Nuxt builds, run extra runner matrices, introduce caching trust issues or rerun known-green tests unnecessarily. Measure actual effect; revert if CI becomes flaky or slower.
2. In existing Landing workflow, classify only a **safe, tiny allowlist** of docs-only PR changes (`README.md`, `website/**`). On those PRs run a representative existing browser smoke set instead of all browser tests; still run registry, typecheck, lint, i18n, generation and both full Pages artifact integration jobs. An unknown path, failed API call, changed `landing/**`, renamed dangerous file, oversized diff or workflow change **fails open to full suite**. Avoid a new classifier service; use the existing GitHub CLI on the runner.
3. Full browser suite for any UI/landing code or tests PR, all pushes to `main` matching Landing paths, and an explicit daily scheduled full run. Manual release qualification continues to use the full job. Keep Playwright traces on failure; never suppress failing tests with `continue-on-error`.
4. Only if GitHub logs demonstrate the faster mode is correct, adjust which check is required. Do not temporarily disable `landing-test`, `combined-pages-validation` or browser tests.

## Review: two critical passes and incorporated corrections

**Round 1 — performance and maintainability.** Rejected 4-way GitHub shard matrix: it repeats Go/pnpm/Nuxt/registry setup and consumes scarce runners. Started with increased Playwright process workers and safe docs-only selection in the *existing* workflow. Avoids a second CI framework or unneeded build artifacts.

**Round 2 — regression safety and release.** Blind path-based selection can skip tests for CSS, translations, shared components or workflow edits. Changed the plan to fail open to the **full** suite for every `landing/**` file and any unknown change; only README/website docs use smoke. Keep the two combined Pages validation jobs on every PR, full on main and daily. Aborted an idea to use `--last-failed`, cached test results or stale prior SHA as release proof. Explicitly record 413 passing and 2 skipped, not “415 passed.”

## E2E and measurement

- Syntax/contract check of workflow and Playwright config; targeted smoke of docs-only branch.
- A UI PR runs the *full* suite on the exact head and validates both Pages bases. Ensure no skipped/cancelled job is represented as PASS.
- Compare previous baseline (2 workers, 24.5 min) against the new full run; target less than ~15 min browser wall time subject to runner capacity. For docs-only PR, target a few minutes of browser time (the full Pages docs build remains a separate bottleneck).
- Review a genuine PR on new head, inspect `Required`, `Landing`, both Pages bases and CodeQL as applicable. Guarded squash merge after success; compare postmerge main.
- Do not add `ci:fast` flags, skip tests on critical UI changes, or change GitHub environment protection rules in this scope.

## Footprint

~30–65 workflow/config lines; reuse existing tests. No new dependency, codegen, or state. Revert worker increase if empirical run shows flake/resource pressure.
