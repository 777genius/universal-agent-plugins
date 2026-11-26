# Cursor scoped automatic MCP trust — 2026-11-21

## Initial audit
- No open Cursor **implementation** PR for scoped per-tool auto-approval. #1016 observes native Editor/ACP without implementing setup; #1017 changes generic installer UX; #931 qualifies native clients. Do not alter their branches.
- Main already installs Cursor **project** Skills and MCP entries and approves **fingerprinted** selected stdio MCP servers for exactly qualified Cursor Agent versions. The Cursor **user plugin** remains a local development preparation, not verified automatic activation.
- Cursor documents `<project>/.cursor/cli.json` `permissions.allow: ["Mcp(server:tool)"]` for CLI and `permissions.json` `mcpAllowlist` for Editor. Editor file entries override in-app allowlists and require an effective Run Mode; these are not workspace-trust APIs.

## Goal / non-goals
Finish a strictly opt-in, publisher-declared and installer-reviewed set of **exact Cursor Agent project MCP tool permissions**, with no separate interactive approval for those names after an explicit confirmed add. Reuse the existing project Skills/MCP projector and locked exact-file trust writer; preserve native disablement and all foreign configuration.
Do **not** grant whole-workspace trust, change the editor/UI allowlist, write private VS Code trust DBs, use wildcards, invoke tools during install, force-load disabled servers, turn account/login failure into Active, or alter other clients.

## Interface
Opt-in plugin manifest extension `com.universal-agent-plugins.cursor-mcp-approval`:
```json
{"version":1,"mcp_auto_approve":{"selected-server":["exact_tool"]}}
```
Names are constrained visible ASCII; exact server must be selected and projected from the same package, and the version must be 1. The plan lists each approved server/tool. Only an explicit add with `ProjectTrustConsent` writes the selected project `.cursor/cli.json` (Cursor Agent CLI, not the Editor). The preference persists after remove, as other native project trust preferences do, and is disclosed; it can be revoked manually. Updates and repair must not silently re-grant revoked tool permissions.

## Three plan reviews
1. **Architecture / DRY:** reuse `ExtensionInterpreter`, `Projector`, `ProjectComponents`, `UpdateProjectTrustFile`; no generic `ClientID` branches, second lifecycle or schema change.
2. **Safety:** exact names, no wildcard/glob/control/bidi, no foreign server, no wrong root; reject conflicts with deny/ask policy and unknown document formats; conditional write, no-follow, idempotence, cautious errors/effect accounting.
3. **E2E:** explicit consent, no-consent update/repair, User/Project isolation, A/B roots, missing/foreign/disabled config, same/updated package, verifier only, failure rollback and native lifecycle; no false assertion of auth, model tool calls or Editor trust.

## Estimates / exit criteria
Estimate ~250–450 Go production LOC and ~250–450 focused tests/docs, depending on shared helpers; complexity 5/10, expected reliability 8/10, pre-CI confidence 7/10.
Go tests, lint, vet, generated-check, relevant exact-HEAD Linux/macOS/Windows Cursor no-auth install and genuine native lifecycle, security checks, independent review and merge/postmerge must pass before declaring complete. Queued/skipped/cancelled ≠ PASS. No release or unrelated CI changes.

## Final audit corrections — 2026-11-26

The Cursor implementation and its genuine pinned native lifecycle are already
qualified on `9c009d86`. Finish the selected shared audit failures without changing
another client's production implementation:

1. Align Kiro fixtures and the installation observer with explicit Prepared intent
   and the existing incomplete native verdict; reject a rejected exact-source
   receipt instead of admitting it through a generic fallback.
2. Give the Windsurf Global sentinel one fixture owner, retaining the existing
   exclusive-create refusal and the offline local profile configuration.
3. Assert both the historical Codex observation label and its archived lifecycle
   outcome; historical observation remains separate from current native readiness.
4. Observe Pi's own component inventories before add, after add, after repeat and
   after removal. Require exact component names, native source paths and unchanged
   owned/foreign bytes in addition to existing signed-source lifecycle checks.
   This qualifies registration only. Preserve public Prepared states and report
   connection failures; authentication, model calls and MCP tools/call remain false.

Review 1 narrowed the work to existing audit seams and one small Pi observer;
no installer changes, broad cache exclusions or duplicate lifecycle engine.
Review 2 retained separate installation/registration acceptance and default native
failure states, and required negative controls for wrong provenance, missing intent
and foreign-state mutation. Estimated addition: 200–300 audit LOC and 100–180
focused test LOC; complexity 4/10, expected reliability 8/10, confidence 7/10
before genuine execution. Reuse successful evidence for unchanged source, verify
the prospective merge with current main, then ordinary guarded merge and postmerge.

## Integration with main `61542d3e`

The shared audit merge preserves main's source and saved-state custody checks,
Cursor Skill tree identity checks, Claude removal discovery, and Windsurf's
independent projection oracle and four-case native gate. It also retains this
branch's stricter unchanged-file identity checks, guarded Codex initialization,
Pi registration-only observations and historical Codex verdicts. Reconcile the
fixtures with those combined contracts; do not choose one parent's audit wholesale.

The Cursor production implementation is unchanged by this integration. Its pinned
native evidence remains tied to `9c009d86`; current shared-harness contract checks
are additional evidence, not a new genuine native run. Likewise, the local real
Pi inventory experiment covers one representative source, not the complete
30-source signed catalog or authenticated MCP calls.

Merged local verification: 398 common-audit tests passed with 12 platform skips;
Prepared 21, Kiro 31, Windsurf 48, Pi registration 10 and Windsurf cycle 26 passed.
CI path-policy checks and the affected Marketing Kit regression contracts passed.
Independent review found no lost gates in the resolved audit unions. A resource
termination during the common suite was handled by running only its unfinished
tests in bounded processes; that interrupted process itself is not recorded as PASS.
