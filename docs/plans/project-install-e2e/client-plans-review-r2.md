# Client project plans — independent R2 review

Reviewer: project-core lane. Reviewed the post-R1 client plan against
`3ea24e3014ddf6679ca86729c56f1069112b0d3a`, the accepted project core contract,
current registry composition, generic activation/attestation dispatch, selected
delivery traits, native-attempt recovery, and backend ownership selection.
Opened current first-party Kimi Code, Cline CLI and VS Code documentation.
No native runtime proof or implementation occurred.

Verdict: **narrow shared integration fixes required**. Most individual path and
client-lane choices are GO. These findings address existing code that runs
before/around an adapter; a project branch inside an adapter alone cannot fix
them. Coordinate the small shared changes with core rather than introducing a
new plugin engine or universal client framework.

## R2-1 — Cline session preparation must reach the adapter under attestation

`providers.Activator.Activate` currently accepts `ActivationComplete` before
dispatch whenever `AutomaticallyActivates` is false, immediately returning
active/installation-verified. Cline's existing native-config adapter has no
automatic CLI activation, so the proposed adapter-only "remain prepared after
attestation" policy would never run. This also affects effective-config
shadowing observations in any project lane that uses this generic shortcut.

Use the existing `ActivationPreflighter`, which the composed provider invokes
before the shortcut: Cline rejects `ActivationComplete` for selected project
session MCP, retaining the prepared state and explicit launch action. Cover
direct adapter reuse too. Project lanes with known shadowed/disabled evidence
likewise refuse positive attestation through their preflight rather than let
generic completion override that negative. Preserve existing user attestation.
No new generic policy interface is needed. Do not invent a persistent session
registration or set an automatic-activation predicate merely to dodge the gate.

Required regression: Cline project MCP is prepared; repeat saved add with
`--activation-complete` refuses with a precise session-only explanation; status
and fresh sync/restart still report prepared and the launch action. A known
shadowed/disabled project configuration must likewise
remain blocked instead of becoming active from the shortcut.

## R2-2 — project component delivery needs its actual lifecycle/recovery policy

Codex, Cursor, Copilot and VS Code retain user CLI-registry traits while their
proposed project lane directly edits native component configuration. Existing
generic policy reads `SelectedDelivery.EffectiveTraits`; zero SelectedDelivery
falls back to user ClientTraits. `nativeLifecycleClient` uses these facts to
write an attempt marker **before** native mutation. A new project branch can
therefore write project settings without the native-config attempt/recovery
guard if it only changes paths. Existing selected modes are closed records for
specific VS Code Local profile and Cursor user Stop contracts; they are not
generic project component authority.

Record one small scope-aware effective-policy route shared with core, selected
from the actual project delivery and persisted scope. Direct project component
lanes must use native-config attempt/ownership/recovery behavior; real project
CLI plugin lanes retain their actual CLI policy. Preserve all existing user
traits. Do not relabel a workspace as an existing user-profile selected mode,
fake ConfigRoot, globally change the client's traits, or add one custom engine
per adapter. The exact helper/field choice can stay internal, but identify its
owner and consumers before client implementation.

Required contract: for one CLI-user/native-project adapter, interrupt after a
native project edit and before acknowledgement. A retry must observe the
recorded unresolved attempt and recover/refuse safely; it must not blindly
rewrite project state or fall into user plugin listing/uninstall.

## R2-3 — VS Code composition, saved admission and physical sibling ownership

The production registry contains `vscode.New()`. Its legacy user behavior shares
the Copilot CLI backend. `vscode.NewLocal` is a different, explicitly configured
profile-registration contract with profile settings/qualified tuple and
`chat.plugins.paths` ownership; it does not mean `.vscode/mcp.json` workspace
component delivery. Also, `SavedDependenciesDeferred` currently rejects VS Code
before checking the declared scope. Merely adding ScopeProject and project
methods will either remain unreachable or accidentally alter frozen user
qualification.

Specify the production composition path for one explicit local-folder Local
harness project adapter. It may be a project branch in the current registered
adapter, preserving the default user branch; do not globally replace it with
NewLocal or mutate the singleton registry. Make saved qualification scoped:
the newly qualified project lane can pass while user dependency qualification
remains deferred. This is the already authorized narrow owner exception.

The current zero-mode `SharesBackend`/sibling lookup also assumes Copilot and
VS Code share one physical install. Project Copilot `.mcp.json` and VS Code
`.vscode/mcp.json` are distinct registries. Fence project physical ownership by
the actual selected layout/context so project operations cannot adopt/remove a
sibling's registration merely because the user backend family matches. Preserve
user backend sharing. Actual overlapping skill paths still use exact existing
collision/ownership rules; do not auto-adopt them.

Required tests: production CLI composition admits project VS Code, keeps its
user saved target deferred, and avoids Copilot global commands. In one project,
create distinct Copilot and VS Code project MCP entries; maintenance/removal of
one preserves the other's entry and receipt. No extra native host is qualified
by this fixture.

## R2-4 — ChatGPT artifact preparation has no executable project-scope route yet

The plan correctly forbids native ScopeProject, fake NativeRegistryLayout and
saved dependency admission for remote ChatGPT. It simultaneously proposes
`--scope project --no-save` preparation under `.uap/`. The accepted common scope
gate and planner reject that request before acquisition/layout. `--no-save`
alone does not bypass the native scope contract, and routing it through user
scope with a fabricated project profile would violate core invariants.

Choose a concrete bounded result before coding. The smallest option is to
return explicit unsupported for every native `--scope project` ChatGPT install,
retain the existing user/manual preparation route unchanged, and describe any
project-local artifact preparation as deferred. If project-local artifacts are
required now, name one reviewed artifact-only entry path outside native saved
admission, using existing preparation implementation, without inventing a
native binding or second engine. Coordinate that one exception with core;
do not leave an unreachable promised branch in the plan.

Required test: saved and unsupported native project requests make zero
acquisition/remote/native effects, while any explicitly supported artifact-only
path has a real entry point and cannot mutate account/workspace state.

## Current Kimi correction accepted

Independently opened current first-party Kimi Code MCP and skills docs. They
support project `.kimi-code/mcp.json` and `.kimi-code/skills/`, with project MCP
overriding same-name user entries and workspace trust controlling activation.
The revised current-component lane replaces the archived CLI session-file
assumption appropriately. Keep the pinned official-artifact gate, nearest-Git
skill-root check, exact fresh-session removal observation and no auto-trust.
Project SSE or other newly documented transport support must be explicitly
qualified; current docs alone must not silently widen user capability policy.

Primary references:

* https://moonshotai.github.io/kimi-code/en/customization/mcp
* https://moonshotai.github.io/kimi-code/en/customization/skills
* https://docs.cline.bot/cli/cli-reference
* https://code.visualstudio.com/docs/agent-customization/mcp-servers

## Per-client R2 ledger

| Client | R2 decision | Concrete integration focus |
| --- | --- | --- |
| Claude | GO | Exact project skills-dir identity/cwd; separate user profile; scope-aware removal. |
| Codex | GO after R2-2 | One component lane; actual native-config attempts/recovery; no global marketplace action. |
| Cursor | GO after R2-1/R2-2 | Component policy and truthful blocked/manual state reach adapter rather than generic attestation. |
| Gemini | GO after shared R2-1 | Existing native-config lifecycle is suitable; shadowing/disabled observations cannot be attested away. |
| OpenCode | GO after shared R2-1 | Exact project layout and effective-config observation; use existing native transitions. |
| Copilot CLI | GO after R2-2/R2-3 | Native component recovery plus distinct project registry ownership; preserve bare/nested document format. |
| Kiro | GO | Workspace/agent overrides and prepared scope are explicit; qualify IDE/CLI separately. |
| Grok Build | GO | One observed project-directory plugin lane; Claude-compatible collisions and no global trust command. |
| Qoder | GO | Explicit project install/uninstall, keep-data and exact scoped source/version listing. |
| Cline | GO after R2-1 | Session MCP remains prepared under completion attestation and restart. |
| Windsurf / Devin | GO after shared R2-1 | One pinned newer harness; actual effective dedicated project config and shadowing. |
| Kimi Code | GO after independent current-doc R1 refresh and shared R2-1 | Current native components accepted; no archived CLI or automatic trust. |
| ChatGPT | Fix R2-4 | Choose reachable artifact-only preparation or explicit project unsupported result. |
| VS Code | Fix R2-2/R2-3 | Explicit production project composition, scoped saved qualification and independent physical ownership. |

No style-only changes requested. Author updates shared/client sections and the
ledger; reviewer verifies the fixes. Parent owns implementation authorization
and later code/native/release qualification gates.

## Applied-fix verification — 2026-10-23

Verified the revised client plan contains: existing activation preflight guards
before generic completion (Cline session-only and known negative project
evidence); one core-owned declarative scoped lifecycle/backend policy and its
attempt/recovery/group/data consumers; explicit VS Code project-only product
composition and qualification while preserving user/NewLocal behavior; distinct
project Copilot/VS Code physical ownership; and explicit unsupported native
ChatGPT project requests for every intent including `--no-save`. Current Kimi
Code project components were independently checked against current official
docs, and the source reviewer refreshed its R1.

**All 14 client plans: R2 GO.** All R2 findings are closed at the plan level.
Actual pinned-artifact/help observation remains each client's stated pre-coding
gate, followed by code review and truthful native/configuration E2E. This review
does not assert implementation or runtime completion.
