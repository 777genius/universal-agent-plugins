# BUG-034 — Qoder live Skill foreign sentinel occupies UAP's own managed namespace

## Evidence and impact (2026-11-04)

The retained real [Qoder Skills 30-source artifact from run 41814723215](https://github.com/777genius/universal-agent-plugins/actions/runs/41814723215/artifacts/13315321747) reports **30 actual `PLAN_REJECTED`** cases with zero installed and an empty post-failure list. The fresh hermetic `TestQoderRawSkillProjectLifecycle` from BUG-033 **passed** in the current-head `Core fast / Run install core tests` on Linux, so the normal, disposable source-to-prepared-bundle route is executable without a native model or subscription. This does not yet establish the cause of all 30 live refusals.

Source review identifies a concrete harness authority violation: `PROJECT_SKILL_ROOTS["qoder"] == [".uap/managed/clients/qoder"]` is the **UAP-managed package namespace**, not the client-native external Skill directory. Before invoking any `skills add`, `do_case` writes its `foreign-neighbor/SKILL.md` inside that managed namespace. Every Qoder source therefore begins with an unowned tree **inside UAP's own managed-client allocation**. Native Qoder project Skills are documented under `.qoder/skills`, whereas the UAP project path exports a separately prepared closed `.agentplugins-qoder-native` view. The foreign preservation sentinel must not masquerade as a managed artifact.

The collision is a **confirmed E2E fixture placement bug**; whether it explains every historical `PLAN_REJECTED` remains an **unproven hypothesis** until a new exact-head 30-case raw ZIP and independent comparison.

## Scoped minimal plan

1. Keep `PROJECT_SKILL_ROOTS["qoder"]` untouched: positive Skill identity and full removal comparison must continue to inspect the actual managed bundle namespace.
2. For **only** `target == "qoder" and kind == "skill"`, place the foreign unrelated sentinel in the disposable native project tree `project/.qoder/skills/foreign-neighbor/SKILL.md`, not under `project/.uap/managed/clients/qoder`. Reuse the existing byte-for-byte `preserved_regular_text` oracle; no new override, bypass or new staging system.
3. Add a focused no-network regression: failed mock add still must preserve that foreign Skill in the native project, and the UAP managed package base must **not** contain the injected sentinel. Existing whole managed bundle snapshot/remove oracle stays strict.
4. Compare freshly generated real Qoder Skills 30-source ZIP for `PLAN_REJECTED`, `passed` and `package-installed-native-unqualified`; only actual installation, exact source/lock, native prepared file, repeat add, verified removal and foreign-file preservation qualify a case.

## Critic review of plan (self-review, not independent approval)

- **Ownership critic:** The package manager's managed namespace is not suitable for an unmanaged sentinel. Keeping it there would make an intentional foreign collision look like an installer defect. Moving only the canary, while preserving snapshots of the managed tree, fixes the test's trust boundary without making planner more permissive.
- **False-positive critic:** A Qoder prepared bundle is **not** a Skill activated by the native client. Continue to assert exact managed bundle bytes and cancellation-safe cleanup; do not relabel `PLAN_REJECTED` or claim that the real Qoder runtime discovered the prepared Skill. If the 30 refusals remain after this fixture fix, investigate the specific next error in the same scope rather than waiving compatibility checks.

**Status:** source-backed fixture defect and scoped plan recorded. Patch, regression, and final-head real results pending.
