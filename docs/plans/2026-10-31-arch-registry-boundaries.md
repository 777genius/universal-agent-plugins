# refactor(landing): separate registry decoding from resolution and presentation

Base: `822a9b9c2ba8a48a9c1bbb23da97b66e16c6257a`. Status: planned; implementation and independent review pending.

## Goal
Untangle the 1,324-line registry.ts without weakening the signed catalog's fail-closed integrity checks.

## Bounded scope
landing/utils/registry*.ts plus relevant existing tests; no localization or dependency migrations

## Architecture decision
Extract pure parser and validation for signed/legacy input; leave eligibility/resolution and user-facing helpers in a stable facade with unchanged exports. Avoid circular imports, maintain strict RFC3339/sequence/source digest/target validation and unchanged result/error shapes. Do not consolidate different trust protocols solely to save code.

## Non-goals / critics' challenges
A shared helper must not accidentally accept an older schema on signed paths. Keep legacy preview and published snapshot modes distinct.

## Exact completion gates
1. Make a behavior-preserving change in a focused commit.
2. Exercise the current test suites, actual CI and relevant OS/native E2E. Verify no security, source authority or file ownership regression.
3. Inspect a real PR diff; obtain an independent code/security review, resolve findings and ensure checks are green on the reviewed head.
4. Squash-merge only with an unmodified expected head and verified base; otherwise keep the PR open and report the blocker.

## Constraints
- Clean Architecture, SOLID, DDD and DRY are a means to maintain verifiable semantics, not justification for extra layers.
- No new runtime dependencies, public schema changes, data migrations, blanket lint suppression or unrelated PR modifications.
- A plan and a green CI are not evidence of production rollout; keep claims tied to actual tests.

## Validation
Registry unit/browser tests, landing lint and static build, signed-feed tests and relevant E2E; inspect downgrade and stale-data behavior.
