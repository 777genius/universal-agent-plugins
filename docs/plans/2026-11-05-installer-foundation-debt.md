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

## Implemented on this Draft branch

- **P0 — dependency inversion:** `agentplugins/usecase.Service.LegacyLock` now accepts the same `ports.MutationLock` seam already used for the new mutation lock. Only `adapters/legacylock.StateLock` names the old `integrationctl/ports.LockManager`. `cli/cmd/agentplugins` owns composition, preserving the existing `~/.plugin-kit-ai/locks/state.lock` physical namespace. The old state-migration caller receives the same adapter.
- **P0 — reconciliation safety:** after the old `Legacy.Remove` returns, acquire the legacy-state lock, verify the old state is absent, and hold that lock until the state-v2 save returns. The outer new mutation lock still covers the use-case operation. The code does **not** nest the old `Legacy.Remove` within its own lock.
- **P0 tests:** verify `StateStore.Save` runs under the old lock, successful and failed saves release it, failed saves preserve v2, reappearance refuses reconciliation, and the new adapter contends with the **same actual file lock** on both directions. No journal/receipt schema changes.
- **P1 — bounded CLI splits:** `read.go` fell from 740 to 380 physical lines, separating read commands and public DTO rendering; `lifecycle.go` fell from 1,005 to 428, separating command wiring, result presentation and existing selection helpers. Functions were moved without modifying their bodies or public format. Risky transaction/multi-target functions remain on their original paths.
- **Architecture ratchet:** add both depguard and executable archtest enforcement against importing legacy `integrationctl/ports` from the new use case. Narrow the grandfathered `file length is` suppressions for the two shortened CLI files, with **no new exclusions**.

### Critical reviews, evidence and limitations

1. **Safety perspective:** the lock must begin only after legacy removal returns; verifying absence and saving v2 must share one lock lifetime. Existing concurrent-reappearance and new lock-held-save / failure tests enforce the contract.
2. **Architecture perspective:** prefer the existing `ports.MutationLock` interface and one adapter over a new lock framework or a duplicate state engine. No additional domain aggregate, event bus, feature flag or dependency has been added.
3. **Compatibility perspective:** adapter uses the exact old `"state"` key and underlying `locks.FileLock`, and all moved CLI declarations stay in the same Go package. The exported Go `Service.LegacyLock` field type changes; Go callers outside the repository constructing this internal-oriented application service may require adaptation, even though CLI and JSON contracts are unchanged.
4. **Independent review:** a manual full CodeRabbit review was requested on PR #713, but the service reported a rate-limit refusal rather than review findings. Do **not** count it as a completed independent review.
5. **Hosted CI:** checks on the final PR HEAD, not older superseded commits, are the release gate. Earlier CI identified an import-ordering defect in `cli_test.go`, which has been corrected; successful checks on superseded commits are not final-head evidence. No claim of fully passed CI until that exact HEAD finishes.

**Remaining scope:** leave multi-target transaction orchestration, client-specific native transitions and the live Directory/dependency PR changes untouched in this Draft. Follow-up refactors need their own characterization and E2E proof, not an inflated diff.
