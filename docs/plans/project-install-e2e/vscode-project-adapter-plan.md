# VS Code project component adapter: conditional implementation plan

2026-10-25. Prepared against assembly `d94d6c1b` and the exact collector source
in PR #515 (`37fc30df`). This is a concrete design for review, not permission to
skip the existing observation gate. The owner project-only exception is recorded
in `docs/PROJECT_INSTALL_OWNER_SCOPE.md`.

## Evidence gate and admitted claim

The pinned help/version observation succeeded; the genuine workspace observation
run is [40182639794](https://github.com/777genius/universal-agent-plugins/actions/runs/40182639794).
Its job was still queued at this checkpoint. Product implementation starts only
after the real artifact establishes relevant selected-folder MCP and skill
metadata, an independent critic verifies that interpretation, and this concrete
plan receives the required independent reviews. A failed sandbox, unavailable
proposed API, or empty/blocked inventory keeps the corresponding gate open.

Verify the downloaded outer artifact digest and inner envelope, exact source SHA,
publisher commit/version, nonroot and namespace proof, strict process-tree cleanup,
all four selected single-folder observations, ungranted trust, original HOME,
source hashes and metadata attribution. Preserve every native row and duplicate.
Require A/B separation, A restart, and removal of A markers while user/B files
remain unchanged. Skill attribution must use its emitted source URI; MCP attribution
is the stated inference from unique sealed marker URLs, not an emitted config path.
Keep `native_project_qualified=false`: inventory does not prove trust, MCP startup,
model/skill execution, active-session isolation or effective Local/Copilot selection.

## Small dedicated backend

Add `vscode.NewProject()` returning a distinct project adapter; preserve `New()`
and `NewLocal()` source and user/Local/TEST behavior. Reuse existing detection,
but implement every project lifecycle/identity/layout capability explicitly so
the Copilot CLI backend cannot be reached by project calls. Reject user/local,
remote and multi-root contexts at the project adapter boundary. Target authority
is the canonical existing project root plus the original detected user profile.
Never substitute HOME, a profile directory or a package path for scope.

Add explicit project composition in `clients/all` and select it in
`cli/cmd/agentplugins/compose.go` after canonical scope resolution, before detection,
planning or acquisition. Default composition and authoring/user routes retain
`vscode.New()`. SDK callers choose the explicit project registry/constructor;
missing project composition fails with actionable admission rather than falling
back to the Copilot adapter. Keep generic packages free of concrete client imports.

After evidence admission, declare VS Code `LifecycleNativeConfig` project policy
in the existing client table. Project backend family is `vscode`, distinct from
Copilot. Select only skills and MCP: hooks, commands, apps and editor-plugin
registration remain unsupported. An unsupported explicit selection must fail
before source acquisition, intent, locks or mutable project state.

## Native layout and ownership

Use `<project>/.github/skills/<name>` and `<project>/.vscode/mcp.json`, with the
editor's `servers` member collection. Do not create portable `.mcp.json`,
`chat.pluginLocations`, a marketplace, user configuration or profile settings.
Record native registry root `<project>/.vscode`; managed packages and plugin data
remain under the existing selected `.uap` namespace.

Add a narrow `nativeconfig.CodecVSCode` with `servers`, explicit `type:stdio`
and `type:http`/`type:sse` transports and the pinned editor schema's
command/args/env/cwd or url/headers subset. Preserve all unrelated top-level
members, inputs, sandbox fields, comments, other entries and enablement. Own only
the complete exact managed member; unknown/changed fields inside it invalidate
the receipt and refuse update/removal. No shared neutral Server shape expansion
is needed: portable environment values remain strings, while foreign native
numeric values are preserved untouched. Unsupported dialect options fail rather
than being discarded. JSONC syntax is accepted in the actual `mcp.json` file;
there is no invented sibling `mcp.jsonc` selection.

Reuse `shared.ProjectComponents`, `ProjectComponentMutation`, nativeconfig
conditional writes/locks and existing scoped journal/recovery. Use distinct
`vscode_project_skill_directory` and `vscode_project_mcp_server` receipt kinds,
exact stable digests, portable source-relative skill paths and the selected
package/data roots. Reuse the existing managed stdio launcher rather than adding
a second shell/placeholder resolver. Reject copied receipts, foreign same-name
targets, aliases/symlinks, changed package bytes and replaced project roots.
Do not adopt another client's same-name skill merely because its bytes match.

## Result and maintenance behavior

Automatically apply exact file preparation and verify it read-only. Report
`Prepared + Installed` with `ActivationAttested=false` and
`NativeDiscoveryReconciled=false`, even when research inventory succeeded.
Installed means owned configuration, never a running MCP server. Supply the
selected project path and action to open it in VS Code, review workspace/server
trust and inspect skills/MCP. Never grant trust, start MCP, accept operator
attestation as native observation, or run Copilot/code add-MCP for this lifecycle.

Replay preserves exact bytes and reports no mutation. Update/repair/removal use
the original scoped binding and exact local receipts. Missing owned files may
be repaired; changed or foreign files refuse. Removal deletes only exact owned
skill directories and MCP members, preserving user/B artifacts and plugin data.
Retain the common transaction's interrupted-attempt fencing and recover/refuse
behavior; introduce no alternate recovery engine.

## Necessary verification and publication

Use meaningful codec preservation/collision fixtures and the existing public
SDK/CLI project lifecycle harness: user/A/B, unchanged replay, version update,
missing-file repair, removal/retained data, same-name foreign and cross-client
skills, scoped locks/receipts, aliases and replaced root admission. Assert zero
Copilot/vendor executions and unchanged user backend. Reuse the existing process
death recovery fixture for the skill-swap/MCP-commit seam rather than duplicating
its process engine. Cover mismatched scope/registry rejection before acquisition.

Run affected Go tests/vet/lint, generated capability checks, necessary terminal
and public CLI contracts, and the existing three-platform project CI. Review the
frozen implementation independently, resolve findings, then integrate into #499
with original traceability. Update the capability matrix and five-language
project guidance to describe preparation and actual observed limits. Do not
rename historical release evidence or claim a new release.
