# UAP vs APM: high-risk E2E and product decisions

**Date:** 2026-11-01. **Stage:** pre-implementation plan, review required. **Scope:** independent evidence capture and recommendations; no product behavior or provider policy changes in this PR.

## Decision and objective

Compare **UAP and Microsoft APM on the same *safety invariants***, not by number of CLI commands or GitHub issues. The completed [Agent Skills 1.0 baseline PR #617](https://github.com/777genius/universal-agent-plugins/pull/617) confirmed three Codex project-file delivery scenarios for both tools, and preserved foreign bytes during refused collisions. It did **not** establish correctness for interrupted operations, full plugin installation, other agents or credentials. This work adds narrowly scoped destructive-regression evidence and uses findings to choose UAP improvements.

Our product is a **standard Agent Plugins 1.0 / Agent Skills 1.0 lifecycle manager across heterogeneous agents**, with a fully local first release and optional management API. Preserve the existing Go installer as the ownership/recovery authority; UI and TypeScript SDK must not implement independent writes. Do not add a second DI system, background daemon, policy engine, framework, new storage table or feature flags in this work.

## Evidence and references

- [UAP project/real-client support matrix](../../PROJECT_CLIENT_SUPPORT.md): many project adapters are **Prepared**, not proven Active; vendor OAuth and arbitrary model runs remain unqualified.
- [PR #617](https://github.com/777genius/universal-agent-plugins/pull/617) / [hosted evidence run](https://github.com/777genius/universal-agent-plugins/actions/runs/41236778408): both products PASS install/repeat/cold locked restore for a single raw Skill in real Codex project scope. UAP collision returned generic `PLAN_REJECTED`; APM named a foreign destination. Neither collision may be counted PASS without independent cause admission.
- [APM v0.35.0](https://github.com/microsoft/apm/releases/tag/v0.35.0), [frozen lockfile contract](https://microsoft.github.io/apm/reference/lockfile-spec/), [install contract](https://microsoft.github.io/apm/reference/cli/install/). APM has a broader manifest for instructions, hooks and MCP, plus corporate governance.
- Unresolved, **reported** APM risks to examine, not pre-labeled product FAILs: [cold skills deletion #3391](https://github.com/microsoft/apm/issues/3391), [Claude profile crossing #3423/#3425](https://github.com/microsoft/apm/issues/3423), [half-applied failure #3373](https://github.com/microsoft/apm/issues/3373), [Codex MCP env plaintext #3393](https://github.com/microsoft/apm/issues/3393), [transitive stale update #3392](https://github.com/microsoft/apm/issues/3392), [post-pull orphan #3188](https://github.com/microsoft/apm/issues/3188). Reproduction reports vary in independent corroboration and target version.
- Established industry behavior: [npm ci](https://docs.npmjs.com/cli/commands/npm-ci/) fails on manifest/lock mismatch and **never** edits lockfiles; [APM frozen](https://microsoft.github.io/apm/reference/cli/install/) has a similar declared goal. Neither contract alone proves file ownership, atomicity or cross-profile isolation.

## Threat model and universal oracle

A package manager modifies agent-readable files and configs. An unexpected **exit 0 with stale/missing primitives**, cross-profile deletion, unacknowledged mutation on failure or secret printed into committed configuration is worse than a loud refusal. For each scenario, observer independently snapshots **owned + foreign + sibling profiles + portable lock** before/after; uses SHA-256 and bounded regular-file reads, forbids symlink traversal and reads only named fixture roots. An operation that cannot be invoked on a *supported* product route is `UNSUPPORTED` with a contract citation; an unavailable external prerequisite is `BLOCKED`, not FAIL; a scenario not executed is `NOT_RUN`.

- `PASS`: actual released/pinned CLI ran, all invariants and provenance predicates hold.
- `FAIL`: the named invariant was violated with direct source-bound filesystem / process evidence.
- `BLOCKED`: setup, permission, trust, missing genuine client or indeterminate outcome prevents attribution. Include the failure stage.
- `UNSUPPORTED`: source-documented absence of a comparable capability; never silently substitute raw Skill installation for whole-plugin deployment.
- `NOT_RUN`: no execution or no deterministic injection; never score it as PASS.
- One tool’s success must not certify the other; an evidence-only green Actions run means receipts were generated, not that products passed.

**Comparable is an outcome, not necessarily command syntax.** UAP may require a genuine, installed client and persisted `uap.toml/uap.lock`; APM may install files without a client binary using `apm.yml/apm.lock.yaml`. Use each released native route, but provision genuine pinned clients for both when that prerequisite matters. Always record such differences. Project and user scope are different lanes, never blended.

## Six parallel lanes, explicit acceptance

| ID | Risk and independently observed oracle | Scope / eligibility |
| --- | --- | --- |
| R1 | **Cold clone without receipts**: seed exact lock and committed delivered Skill; recreate clean HOME/state/cache; run frozen restore. Require no committed file deletion, correct bytes and unchanged portable lock. Specifically test the APM `claude_skill` / `skill_bundle` path, not only `apm_package`. | Claude project, real declared source; initial pinned fixture. Cold-clone and missing-receipt are distinct controlled subcases. |
| R2 | **Claude user-profile A→B→A and selected uninstall**: same ordinary user HOME, two different disposable `CLAUDE_CONFIG_DIR` roots, exact foreign sentinels, state retained across commands. Any mutation outside selected profile is FAIL even when exit status is 0. | User scope; UAP target detection must be *genuine Claude Code* and current profile support documented; if not eligible classify, do not imitate with fake clients. |
| R3 | **Transitive update**: parent v1 pins child A, parent v2 pins child B. Verify selected parent commit, resolved lock graph and actual child bytes are all B after one explicit update. Exit 0 with child A is FAIL. Also test intentional removal of child from parent. | Both tools **only if** released UAP accepts this transitive package relationship; otherwise UNSUPPORTED + separate UAP single-level update contract, no false parity. |
| R4 | **Mid-operation failure**: after a proven write barrier, terminate process or inject an actually deterministic failure; reopen state and invoke documented recovery. Verify mutation report, no silent unowned write, recorded partials, idempotent retry. | No arbitrary sleep/kill claiming a particular commit point. If no stable external barrier: BLOCKED/NOT_RUN with honest reason; test controlled preflight failure separately, not mislabeled as post-write atomicity. |
| R5 | **MCP secret variable**: inert stdio MCP declares `\${UAP_APM_SECRET_CANARY}` in config; install while the environment holds a unique *synthetic* value; inspect generated Codex `.codex/config.toml`, user and project files, lock and report. Plaintext literal canary anywhere outside the intended ephemeral process env is FAIL, also track whether native consumer supports env forwarding. | Real Codex profile, no OAuth, no network/tool call. A literal `\${VAR}` incorrectly passed to runtime is a distinct functional failure. Secrets MUST NEVER appear in artifacts; observer stores only booleans and hash of synthetic canary. |
| R6 | **Removed dependency after git pull**: author installs two distinct dependencies and commits manifest+lock; teammate installs; author removes one and commits; teammate pulls exact tracked files; run ordinary install/audit/prune as documented. Former managed Skill must be removed or **explicitly reported as an orphan**; it must not remain silently agent-visible. Other owner / teammate edits remain untouched. | Project scope, two immutable sources; retained local ownership on teammate; execute literal `git commit`/`git pull` with disposable identities. Different available cleanup semantics are reported separately. |

The R1 path should include a genuine APM `claude_skill` fixture and **not** mistakenly reuse #617’s passing Git-subpath case. R3 and R6 use separate fixture repositories/commits. R4 needs proof of timing instead of a slow-host sleep.

## Bounded executable architecture

1. **One Python 3.13 observer** in `scripts/comparisons/`, composed from *existing* `run_process`, `isolated_env`, regular-file/ownership observers and immutable receipts in `uap_apm_skills_e2e.py`. No product-specific logic hidden in common result classification.
2. Minimal `UapAdapter` / `ApmAdapter` command builders for each *eligible* lane. Each declares its exact source/target/scope and the planned outputs. Do not add a general package-manager framework, extension system, stateful mock provider or new application runtime.
3. One **path-scoped** GitHub Actions workflow dispatching **three independent scenario groups in parallel** (R1/R2; R3/R6; R4/R5), each with isolated runner HOME, sandbox and explicit per-case time/byte limits; selected test may be skipped only as documented UNSUPPORTED/BLOCKED. A single shared source-pinned tool setup artifact is acceptable only if validated at consumption. Avoid redundant full repository CI jobs and unattended periodic schedules.
4. Pin UAP to release 0.1.95 source SHA `ec85ee7d759944b9799e451c96bfc1859f9d5533`, APM to 0.35.0 wheel/digest from #617, and real client binaries to previously verified hashes. **Do not compare PR HEAD UAP to released APM**. If a newer actual release is selected, use a separately labeled second lane and pin both products.
5. Evidence: per-product and per-case JSON with status, exit/timeout, exact command without secrets, receipt paths, input+output hashes and provenance, plus Markdown matrix. Retain sanitized bounded artifacts on failures (`if: always()`). No full runtime dumps, user profile paths, credentials or secret values. Child cleanup and no detached-process guarantees must be explicit limitations.
6. Fail-closed observer unit tests for missing baseline, symlink ancestor, wrong lock identity, false PASS from exit 0 without payload, process timeout, foreign mutation, leaked secret, inconsistent success report, and a `BLOCKED` scenario never increasing pass counts.

## Product direction and minimal decisions

**Priority 0 — Correctness and user safety**
- Preserve UAP’s own current transaction/recovery ownership semantics; add **E2E proof**, not another state database.
- [#629](https://github.com/777genius/universal-agent-plugins/issues/629): type only a **confirmed** foreign-file collision into a useful `SKILL_DESTINATION_OCCUPIED` human message; no unsafe implicit `--force` or masking unrelated errors. Reuse existing typed errors; JSON schema remains stable unless versioned.
- For local/multiple profiles: target identity MUST be `(client, scope, canonical root, user authority)` and be checked before write/replay/cleanup. Do not take an APM global-lock schema for granted.
- Every successful return must imply fully observed required *target components*, or explicitly `Prepared` / partial with actionable next step; never report native execution from file-copy evidence.

**Priority 1 — Capabilities where APM is stronger**
- Existing `uap.toml` + lock remains authoritative for package and raw Skill dependencies. Consider attaching standardized authoring *inputs* (instructions/agents/hooks) only as **explicit, target-qualified components** where the standard or vendor adapter supports them, instead of a parallel universal `apm.yml` clone.
- A **read-only** `deps doctor/audit` command may eventually combine exact lock-to-disk drift, orphan detection, native effective-selection diagnostic and report per-target provenance. Prefer extending current `doctor`/inspection APIs; do not create a second inspection database or background reconciler.
- Enterprise source/policy control is useful only after there is a concrete org requirement: integrate **adapters** to external approval/registry policies using existing CLI preflight, not a homegrown rule language with inheritance/bypass semantics in v1.
- Expose status/errors via existing management API + CLI and reusable TS client, so desktop/web share one source of truth.

**Priority 2 — UX and migration**
- Progressive disclosure: one human summary + precise reason/next action + optional details, machine outputs unchanged.
- APM-to-UAP migration should be *an explicit dry-run preview / import assistant*, **never** silently reinterpret `apm.yml` as `uap.toml`. Preserve unsupported primitive list and source identity.
- No automatic vendor auth, trust bypass, model execution, silent forced repair or foreign-content adoption.

## Three pre-code critical lenses and exit criteria

**Round A — standards / fairness:** confirm each candidate is genuinely representable by BOTH released products; split shared outcome and product-only reach; no false equivalence of whole Agent Plugins to raw Skills. Reject invented UNSUPPORTED outcomes.

**Round B — filesystem / safety:** challenge receipt recreation, delete-owner identity, symlink/race, getenv secrets, cross-profile roots and SIGKILL injection. If safety not provable by outside observations, narrow claim and status.

**Round C — product / simplification:** challenge the necessity of each new abstraction, runner/job and feature. Reject new policy DSL, daemon, database, feature flags, duplicated E2E framework and unrelated product rewrites. Prefer 6 exact scenarios over a combinatorial cross-client test matrix.

Independent external review: CodeRabbit for protocols/assertions/failure classification, Strix for executable shell and trust boundaries, final real-run receipt review. Re-review amended exact HEAD after changes. Unit tests on high-risk oracle only, E2E on the six externally visible routes. Publish only verified results, request merge only after applicable exact-head CI and security review.

## Explicitly deferred

Agent Plugins whole-package competitive scoring; real model/OAuth; thousands-of-agents orchestration; organization IAM; desktop migrations; native install on every client/OS; generalized chaos framework; broad feature parity; mutable-source live network attacks; automatic release/npm publication. These are separate work with separate evidence and approvals.

**Exit:** six independently accounted lanes per tool (`PASS/FAIL/BLOCKED/UNSUPPORTED/NOT_RUN`, never silently omitted); evidence reviewed, reproducible hashes + exact tool versions, zero destructive effects outside disposable roots; product priorities and validated questions documented. Any remaining unqualified scenario stays explicitly open rather than declared complete.
