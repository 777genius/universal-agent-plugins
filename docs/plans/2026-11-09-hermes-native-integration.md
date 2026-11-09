# Plan: Hermes Agent native Skills + MCP integration

**Date:** 2026-11-09 · **Status:** Draft plan; implementation evidence is not yet claimed  
**PR lane:** Hermes only · **Base:** `main` @ `00bcdf486dabef2dc2a4e6cb239bae818a451ab0`

## Objective and acceptance contract

Deliver a first-class `--target hermes` through existing UAP client adapter contracts. Support **User Skills**, **Project Skills**, and **User MCP** with source-verified, ownership-safe add/update/repair/remove. The installer must not claim project-local MCP: Hermes' current MCP authority is `~/.hermes/config.yaml`, even when invoked from a project. Project skills require an explicit Hermes project-trust decision; UAP must not grant it. Report Prepared when native discovery or authentication is unobserved.

**Sourced behavior:** Hermes official Skills documentation confirms profile-local `~/.hermes/skills`, project `<git-root>/.hermes/skills` and `.agents/skills`, separate trust and precedence. Hermes quickstart documents YAML `mcp_servers`. Microsoft APM's `hermes.py` is a read-only reference for format, `HERMES_HOME` and secure file handling, **not** reusable Python runtime code.

- https://hermes-agent.nousresearch.com/docs/user-guide/features/skills/
- https://hermes-agent.nousresearch.com/docs/user-guide/features/mcp/
- https://github.com/microsoft/apm/blob/main/src/apm_cli/adapters/client/hermes.py (MIT; preserve notices if code is copied; prefer native UAP Go implementation)
- https://github.com/777genius/universal-agent-plugins/blob/main/docs/adr/0007-client-adapter-contract-and-registry.md

## Strict in scope

1. Extend `domain.ClientID` / `ClientDefinitions` with one Hermes identity, honest native capabilities and scope/component constraints. No changes to other clients' declared capabilities.
2. Implement `clients/hermes` behind existing optional `HostDetector`, `TargetLayout`, `ProjectTargetLayout`, `Projector`, lifecycle and exact-ownership interfaces **only as required** by existing installer; add one constructor in `clients/all`. Do not introduce a generic new client framework.
3. Detect the real `hermes` executable and profile root (document `HERMES_HOME` override). Canonicalize and contain roots before any operation. Project root is UAP's explicit selected root; ensure Hermes' native nearest Git-root behavior is respected or fail with guidance, not misplace files.
4. User Skills: project a bounded source-verified `SKILL.md` tree into Hermes-native skill discovery, preserve existing unrelated skills. Project Skills: prefer `.hermes/skills/<name>` under the exact selected git root; mark Prepared while trust/discovery remain unverified; do not modify `skills.trusted_project_dirs`.
5. User MCP: translate only supported `stdio` and `http` shapes into the `mcp_servers` mapping in `config.yaml`; preserve unrelated YAML keys, comments where safe, credentials and existing MCP entries. Use existing UAP native mutation/ownership/atomic patterns; fail closed for malformed YAML, unexpected root, symlink, key collision or unsupported transport. Protect sensitive config (owner-only mode where available); never echo credentials.
6. Explicitly refuse **Project MCP** before source acquisition. If package has Skills + MCP, disclose per-component non-support; never silently install its MCP to User from a Project command. Other artifacts (hooks, commands, extension packages) are unsupported unless proven native.
7. Integrate CLI completion/target selection and compatibility/reference documentation from existing single-source `ClientDefinitions`. Keep public installation result honest: Installed/Prepared/Needs user action, not Active without native proof.

## Deliberately out of scope

- Running Hermes models, authenticating Hermes, auto-trusting repositories, changing its own Skill Hub, modifying Hermes source.
- Project-local MCP (no documented native authority), bot/gateway integrations, patching other clients.
- New long-running daemon, new registry schema, new global YAML framework, migration of existing UAP installs, or feature flags.
- Copying APM's Python adapter into UAP; treat it as behavior reference only.

## Implementation sequence

1. **Contract first:** add definition + alias, capability tests; register adapter. Keep generic planner and usecase imports untouched.
2. **Filesystem + YAML preflight:** profile resolution, scope guard, safe parse/update plan, foreign-entry conflicts, file permissions; use existing kernel if it can preserve Hermes YAML exactly, otherwise implement a *small Hermes-local, receipt-guarded YAML document adapter* without broad framework changes.
3. **Skills projection + receipt:** use existing shared source tree staging, package digest and ownership receipts. Ensure repeat add is idempotent and repair uses recorded pin.
4. **MCP transaction:** implement exact per-server add/update/remove using the native config transaction boundary, ensuring rollback leaves an original file untouched on any failure; no writes on dry-run.
5. **Readiness + truthful UX:** report native trust requirements, no fake activation. Add docs and CLI target hints.
6. **Review + release qualification:** run all targeted contract and project tests, architecture checks, and OS matrix, then optionally a real Hermes executable with a disposable HOME and a local stdio MCP fixture. Do not require a real remote model or reveal tokens.

## Test plan (mandatory before ready-for-review)

- Contract tests: stable client ID, detected vs absent executable, independent env roots, windows path handling, immutable profile authority, project/user scope isolation.
- User Skill lifecycle: add twice/no-op, update, repair missing managed file, remove while preserving foreign skill and owner edits.
- Project Skill: Project A/B separation, git-root mismatch refusal, trust unchanged, remove from A never touches B or User.
- User YAML MCP: stdio + HTTP mappings, comments/unrelated keys retained where feasible, binary malformed/empty YAML refusal, duplicate server collision, invalid transport, symlink, non-file, non-0600, permissions, cancellation, rollback and secret-free diagnostics.
- Project MCP: negative E2E **no acquisition/no mutation**; source cannot downgrade to user.
- Native qualification (when Hermès is actually available in CI): command executed in an isolated profile and project to observe listing, selection and restoration; scripted MCP fixture only. Keep host-version bounds in evidence.

## Risk and review gates

- **P0:** user credential loss via YAML rewrite → existing file preservation, bounded parse, atomic per-key edits and malformed-file failure.
- **P0:** accidental project-to-user MCP write → explicit hard refusal.
- **P1:** Hermes git-root precedence/activation and project trust → Prepared unless directly observed.
- **P1:** unsafe relative executable/environment substitutions and output leakage → existing path policy and redacted diagnostics.
- **P1:** dependency bloat → prefer existing YAML parser if present; no new direct dependency without exact need.

## Expected code footprint

~1,000–1,800 net new lines including tests for a source-complete, careful implementation, conditional on existing native YAML and ownership APIs; revise estimate if forced to implement exact node-preserving YAML writes. **Definition of done:** installed UAP target, faithful capabilities, tests and real-platform evidence, green required CI and no unreviewed security exceptions. No automatic publication or merge without completing those gates.

### Review round 1 / 6 — native facts

**Finding and concrete plan correction:** Source truth: Hermes project Skills are scanned only at the nearest Git root, with an explicit native trust gate; User MCP lives in profile config.yaml. Split those into two distinct acceptance paths and never infer Project MCP from APM's project flag.

**Decision:** This is part of the acceptance criteria; a contradictory implementation must be rejected rather than treated as a harmless warning.
