# refactor(cli): isolate source acquisition and catalog evidence workflows

Base: `822a9b9c2ba8a48a9c1bbb23da97b66e16c6257a`. Status: planned; implementation and independent review pending.

## Goal
Reduce coupling in 1,125-line source.go and keep source and directory trust decisions reviewable in isolation.

## Bounded scope
cli/internal/agentpluginscli source handling; focused tests, existing lint baseline only if necessary

## Architecture decision
Extract coherent source acquisition, directory release processing and evidence/policy helpers into same-package files. Keep App as facade, preserve exact signed snapshot validation, immutable digest expectations, existing error and cleanup paths. No new package manager, network fetch, or silent fallbacks.

## Non-goals / critics' challenges
Avoid changing source precedence, repo pinning, verified evidence or ownership/cleanup. No client-specific changes.

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
Uncached CLI source, update/repair/replay, native E2E, pinned conformance, Required; inspect unsafe paths and signature/digest behavior.
