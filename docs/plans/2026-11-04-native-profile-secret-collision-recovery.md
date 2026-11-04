# UAP native lifecycle qualification: Claude profiles, Codex MCP, Skill collisions, crash recovery

**Date:** 2026-11-04. **Base:** `87b769a8f546b0bb1c25c4c34422f3f756995697`. **Status:** draft pre-code plan; research/spikes first. **PR:** one bounded PR, no release.

## Goal and acceptance

Close the remaining **proved**, supported-route gaps in [#641](https://github.com/777genius/universal-agent-plugins/issues/641), [#642](https://github.com/777genius/universal-agent-plugins/issues/642), [#629](https://github.com/777genius/universal-agent-plugins/issues/629), and CLI-level post-write crash evidence **without weakening safety**. A verified unsupported/native limitation must remain an honest BLOCKED/UNSUPPORTED outcome, not a fabricated product PASS. Keep unrelated PRs, fixes and release policy unchanged.

## Constraints and existing ownership

- Read `AGENTS.md`, accepted [ADR 0005](../adr/0005-standard-first-agent-plugins-installer.md), [support](../SUPPORT.md), [project matrix](../PROJECT_CLIENT_SUPPORT.md), and `install/integrationctl/agentplugins/installer/README.md` before changes. **Do not edit VS Code** implementation/tests/fixtures. Do not modify legacy `plugin.yaml` authoring, portable Agent Plugins 1.0 interpretation, signed Registry or landing.
- Reuse Go `installer` + `transaction` journal, `nativeconfig` ownership kernel, selected `clients/claude`/`clients/codex` adapters, and existing CLI/Management error path. No second manager, secrets database, owner DB, daemon, feature flags, universal template substitution, new policy DSL or blanket test harness.
- Scope is a single repository PR with **separate reviewable commits** (plan/spikes, independently justified fixes, E2E). Every commit is pushed immediately; leave draft until **all** exact-HEAD gates pass. A green evidence-collection job alone is not a feature PASS.
- Source/target labels distinguish published UAP **0.1.95** (historical probes) from current source; pin genuine Claude **2.1.263** and Codex **0.164.0** or label a deliberately updated pinned native version. Never use host user profiles or vendor credentials.

## Verified baseline and unproved hypotheses

| Lane | Established, do not reimplement | Specific missing proof |
| --- | --- | --- |
| Claude #641 | Historical real A→B→A `skills add`: A succeeds, B refuses indeterminate, A replay unchanged; no A overwrite. Native list parser requires exact `id/scope/enabled/installPath`. | Which B-created files changed after refusal and whether native list is malformed, sibling-visible, or genuine conflict; safe B install, selected B remove/recover. |
| Codex #642 | `nativeconfig.CodecCodex`, project `.codex/config.toml` owner receipts; existing real `TestAgentpluginsCodexNativeTrustedProjectMCP` does direct `initialize/tools/list/tools/call`. | Need/authority for **explicit Codex-only stdio variable-name forwarding** and proof of actual MCP child receiving a random synthetic canary without persistence. |
| Skills #629 | Existing `FOREIGN_DESTINATION` guidance for `nativeconfig.ErrCollision/ErrNotOwned` in PR #678, foreign byte preservation in #617 and later cross-target tests. | Exact source-level reason of released `PLAN_REJECTED` for a **Skill directory**; whether generic, untyped project `requireAbsent` error is lost before picker, and whether current main already fixes it. |
| Crash | PR #654 process exits at *durable post-activation/pre-receipt* fault and recovered kernel reinstalls correctly; `native_recovery` and Management restart coverage exist. | Uncovered **post-state-commit / journal-finalization** transition and real CLI restart/recovery receipt/status; no proof of arbitrary power loss. |

Sources: #633/#634 review protocol, #646 four initial gates, #654, #678, #679, #700, #624 and the published `uap-apm-risk-*` artifacts. Native config docs: Codex `mcp_servers.<id>.env_vars`; Agent Plugins 1.0 §9.2 expands **only** `${PLUGIN_ROOT}`/`${PLUGIN_DATA}`, preserving other placeholders literally.

## Spike-first decision gates (run with disposable HOME, XDG, UAP state and native roots)

1. **C1 / Claude** — independently read sanitized `claude plugin list --json` under A and B, before and after native probe and UAP invocation; record exact selected root, parsed ID/scope/path, exit code, *file names + digests only* of new B files. Compare A/B physical paths and per-profile `AGENTPLUGINS_HOME`, distinguish **Claude CLI initialization** from UAP effects. Expected result: safe supported cause identified or explicit unknown/blocker. No code fix until this passes. Confirm cleanup/removal only in B if B acquires an owned receipt; never authorize deletion from A's saved receipt.
2. **D1 / foreign Skill** — reproduce the pinned project Scope Codex foreign `.agents/skills/benchmark-note/SKILL.md` with real released UAP and current source in separate roots. Trace the **typed** refusal from `ProjectComponentMutation.requireAbsent` through `PrepareSkills`/review controller; compare actual CLI and picker/JSON outputs. If conflict confirmed but untyped, wrap a dedicated existing typed collision or add **one narrow domain sentinel**, map it once in the existing CLI/Management error boundary. Preserve generic refusal for unrelated errors; never forward untrusted provider strings. No `--force`, foreign adoption/deletion or public schema break.
3. **M1 / Codex** — reuse the actual existing Codex native direct-MCP RPC fixture. Prove project-local `env_vars = ["UAP_TEST_CANARY"]` is read by pinned trusted Codex and a controlled inert stdio child. Scan bounded non-symlink roots for raw/base64/hex canary; record boolean/digest only. **No product feature unless a concrete supported UAP opt-in host caller exists and the trial passes.** If justified, pass strictly validated env **names only** through existing target-qualified request/extension/Management seam to `nativeconfig.CodecCodex`, not portable `mcp.json`, and bind selected server + profile. No value persistence, unknown-token interpolation, or unsupported nonlocal env source. Missing ambient value must be clearly Prepared/Blocked, not silently echoed.
4. **R1 / crash** — inventory exact existing `dirswap` fault phases and current four-platform catalog coverage before adding tests. Use a real separate OS process deterministically exiting **after** durable state commit but before journal/backup finalization if not covered. Reopen entirely from disk and invoke existing `Recover`; assert old/desired decision, byte and ownership facts, foreign neighbors, journal cleanup, idempotent clean retry. Add only a **real CLI/Management restart** observation if an existing test-only barrier can prove the phase without a production testing hook. Otherwise keep this sub-gate BLOCKED, never time-based `sleep; kill` pseudo-proof.

### Acceptance for each lane

- **Claude:** A→B→A with truthful native identity, both scopes/paths bound, no foreign or sibling changes; B-only remove + reopen if supported; a refusal remains fail-closed with actionable reason and unchanged foreign content. Capture negative malformed/symlink/duplicate-list tests.
- **Codex:** existing direct-MCP initialize/tool-call kept; explicit name forwarding only with proven consumer need, invalid/missing names negative, selected profile and source pin unchanged, reinstall/remove preserve unrelated entries; raw and common-encoded canary absent from files/artifacts; no model/OAuth call.
- **Collision:** observed cause is exact foreign destination; a targeted *typed* human explanation instead of generic `PLAN_REJECTED`; preserve existing noncollision codes/JSON compatibility, byte identity and negative symlink/race cases.
- **Crash:** true post-write child death (not mock error), full re-open, state/receipt/journal and native neighbor assertions, recovery idempotence; no automatic unknown-state rollback or false `Active`.

## Review checkpoints — two rounds **before code**, repeated post-code

- **Critic round 1 (contract/security):** challenge spec conformance, profile authority, secret environment inheritance, path/symlink/hardlink and cross-OS cleanup, and whether test claims exceed observations. Reject unsafe or nonrepresentable cases.
- **Critic round 2 (architecture/E2E):** independently challenge duplicates against #654/#678/#624/#700, native fixture provenance, missing negative controls, PR size, use of new layers, and assumptions that `Prepared` implies loaded. Resolve each actionable issue in the plan before authorizing production edits.
- After each implementation wave, review actual **current diff**, run focused and cross-platform tests, request CodeRabbit/Strix scrutiny and recheck every finding against source. Fix valid findings; record reason for nonapplicable ones. **Do not claim an external agent review until one actually appears.**

## Implementation ordering and scope budget

1. Commit this plan and pre-code review dispositions on one draft PR; request outside critique.
2. Execute C1/D1/M1/R1 with existing scripts/CI/fixtures; append only concise reproducible evidence and status (PASS/FAIL/BLOCKED/UNSUPPORTED/NOT_RUN).
3. Implement only verified corrections to existing Claude identity, Skill conflict projection or Codex-specific selection; **skip speculative code**. Extend existing tests rather than copy the comparison framework. Keep production changes small and independently justified.
4. Repeat native/disposable and four-platform E2E on **final PR HEAD**. Minimum: `go test ./...`, `make lint`, `make test-install-compat`, portable `scripts/package-manager-e2e.py`, native Claude/Codex project lanes where relevant, Required/CodeQL and independent feedback. The full suite may run on hosted CI where runtime is unavailable locally; preserve proof by exact SHA.
5. Resolve review comments and rerun changed-head checks until no blocking findings. Squash merge to `main` only if every affected feature is proven or explicitly left out as a documented unsupported scope, checked HEAD/base are stable, and **all required** jobs pass; verify post-merge state. Never publish a release implicitly.

### Explicit non-goals

No blanket multi-client rollback, arbitrary power-loss guarantee, vendor login/model calls, new UI flows, changes to published support claims without real proof, new cross-provider secret format, modifications to open unrelated PRs, or changes solely to make a benchmark green.

### Stop/re-scope rule

If a spike reveals that the desired feature requires a new manifest/state schema, adopting foreign files, cross-profile ownership inference or a new standalone framework, **do not implement it in this PR**: retain the existing refusal and report the precise blocker. A known safe refusal is preferable to a false successful install.
