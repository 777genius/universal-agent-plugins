# BUG-022 — post-remove package re-registration under a fresh ID

## Source-backed reproduction and risk

The latest independent Strix review of [PR #698](https://github.com/777genius/universal-agent-plugins/pull/698) on 2026-11-04 reproduced a false-PASS harness oracle: the post-remove check compares only old installation IDs against new IDs. A successful remove followed by the same package and target under a **new** installation ID would be certified removed, even with an empty native file snapshot. This is a harness proof defect, **not** evidence that UAP production reinstalls a package.

## Minimal plan

1. Reuse existing `has_target_registration` on the post-remove `list` JSON. Reject same package name and client target regardless of installation ID.
2. Retain old-ID disjointness, list schema validation, native residual checks, lock checks and foreign-sentinel checks; expose a boolean in the case report.
3. Regression: add and repeat succeed, remove succeeds, old ID disappears and native file is removed, but `list` re-registers the same identity with a new ID. Expect `cleanup-failed`. Existing clean removal remains PASS.

## Critical review round 1 — safety

Reject a re-minted registration without deleting or rewriting native state. Require exact package name + exact target, not an unrelated package or substring.

## Critical review round 2 — scope and reliability

Change only E2E harness proof and regression. No installer modification, no LintAI/cgroup bypass, no feature flags. Do not treat unit results as full 30×30 qualification; recheck live Actions, raw JSON artifacts, and independent review on final branch HEAD.

## Status

Plan recorded before implementation. Implementation and final-HEAD qualification are separate gates.
