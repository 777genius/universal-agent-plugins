# Cursor project implementation independent review R2

Reviewer: core_clients, 2026-10-23. Reviewed adapter `524b808b`, composed
public CLI lifecycle/transfer `37bbb5d7`, and corrective frozen
`f55dc4628a8ce1b671b80e82d07dd0d66011eb15`. The reviewed shared helper and
original two client-plan GO results are separate records. No production file
was edited by this reviewer.

## Decision: GO at f55dc462

One selected-data-authority finding is independently verified closed. No
remaining implementation blocker was found in the configured project component
lane or its composed source lifecycle. This does not supply native Cursor,
MCP runtime, editor or release qualification.

## R2-1: active reprojection ignored selected data authority

At `37bbb5d7`, ProjectActiveNative ignored its final dataPath parameter. Core's
activeNativeDelivery validates and supplies the selected persisted data receipt,
but the adapter returned receipts derived only from the old immutable mcp.json.
A different selected data locator could therefore silently retain the prior
launcher/environment locator instead of refusing the inconsistent projection.

Closed in `f55dc462`. The project-only read-only verifier now requires selected
stdio data to be absolute and nonempty, exact launcher/package/data environment
bindings, no native cwd field, and the exact managed launcher protocol prefix
for mode, package, data, resolved cwd, anchor and separator. It recomputes the
cwd prefix from the current standard envelope and supplied selected locators.
Inconsistent authority returns ErrProjectionRefreshRequired without rewriting
digest-covered package bytes. Remote components and the preserved user lane
retain their prior behavior. The existing positive stdio contract now supplies
its actual data locator; wrong and empty selected-data regressions require
refusal and an unchanged immutable package inventory.

## Scope, ownership and lifecycle assessment

- Exact validated project context selects `.cursor` while the original user
  config root remains distinct. Planning, active projection, activation,
  binding validation and removal fence the scoped registry and package path.
  Mutable state/data/prepared packages stay under the selected `.uap`.
- Declarative project policy uses the existing native-config lifecycle rather
  than borrowing user CLI-registry policy. User manual package preparation,
  detection and existing attestation behavior remain intact.
- Skills and existing rendered command skills use exact directory receipts;
  MCP members use the existing transactional ownership kernel. The adapter
  removes only the managed-package receipt from native component inputs and
  leaves unknown kinds visible for closed refusal. Foreign/disabled same-name
  objects collide; removal never owns an entire shared config document.
- Managed stdio uses the existing launcher and explicit package/data/cwd
  protocol. The new active check validates those bindings read-only. Source
  directories and immutable package bytes are never executed by the proof.
- Successful file configuration remains prepared plus installed verification.
  Project preflight rejects activation attestation before provider shortcuts,
  including prepared component selections and direct adapter reuse. Discovery,
  aliases, enablement, policy, approvals and MCP initialization stay unobserved.
- The composed Git/ZIP journey retains production scope composition, solver,
  locks, immutable store, planner, stager, installer and native transactions.
  Only detection and security-assessment boundaries are inert. It exercises
  saved A/B add, byte-stable locked sync, read commands without effects,
  missing-native-skill repair, moving pin update, exact portable export,
  cold-cache import/install and exact removal. Foreign user config/hooks/skills
  and project-B native/state inventories remain exact while A changes.
- Remote-only composed fixtures avoid launcher or configured-command execution.
  Separate provider tests cover stdio/data, rendered commands, no-op/update,
  withdrawn MCP, repeated removal, malformed JSON, foreign collisions, writer
  failure rollback, scope/context mismatch and contained native aliases.

## Independent final execution

Go 1.26.10, GOMAXPROCS=3, `-p=2`, bounded restored RAM toolchain/cache:

- Complete Cursor client package: **PASS 0.088s**.
- Final `^TestProjectCursor` production provider contracts: **PASS 0.124s**.
- Final public `TestCursorProjectComposedSourceLifecycle` real TLS Git/ZIP
  lifecycle and cold transfer: **PASS 7.667s**.

No Cursor process, model request, configured MCP command, package payload,
trust/approval/login change or hosted native proof was executed. Author's
corrective lint gates and the other independent review remain applicable to
the final checkpoint. Actual native discovery, fresh native restarts, editor
behavior, MCP handshake/tool invocation and release qualification remain pending.
