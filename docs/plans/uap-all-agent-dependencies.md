# Dependency-managed installer for all 13 in-tree adapters

Base: `c332eb37787f408dfe2ca3c1439561a63435e767`.
Planner: Opus 5.5 (`claude-opus-5-5`, xhigh). Implementer: owner-authorized
Codex fallback after `mimo_project_bridge_scope_denied`; the installed runtime
required a different exact canonical controller account set. No admission or
credential boundary is bypassed.

## Scope and preservation

Account for every adapter in `clients/all.Default()`: eleven qualified local clients
participate in saved dependencies, ChatGPT retains its existing manual
preparation workflow, and new VS Code dependency qualification is explicitly
deferred by the owner. Claude, Codex, OpenCode, Cursor and Gemini are the priority. This does not claim automatic remote installation in
ChatGPT or qualification of running native clients.

The supported user-scope workflow shares `uap.toml`, `uap.lock`, immutable package
storage, ownership receipts and installer transactions across `deps
init/lock/sync/update/status/recover` and saved `add/update/remove/repair`.

Preserve `docs/AUTHORING_CAPABILITY_PRESERVATION.md` and the owner decisions in
`docs/STANDARD_FIRST_AUTHORING_ENGINE_IMPLEMENTATION_PLAN.md`. Do not delete
legacy/YAML implementations, introduce YAML fallback, add an agent or a second
engine, change persisted store/manifest/lock schemas, enable project scope,
register `vscode.NewLocal`, or expand separately qualified local-file routing.
Existing group, historical-removal and OpenCode group restrictions remain.

No commit, push, npm/site publication, release qualification or authoring work is
part of this implementation lane. `SAVED_DEPENDENCIES_CLI_RELEASED` stays `false`.

## Support and host authority

`installer.ClientSupported(registry, id)` requires a registered adapter and a
known definition that is neither hostless nor personal-mapping-only, and whose
domain traits do not defer saved dependency qualification. CLI saved target
selection and the generated landing capability artifact use this authority.
`Engine.SupportsClient` and `Discover` preserve explicit SDK registry membership,
including legacy VS Code APIs; saved qualification cannot disable them. Generic installer packages never import
the all-client composition package. The CLI composition injects the same canonical
registry into the installer that it already uses for planning and detection.

`ClientMetadata.Capabilities` exposes the existing domain capability contract.
`deps status` reports supported targets and `excluded_targets`; ChatGPT's reason
is `manual_preparation_only`; VS Code is `dependency_management_deferred`.
`ClientSupportExclusionReason` is the shared reason policy, including the explicit
owner scope decision. Preserve the legacy VS Code adapter and its tests.

| Local client | Executable authority | Completion boundary |
| --- | --- | --- |
| Codex | Explicit CLI | Active with recognized listing; otherwise manual verification and explicit attestation. External uninstall remains explicit. |
| Claude | Explicit CLI | Existing CLI registration/listing contract. |
| Cursor | Explicit editor profile/executable | Initially manual. The composed provider accepts explicit activation attestation and records active/installation-verified state; this is operator evidence, not observed runtime discovery. Authentication remains separate. |
| Copilot | Explicit CLI | Existing marketplace/CLI listing contract. |
| Kiro | Explicit CLI with ACP duplex capability | Existing native config and bounded ACP verification contract. |
| Grok Build | Explicit CLI | Existing native plugin registry contract; unknown listings retain manual follow-up. |
| OpenCode | Explicit host-probe executable | Existing host-version selection, native config and namespace checks. Unsupported transports remain unsupported. |
| Gemini | Explicit profile; no executable required | Existing native configuration lifecycle. |
| Cline | Explicit profile; no executable required | Existing native configuration lifecycle. |
| Windsurf | Explicit profile; no executable required | Native MCP configuration when supported; skill-only preparation retains manual follow-up. |
| Kimi | Explicit profile; no executable required | Existing native plugin registry; SSE remains unsupported. |

Any supplied executable must still be an absolute clean path. CLI-registry
adapters require it on install, as does OpenCode host preparation. Native configuration clients do not gain an artificial process dependency.

No backend-profile API, VS Code-specific lifecycle orchestration, new identity
fallback, repair guard or sibling-alias ownership policy is introduced. Existing
request, binding, native recovery and imperative capabilities remain intact.

## Lifecycle convergence and component reporting

Do not change persisted `domain.LifecycleConverged`.
`environment.LifecycleFacts` distinguishes automatic retry from manual completion:

- Materialized manual/prepared activation with positive package/installation/
  runtime verification stops automatic mutation and retains `activation_manual`
  or `activation_prepared`, alongside authentication facts.
- Failed/not-run/absent verification, failed activation and unmaterialized
  packages remain incomplete. Never treat them as ready through attestation.
- Active/installation-verified state with unchecked authentication reports
  `auth_unverified` without another activation update. Pending/failed
  authentication remains independently visible.

Manual status offers the existing `add <same-source> --target <agent>
--activation-complete` contract when available. The composed provider handles
manual activation attestation before dispatching a direct adapter; inspecting
`cursor.Adapter.Activate` alone does not describe this CLI contract. VS Code saved targets are excluded rather than adding a new backend workflow.

Use actual `Prepared.Plan().Delivery.Components` for additive per-binding
`component_support` in sync, dry-run and saved-add output. Human output names
unsupported components. Unsupported-only packages fail preflight; mixed packages
can install supported components without retrying forever for unsupported ones.
Do not invent component decisions after failed SDK Prepare, or re-prepare
unchanged actions just to populate output metadata.

A failed unsupported installation must not mutate installed state, manifest,
lock or profiles. Verified immutable acquisition content may remain cached;
validate those bytes against the exact lock rather than rolling cache storage
back. Cache population is not an installed binding or an ownership claim.

## CLI and public surfaces

Keep the existing qualified local-file facade shortlist separate from saved
support: Claude/Codex local-file routes do not automatically expand when registry
support expands. Preserve direct local, discovery, native/OpenAI loader and
personal ChatGPT mapping paths.

Reject undetected/failed-detection targets for install/update/repair. Removal can
use a recorded explicit profile after a client disappears. Do not adopt existing
imperative bindings, confuse content-store corruption with projection drift,
move repair to a newer pin, drop retained plugin data, or erase ownership during
incomplete external uninstall.

Generate landing capabilities from domain definitions and `ClientSupported`,
with a drift check. Saved command generation uses that artifact; local/discovery/
ChatGPT/no-save routes remain explicit. Existing public release gating stays off.
Documentation describes current source support and these boundaries without
claiming a release or live-native-client qualification.

ChatGPT remains imperative, not saved: prepare an eligible package/marketplace
from signed app evidence or verified personal mapping; the user installs it in
ChatGPT Plugins and verifies its remote connection. Existing explicit activation
and external-uninstall attestations remain available. This is not local stdio
registration or automatic remote connector installation.

## Focused verification

Use disposable fixture profiles, HOME/data roots and local Git repositories only.
The harness uses real CLI, standard loading, source acquisition, content digests,
store, installer, projections, native observer, receipts and transactions.
Native processes, OpenCode version evidence, ACP responses and the trusted helper
are fixtures. The small 0700 helper still goes through real digest/copy/metadata
checks; it is never launched. Real helper/process qualification tests remain
separate and unchanged.

The eleven adapter subtests run in parallel with independent repositories,
profiles, state, runners and data. They retain the real isolated Git transport
and the complete race-tested matrix. For each adapter, one cohesive scenario
exercises:

1. Actual saved `add` at fixture Git HEAD; continue the same installation with a
   moving `main` requirement for lock/update behavior.
2. Sync and an unchanged repeat: no download/ref resolution, lifecycle mutation,
   ACP restart or host re-preparation, and identical manifest/lock/state bytes.
   Legitimate read-only observations are distinct from mutation.
3. Truthful status; Cursor activation attestation records active/installed state
   while retaining any authentication-review fact. Persisted convergence need
   not be true while authentication is unchecked.
4. A new source commit, targeted update and locked sync advance the exact digest.
   The schema-valid stdio fixture requests real `PLUGIN_DATA`; assert an absolute
   fixture-contained locator before writing a marker, then preserve it.
5. Drift refuses sync without overwriting edits; repair restores the binding's
   exact pin and retains data.
6. Transfer only manifest/lock to a second fixture and reproduce the digest.
7. Remove target intent, preserve incomplete external-uninstall ownership when
   required, then remove the binding while retaining content store and data.

Reuse existing Codex interrupted-retry/owned-recovery coverage. Add one native
Gemini recovery representative instead of duplicating generic recovery policy
for every adapter.

Negatives cover manual-only ChatGPT and mixed selection before acquisition,
undetected/failed detection, unsupported-only SSE for Kimi/OpenCode/Grok,
Kimi skill plus unsupported SSE with exact component reporting, native foreign
bindings and owner-deferred VS Code targets before acquisition. Kimi activation
must acknowledge a verified native registry effect so the existing recorder clears
its completed attempt; uncertain writes retain the strict recovery guard. Preserve existing tests and legacy capabilities.

Run focused SDK/host-authority and environment tests, the full adapter CLI matrix
with `-race`, composition tests, the generated-capability drift check and landing
saved-dependency tests on the parent's hosted test workspace. Do not run heavy
local tests, duplicate an active job, or repeat passed checks without a changed
risk. Parent owns final independent review, broader repository checks and any
later delivery decision. This plan is not evidence that those gates have passed.

Windows preserves the existing Kiro automatic ACP containment refusal: the matrix
asserts its real preflight error and absence of installed effects. Kiro’s complete
MCP lifecycle runs on supported Unix platforms; Windows fixture capabilities do
not claim automatic ACP support.
