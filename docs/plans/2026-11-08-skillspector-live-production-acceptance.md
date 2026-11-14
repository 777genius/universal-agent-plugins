# P1 — SkillSpector live acceptance: production ref and browser verification

## Current reproduction
Production acceptance run `42456634551` reached real deployed site / Skills API checks but failed reading existing GitHub branch `skillspector-reports` with unauthenticated GitHub REST HTTP 403. Exact cause of 403 (rate limit vs denial) was not recorded. Continue the **existing** PR #790, which supplies GitHub Actions built-in token only in the trusted fixed-host ref request; redirects rejected; non-200 fails closed.

## Minimal completion
1. Review token scope and step permission `contents:read`, exact fixed github.com API host and no redirects; never log token, leak headers to external endpoints, or accept arbitrary ref.
2. Unit/contract test HTTP 200, 403/404, redirect and malformed ref; no fallbacks to stale signed Complete or mock report.
3. Run exact-head Required, Landing/typechecks, security review, real API+Chromium production acceptance after signed cohort publication; compare ref SHA, signed reports/manifest, accepted entries vs displayed catalog and no false Partial/Complete.
4. Verify actual deployed site and publication owner/content/anti-rollback; production checks are independent of PR-only scripted probes. Preserve signed Directory v1 and original report fixtures.
5. Independent review → SHA-gated merge → post-merge live acceptance. If GitHub public API continues to return non-200, retain red job/diagnostics and investigate rather than claiming success. No full scanner/site refactor.

## Non-goals
No new signing mode or registry schema, no bypass of HTTP 403, no fake success with historical artifacts, no duplicate replacement PR.
