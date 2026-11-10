# Popular agent project readiness and native integrity program

2026-11-02 — source-only design for one large follow-up PR. Scope is deliberately independent of PR #674 (real Codex model-driven MCP E2E) and other active PRs for Skills catalog and installer core hardening.

## Problem and evidence

`agentplugins doctor` currently collapses `ActivationPrepared` and `ActivationManual` into the same `degraded/activation_pending` finding. For project Codex, Claude Code, OpenCode and similar clients, `Prepared + installation_verified` is a truthful completed **delivery**, not a broken installation or proof of native use. The existing human guidance repeats an `add ... --target` command without naming the selected project root, risking an accidental user/project mismatch.

Project MCP/Skills files can also be absent, replaced by links, or drift outside the selected project after a prior successful transaction. Generic package digest proof is separate from the exact project-native files. A read-only doctor should point out these *observations*, never repair or claim runtime success.

## Architecture and constraints

1. Keep the state machine authoritative: classify persisted facts, do not synthesize activation, policy, authentication, native trust, tool calls, or a vendor login.
2. Add a pure project readiness assessment in the CLI's presentation boundary; use the existing domain states and no new persistent schema, feature flags, timers, external calls or production activation side effects.
3. One stable `project_readiness` JSON array sorted by installation/client/binding. Never include user/home/profile filesystem paths, raw config, secrets, auth or command output.
4. Clarify `prepared`, `manual_action_required`, `recorded_runtime_verification`, `blocked`, `failed`, `authentication_pending`, and `not_checked` without converting an unknown native state to "healthy".
5. High-value client-specific next actions for Codex, Claude, OpenCode, Gemini, Copilot, Cursor, Pi and Kimi; other agents receive conservative generic guidance. Every project action includes an explicit `--scope project --project-root <PROJECT_ROOT>` when suggesting an installer command.
6. Add a bounded, observational audit for native **Codex project** component receipts: the recorded native root is the authority; verify exact project-relative skill and MCP layout, reject link traversal, detect missing/invalid native files, duplicate native object identity and missing expected MCP tables. Treat inconclusive state as unknown rather than success. No silent modifications.
7. Pure and integration tests cover the full client matrix, ordering, scoped actions, no false project degradation, spoofed/edited/missing files, wrong TOML, symlinks, sibling isolation, no external process/network/auth, fail-closed diagnostics and backward compatibility.
8. CI acceptance: new contracts and end-to-end `doctor --scope project --format json` on disposable roots, Go tests, Required, CodeQL, security reviewers. Preserve native Codex/Claude/OpenCode v1/v2 separation; only merge after reviewed green checks.

## Critical review questions before code

**Q1: Can file-only proof mark a client Active?** No. The readiness result explicitly labels it `prepared` and identifies native verification as unobserved, even if a separate test has qualified the underlying CLI release. This command does not run that native test.

**Q2: Should doctor silently repair missing project config?** No. Read-only inspection only; direct users to an explicit reviewed `repair` operation with exact project-root selection, and do not invent an installation source.

**Q3: Can path or config data escape into diagnostics?** No. No absolute paths, TOML content, tokens, headers or raw client messages in published JSON. Per-project checks emit only stable code, status and bounded fixed guidance.

**Q4: Should a missing/unavailable native CLI fail materialization?** No. Distinguish client visibility from owned bytes. Readiness remains `native_unverified` unless there is concrete and safely observed file drift.

**Q5: Does a foreign project/native setting belong to UAP?** Never. Only inspect UAP-recorded object IDs in the selected project root; never follow symbolic links or touch parent/user profiles. Foreign entries and comments are not UAP-owned, and must survive the lifecycle.

## Independent critic rounds (pre-implementation)

- Round 1, correctness: never infer runtime from Prepared; avoid a default green state when native evidence is missing; make ordering deterministic.
- Round 2, security: never read or follow symlinks outside the selected project, bound TOML to 1 MiB, do not echo user-provided paths or secret values, do not mutate or launch clients.
- Round 3, compatibility: keep default command schema and existing contracts working, add new read-only fields not flags, ensure user-scope and legacy clients are unaffected, test each new classification and code on Linux/Windows path shapes.

## Scope boundary

No new agent-specific installation mechanism, account login, feature flag, release/registry change, or attempt to claim native model tool execution. PR #674 remains the separate real Codex native proof and merge gate.
