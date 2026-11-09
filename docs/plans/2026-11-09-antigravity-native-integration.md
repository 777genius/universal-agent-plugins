# Plan: Google Antigravity native plugin integration

**Date:** 2026-11-09 · **Status:** Draft plan; no native activation claim  
**PR lane:** Antigravity only · **Base:** `main` @ `00bcdf486dabef2dc2a4e6cb239bae818a451ab0`

## Objective

Add `--target antigravity` to UAP for **real Antigravity-native plugin bundles** (Skills, declared MCP, agents/subagents, rules, and hooks only where their formats and UAP permission model match), and safely support the documented User and workspace MCP/Skills surfaces. First-class means add/update/repair/remove via UAP ownership and a truthful activation status, not simply writing `mcp_config.json` or emitting a synthetic plugin name.

**Ground truth:** official documentation describes a `plugin.json` manifest, strict validation, `agy plugin` CLI lifecycle and separate `.agents/mcp_config.json` workspace / `~/.gemini/config/mcp_config.json` User settings; MCP remote schema needs `serverUrl`. APM's Antigravity adapter implements a thin Gemini-derived **MCP-only** path; it is insufficient for native plugin parity.

- https://antigravity.google/docs/build-plugins
- https://antigravity.google/docs/marketplace?tab=cli
- https://www.agy.dev/docs/cli/mcp/
- https://github.com/microsoft/apm/blob/main/src/apm_cli/adapters/client/antigravity.py (MIT; behavior reference)
- https://github.com/777genius/universal-agent-plugins/blob/main/docs/ARCHITECTURE.md

## Scope and safety limits

1. One new `domain.ClientID` / honest `ClientDefinitions` entry, `clients/antigravity` capability-based adapter and `clients/all` registration. Do **not** change how Gemini target behaves or add vendor-specific switches in generic planners.
2. Detect `agy` and native home and distinguish CLI vs editor; ensure profile identity and version evidence are preserved. No optimistic detection from a directory alone.
3. Native User plugin packaging: convert UAP-validated package tree into the documented Antigravity plugin layout, with generated manifest and supported components only. Use existing staging/package store, never synthesize arbitrary executable hooks from text. Validate locally against approved schema and `agy plugin validate --strict` if available, in a disposable sandbox.
4. Native plugin installation: use the *documented CLI* on a scoped/owned bundle if and only if its behavior can be verified without adopting or replacing foreign plugins. Keep precise install identity and a reversible effect receipt. Otherwise report Prepared + explicit manual commands. Never claim Active from copied bytes.
5. Project scope: separate **project-native Skills / workspace MCP** from **User native plugin registration**. A project-only request must not register or modify User plugin state. Honor documented `.agents` rules, trust, existing files and source-owned entries; refuse unsupported project hooks/plugins explicitly. Do not auto-create opt-in directories unless native contract proves safe.
6. MCP mapping: distinguish stdio and remote fields (`serverUrl`), preserve foreign entries using existing exact-member native config operations, disallow unsafe/unrecognized transforms, and keep secrets private.
7. Update CLI target UX, compatibility/reference/docs and the single capability matrix. Disclose untested vendor authentication and UI enablement.

## Explicit non-goals

- Implementing a new Antigravity Marketplace client/credentials/OAuth, auto-enabling hooks, granting workspace trust or bypassing vendor approval.
- Rewriting UAP's package schema or inventing an abstract plugin-converter framework before contract exists.
- Migrating existing Gemini/UAP installs, mutating `~/.gemini` on project requests, installing arbitrary shell hooks without approval or enabling feature flags.
- Treating APM's `antigravity.py` MCP-only adapter as equivalent to Antigravity native plugins.

## Implementation milestones

1. Add identity, discovery and capability tests with separate User/Project policy. Keep project-native plugin registration **unsupported until proven**.
2. Build bounded, schema-validated converter for UAP source components. Preserve relative paths, name identity, source digests, permissions and executable inventory; reject unknown required components rather than drop silently.
3. Compose with existing UAP staging, user consent, ownership receipts and transaction/recovery. Decide documented CLI invocation vs prepared handoff based on native test; no untracked side effects.
4. Add project Skills and MCP only through a safe explicit workspace authority, foreign-file preservation, reversible receipts and opt-in semantics.
5. Add real `agy` sandbox E2E: validate/install/list/enable state as applicable/update/uninstall, restart/config verification, A/B projects; no real model/API login.
6. Document supported/unsupported features and user actions accurately; ensure website/signed Directory does not advertise a component as native merely because the converter can prepare files.

## Required test matrix

- Adapter/detection/profile contract under Linux/macOS/Windows paths, no global mutation when absent or in Project scope.
- Plugin manifest schema; each supported component; malicious names, traversals, symlinks, executable hooks, oversized payloads and unsupported component hard refusals.
- Idempotent full cycle and interrupted operation rollback, foreign user plugins untouched, cleanup of exact UAP-owned native state only.
- Project Skills/MCP A/B isolation; config JSON merge and comments handling where documented; project configuration opt-in; disallow cross-scope fallback.
- Real Antigravity CLI smoke with disposable profile: **validating the staged plugin alone does not establish runtime activation**. Native list/enable and retained state must be observed.
- CI suite on all three desktop OS platforms where distribution exists, otherwise document unqualified native lanes and keep PR Draft.

## Review blockers / decisions

- **P0:** distinguish a *plugin managed by Antigravity* from loose MCP configuration. Different ownership contracts.
- **P0:** documented `agy` install and uninstall side effects must be observable, not assumed reversible.
- **P1:** external hooks can execute code; avoid silently registering/enabling them or leaking secrets.
- **P1:** User plugin directory may coexist with Gemini paths; no ownership crossing or replacement.
- **P1:** permissions/credentials in remote MCP; never serialize sensitive values into log/evidence.

## Estimate and done

~1,800–3,200 new lines including fixtures/E2E if native CLI API and existing UAP staging can be reused; if vendor lifecycle lacks safe uninstall/update semantics, stop at documented Prepared status with exact limitations rather than claim complete native parity. Done requires genuine native validation, mutation-proof fixtures, review, green CI and evidence. No forced source migrations.

### Review round 1 / 6 — native facts

**Finding and concrete plan correction:** Source truth: agy native plugin lifecycle is distinct from workspace MCP; APM's 55-line adapter targets only MCP. Define separate acceptance oracle for plugin validation, installation, discovery and safe removal.

**Decision:** This is part of the acceptance criteria; a contradictory implementation must be rejected rather than treated as a harmless warning.

### Review round 2 / 6 — scope

**Finding and concrete plan correction:** Scope: native plugin installation may be User-authoritative; explicit Project requests may only deliver documented Project Skills/MCP, and must never register a User plugin as a fallback. Optional .agents creation requires verified client behavior.

**Decision:** This is part of the acceptance criteria; a contradictory implementation must be rejected rather than treated as a harmless warning.

### Review round 3 / 6 — Clean Architecture / DRY

**Finding and concrete plan correction:** Architecture: a single Antigravity adapter composes existing UAP staging and ownership; a small pure converter owns plugin.json projection, not a new generic compiler or forked Gemini adapter.

**Decision:** This is part of the acceptance criteria; a contradictory implementation must be rejected rather than treated as a harmless warning.

### Review round 4 / 6 — security and ownership

**Finding and concrete plan correction:** Security: hook execution and native agy shell side effects are not under UAP's transaction kernel. Do not auto-enable or install untrusted hooks without explicit consent, and never advertise atomic rollback for external vendor operations unless it is observed. Reject path collision, unexpected CLI output or foreign plugin ownership.

**Decision:** Treat this constraint as a release gate. Do not replace missing native proof with mocked success or expand the architecture just to claim feature parity.

### Review round 5 / 6 — test oracles

**Finding and concrete plan correction:** E2E: one genuine agy version, isolated HOME/project, staged manifest validation, CLI install/list/disable/remove/restart and controlled MCP handshake where feasible; compare native inventory before/after, check foreign sentinels; on unavailable native runner keep Draft and mark lane unqualified.

**Decision:** Treat this constraint as a release gate. Do not replace missing native proof with mocked success or expand the architecture just to claim feature parity.
