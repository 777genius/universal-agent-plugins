# refactor(core): separate transaction lifecycle and recovery responsibilities

Base: `822a9b9c2ba8a48a9c1bbb23da97b66e16c6257a`. Status: planned; implementation and independent review pending.

## Goal
Protect the highest-risk commit/rollback/recovery invariants while reducing the 1,436-line transaction kernel's unrelated responsibilities.

## Bounded scope
install/integrationctl/agentplugins/transaction and only narrow architectural guards/tests if essential

## Architecture decision
Keep existing package, exported signatures and persisted receipt formats. Separate operation execution (single/group), crash recovery, owner authority and durable receipt/state coordination into focused files. Reuse exact existing checks; do not change authorization or rollback semantics. Any deduplication must preserve preflight-before-effects and rollback classification.

## Non-goals / critics' challenges
A cosmetic split does not solve all infrastructure coupling; do not introduce a generic transaction framework or unsafe mock PathPolicy. Do not alter public interfaces.

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
Uncached transaction tests, usecase/installer regression, architecture test, Required and cross-platform checks; review state-commit ordering and fail-closed paths.
