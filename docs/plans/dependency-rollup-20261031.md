# Dependency rollup: generated Node template and docs toolchain

Date: 2026-10-31. Scope: existing Dependabot PRs #573 #546 #512 #491 #489 #463 (Node-generated scaffold lock) and #465 (Python docs certificate bundle).

## Decision and reason
- Six npm PRs overlap in one generated `cli/internal/authoring/scaffold/templates/package-lock.json`. Individually merging stale branches causes golden fixture failures and excessive main/Pages runs. Combine their exact bot-provided integrity-checked lock entries once; prefer qs 6.17.0 (superseding 6.16.1 from body-parser's earlier bot run).
- Update the SHA-256 pin for embedded Node lock bytes and generated mcp-stdio / hybrid-stdio golden tree hashes. Never disable the byte-for-byte guard.
- Update only the certifi package plus its two upstream wheel/sdist hashes; preserve all other pinned Python generator/toolchain requirements and comments.
- Explicitly exclude #574 `vue-i18n@11.4.14`: its bot CI has a real Rollup error caused by cross-major `@intlify/core-base@11.4.14` resolving `@intlify/shared@10.0.8`. It needs an isolated, compatible migration and full landing E2E, not an unsafe lock-only bump.

## Required validation
- Verify exact bot-sourced versions, integrity hashes, semver peer/transitive compatibility, no dependency removal.
- Go scaffold byte-integrity test, golden deterministic generation for both Node stdio templates, current standard loader, native Windows/macOS/Linux, and full required CI.
- Real generated Node npm ci and MCP stdio startup when feasible in CI; no skip/weakening of invariants.
- Python `pip install --require-hashes`, docs build, and Landing build integration.
- Independent security review and green CI on the exact current PR head before squash merge.
- Close superseded bot PRs with precise explanation only once the validated consolidated PR merges.

No runtime public API migration, new framework, or unrelated PR changes.
