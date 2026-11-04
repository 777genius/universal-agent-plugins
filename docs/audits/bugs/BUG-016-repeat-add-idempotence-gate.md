# BUG-016 — Full matrix labelled lifecycle PASS without a repeat-add idempotence probe

## Reproduction (2026-11-04)

On PR HEAD `f27693996b2bc3f47735fc4491f8cc88acc6a59e`, `scripts/live-install-audit.py:do_case` executes exactly one add, an installed-state list/lock/native proof and remove/post-state checks. It never repeats the same add before removal. The [real 900-case historical ZIP matrix](../2026-11-04-raw-zip-417533.md) records `passed` results without any `repeat_add` output or retained-ID verification. This is a **test harness evidence gap**, not a demonstrated UAP repeat-add product defect. The acceptance request explicitly includes idempotence.

## Minimal plan (before code)

1. Only when the first add is successful **and** the exact installation is verified in list, invoke the identical pinned add a second time in the same isolated case before remove. Capture the real repeat exit and list JSON. Require success plus the exact same installation ID set for that source/target and preserved managed lock contents; record the evidence per row. Do not run the repeat after refused/unsupported/failed first add.
2. Snapshot *only* the content-addressed native client roots and compare them before/after repeat. Keep logs/caches as evidence of changes rather than deleting or weakening the oracle. Foreign-neighbor sentinels remain checked before status promotion.
3. Add a regression that proves zero-exit initial add + valid list + failed repeat cannot be PASS; successful repeat retaining the same identity can pass if removal and native checks qualify. Avoid restructuring unrelated source/target classifications.
4. Run the complete existing Python guards, then inspect fresh final-HEAD Actions JSON ZIPs and independent review. Never retroactively qualify the prior 374 historical PASSES for this new condition.

## Critique round A — evidence and security

Repeat must use the *same* source, target, scope, project, disposable HOME and policy gates; no retry that bypasses LintAI/cgroup or reuses host account state. A zero-exit repeat alone is insufficient: identity and immutable receipt must be retained, not duplicated or replaced. Do not fake PASS from a skipped, failed or unsupported repeat.

## Critique round B — scope and non-overengineering

No new feature flags or generic test framework. A handful of captured command outcomes and explicit predicates in the existing `do_case` is sufficient. The existing complete removal, lock pin, native state and foreign-sentinel checks still apply. If the native client mutates a legitimate log, retain evidence and classify conservatively rather than delete unknown files.

**Status:** repeat-add source/identity/native-state/lock oracle and regression pushed; full latest-head E2E and independent clearance still pending.
