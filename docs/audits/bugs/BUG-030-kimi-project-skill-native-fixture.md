# BUG-030 — Kimi project Skill audit chooses an incompatible native version and lacks its Git root

## Reproduction / actual GitHub Actions evidence

The raw [Kimi Skill 30-source artifact](https://github.com/777genius/universal-agent-plugins/actions/runs/41814723215/artifacts/13313879897) for PR #698 at HEAD `97378eac646ea6f64686d37b7c564910137a918e` records 30 real source attempts, 30 `install-failed` responses, each with `PLAN_REJECTED`. The independently inspected workflow installs genuine `@moonshot-ai/kimi-code@2.4.1`, and the artifact confirms `client_version.value=2.4.1`. In the **unchanged production adapter** `install/integrationctl/agentplugins/clients/kimi/scope.go`, `currentKimi` only qualifies native **2.3.1 or 2.3.2**. A project containing Skills must additionally have the nearest real `.git` marker at the selected project root (`kimiSkillRoot`). The current audit `do_case` creates plain disposable directories with **no Git initialization**. Both are independently source-verifiable preconditions for Kimi project Skills.

These two mismatches explain why this lane cannot qualify; they do **not** establish that fixing them will yield 30 PASS or that all underlying `PLAN_REJECTED` cases share only these causes. Other client lanes and user-scope Kimi plugins must not be changed without further evidence.

## Minimal plan, scoped to E2E evidence, not production code

1. For only `matrix.target=kimi, matrix.kind=skill`, install a *published* explicit supported version `@moonshot-ai/kimi-code@2.3.2`. Keep 2.4.1 pinned for the existing user-scope **plugin** baseline; record actual observed CLI version and do not loosen `currentKimi`. The pinned version 2.3.2 is independently listed in the public npm package version history and the publisher's GitHub Releases.
2. For only a Kimi **Skill project** case, use the genuine `git init --quiet` operation in its existing disposable project root **before** the UAP add. Bind the Git executable to a canonical absolute path once before any third-party acquisition, prohibit inherited Git configs/hooks through the existing isolated env, and fail the case as `harness-error` if initialization fails. Never create a synthetic `.git` marker or modify a non-disposable root.
3. Add a minimal, no-network regression that checks real `git init` happens for Kimi Skill only, produces a real `.git` directory, cannot run outside the disposable case, and leaves unrelated targets/kinds untouched. If Git itself is unavailable, report harness failure instead of source failure.
4. Re-run focused Python tests, applicable required checks, and **real final-head Kimi 30 Skills** artifact, then review exact outcomes. No password/model subscriptions, fake client binaries, security policy exceptions, feature flags, or production compatibility changes.

## Critical plan review — two independent failure perspectives (not external approval)

- **Compatibility critic:** Changing native Kimi acceptance to accept the newer unobserved 2.4.1 would break the product's explicit version admission contract. Selecting existing documented 2.3.2 only for project Skills is smaller, preserves 2.4.1 plugin evidence, and maintains native version traceability.
- **Safety/false-positive critic:** Writing an empty `.git` directory just to satisfy a marker is a false E2E precondition. Execute the genuine pinned Git executable, with isolated HOME and no external hooks, only under the disposable case. A matching version and Git root are prerequisites, **not** evidence of a successful install/remove.

## Status

Two preconditions confirmed from real source/ZIP; narrow workflow and harness correction plus focused regressions **pending**. Preserve all 30 historical failed outcomes and do not merge on this report.
