# P1 — Marketing Kit Project/Copilot release qualification

## Context
Continue existing draft PR #775. Five signed Marketing Kit Project client routes have historical E2E evidence; a genuine published `0.1.101` Copilot install fails **before** lifecycle because the signed Directory root lacks Copilot authorization. Latest six-client real run also reported immutable binary/scanner HTTP 504. HTTP 504 is an acquisition failure, not evidence that native lifecycle passed or failed.

## Minimal plan
1. Keep the exact five qualified routes and the sixth Copilot lane in the existing harness; do not invent extra framework/test identities.
2. Separate three outcomes: temporary immutable binary/scanner download failure; **correct signed eligibility refusal**; and genuine accepted Copilot add/list/repeat/update/cold locked restore/remove. Never relabel either of the first two as a lifecycle PASS.
3. Require a separately reviewed, correctly signed immutable Directory v1 root and mandatory child closure authorizing Copilot. **No silent grant/schema v2, revoked or borrowed publisher signature**, and no public release/grant mutation without explicit authorization.
4. On authorized signed release, test real pinned `@github/copilot` CLI Project A/B and retained foreign files after every operation, owner boundaries, lock digests, supported native MCP behavior. Validate six lanes and all original JSON/ZIP, not only workflow color.
5. Exact-head review/CI, merge only after signed grant and actual Copilot lifecycle succeed; post-merge production acceptance. Current npm version/release timing does not block unrelated test improvements.

## Non-goals
No scanner bypass, no forced positive expectation for unsupported Copilot, no new public grants automatically, no duplicate PR replacing #775.
