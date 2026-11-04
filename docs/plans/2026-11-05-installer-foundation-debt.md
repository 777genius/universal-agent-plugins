# 2026-11-05: Installer foundation debt — bounded refactoring plan

**Status:** working plan, Draft PR only. **Baseline:** `main` at `8aa409933baed3886150a24c74ce5908c6eff632`.

## Evidence and boundaries

- `usecase.Service.LegacyLock` directly names the older `install/integrationctl/ports.LockManager`. This is the remaining behavioral outside-layer dependency documented in `docs/ARCHITECTURE.md`.
- `usecase/legacy_remove.go` currently acquires the legacy `state` lock inside `verifyLegacyAbsent` and releases it **before** marking and persisting the v2 state as absent. The gap allows a legacy writer to change its state after verification, before the new state is saved.
- `cli/internal/agentpluginscli/lifecycle.go` (1,005 lines) combines update/repair/remove, command construction, target selection, and result rendering. `read.go` (740 lines) combines list/info, doctor and output DTOs. These remain on the shrink-only lint baseline.
- New management application/state/HTTP boundaries are already implemented; do not overwrite or reimplement them. Active client and Directory PRs #707–#710, and native E2E PR #698, must not be disturbed.
- `clients.Env`'s `nativeconfig.Kernel` is a known documented exception. Do not add an interface with no second consumer merely to improve a diagram.

## Intended change (one lifecycle/architecture family)

1. Replace the legacy lock type visible to the **new use case** with the existing narrow `ports.MutationLock` signature. At the infrastructure edge, adapt the original `legacyports.LockManager` while retaining **exactly** its `"state"` lock namespace, same physical lock root and existing errors. The adapter is a compatibility bridge, not a second lock implementation.
2. Hold that legacy state lock **across verification and v2 reconciliation save** after the old `Legacy.Remove` completes. Preserve the existing outer mutation lock, lock ordering, confirmation gating, receipts, rollback behavior, and failure-closed response. Ensure all exits release both locks.
3. Add tests for the compatibility bridge and lock-held-through-save invariant (including lock failure / legacy reappearance); keep public JSON, CLI exit contracts and golden snapshots unchanged.
4. Only after P0 passes, consider low-risk *behavior-preserving* splitting of lifecycle/read CLI code along real responsibilities. Do not migrate monolithic functions to new lint-exempt files, move VS Code code, or change public output. If the split would trigger new suppressions, defer it; shrinking a file is not a reason to weaken the architecture gate.

## Alternatives rejected

- Reusing the old `LockManager` directly in `usecase`: keeps the documented architecture breach.
- A new lock service/framework or a second mutation engine: needless abstraction and potential semantic drift.
- Locking before `Legacy.Remove`: possible nested acquisition of the old `"state"` lock.
- Holding the old lock **only** for the negative check: preserves the verified TOCTOU gap.
- Blindly chasing 10–15k changed lines: cosmetic moves conceal review risk. Size is an outcome, not an acceptance criterion.

## Verification and review gates

1. Run `make lint`, `make test-core`, full Go/CLI compile and targeted legacy removal tests on the final PR HEAD in CI; lifecycle changes also require relevant native/cross-platform tests.
2. Review independently from three angles: **safety** (lock order, races, refusal/cleanup), **architecture** (dependency direction, SOLID/DRY), **compatibility** (public output, install/recovery/OS behavior).
3. Fix substantive review findings and repeat tests; no broadened `.golangci.yml` exemptions, generated-file edits, feature flags or unnecessary new interfaces.
4. Keep the PR **Draft**. No merge or release is authorized by this task.

## Risks and reversibility

- **Deadlock:** do not nest legacy lock around `Legacy.Remove`; acquire only after it returns.
- **State consistency:** a failure to acquire the legacy lock or verify its absence must leave v2 materialization unmodified.
- **Crash during reconciliation:** retain atomic state v2 storage; no change to durable state format.
- **OS lock semantics:** preserve the existing legacy `locks.FileLock` implementation and root; verify on hosted CI.
