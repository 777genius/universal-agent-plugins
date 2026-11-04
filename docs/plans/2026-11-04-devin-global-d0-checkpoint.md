# Devin Global D0: source/runtime gate and exact-profile authority

2026-11-04. **BLOCKED — not a native Global E2E PASS.** This is an evidence
checkpoint for [#710](https://github.com/777genius/universal-agent-plugins/pull/710),
not a change to the already reviewed implementation plan. Do not grant
Windsurf-wide user Skills/MCP support or merge based on this note.

## Repository snapshot (recheck before edits)

Read before any product code: `AGENTS.md`, the accepted leaf plan,
`clients/windsurf/{detect,scope,version_profile,project_native,project_lifecycle,activate,plan}.go`,
`clients/detection.go`, `adapters/clientdetect/detector.go` and
`domain/clients_runtime.go`. On this check:

- #710 head `e6a808eb13bfea8985ebca3057e115368a0abc61`, draft, plan only;
  its base is `05338655fe9e9f829f454c7feb43bcacada9523d`.
- Moving `main` was `acedefd5fab319176fa4ff286349eeda0dd1ff62`.
- Independent moving PRs: #698 `5e16be5520457d10bd05fa7b151ee4cc03b363ec`,
  #704 `90ff528f8dde9d6b02119b1de0a47c5043b9a67e`,
  #707 `152e5b78bfbebe24a06c3757ddfffd418e8e5650`.
  These are observation SHAs, **not** integration baselines to silently overwrite.
  #707 remains first main-merge priority.

## What is genuinely established vs missing

The prior retained
[project evidence](project-install-e2e/windsurf-devin-implementation-evidence.md)
records an actual official Linux x86_64 archive for Devin `3000.11.205`
(archive SHA-256
`2e65c8b93682258fc91f5e4cead92e3698338cebf1b31dbc32a3201cb63705c4`,
extracted binary SHA-256
`642c82e32cb3dec90cf5ab01975520b425307872c7a650c9fb5b0653f37681f4`)
and bounded, credential-free real `--version`/help execution. Its observed output
was `devin 3000.11.205 (6dea39440ae3)`. This is **historical version/format
evidence only**, not proof of current Global selection, source discovery, MCP
handshake, tool calls, or an executable available in the present runtime.

This run inspected the connected repository and the original evidence, but the
current isolated execution host has **no `devin` executable** and cannot resolve
github.com to obtain a binary. No genuine `devin skills list --json`,
`devin mcp list/get`, source discovery, Devin-driven
`initialize` / `tools/list` / `tools/call` or remote Streamable HTTP
interaction has been executed. These stages are **BLOCKED / NOT OBSERVED**;
existing inert collectors, direct RPC to a test server, test fixtures and
green plan-only CI must not be promoted to native success.

## Confirmed design collision in current adapter

1. `DetectSurfaces` computes one `DetectedClient` and chooses
   `shared.FirstPath(windsurfCLI, devinCLI)`. Coexisting binaries therefore
   select **Windsurf first**, even when a real Devin binary is also present.
2. `ConfigRoot` is `selectedLegacyRoot` (`~/.codeium/windsurf*`) and may
   intentionally be empty. It is **not** the Devin Global config authority.
   The `devin_local_config` discovery surface is only evidence and must not
   silently replace the current mutation root.
3. `RequiresProjectVersion` probes only an explicitly selected absolute
   primary `devin`/`devin.exe`; `requireCurrentDevin` accepts only
   observed exact `3000.11.205`. The detector has a bounded original-host
   version environment, but ordinary Detect deliberately does not probe.
4. `DetectedClient` currently contains one `ConfigRoot`, one
   `ExecutablePath`, and `Version`. Management startup reconstructs a
   selected target with root/executable but does not supply a version in
   `managementDetector.Detect`. This is a concrete **selection and propagation
   seam**, not permission to treat any Windsurf user target as Devin.
5. Current user RefinePlan/Activate/Deactivate are legacy Cascade routes:
   user Skills are prepared, and owned MCP entries target the selected Cascade
   codec/root. Current Devin Project uses the separate
   `shared.ProjectComponents` + `ProjectComponentMutation` with
   `nativeconfig.CodecDevin` and stays prepared even after installed files.
   Changing global capabilities unconditionally would misroute old receipts.

Canonical Global candidate is a **validated original** XDG user config root
`$XDG_CONFIG_HOME/devin` (or `~/.config/devin` when applicable) on Unix and
`%APPDATA%/devin` on Windows; component paths are `skills/<name>/SKILL.md`
and `mcp_config.json`. Project remains `.devin/skills` and
`.devin/mcp_config.json`. Current
[Devin MCP docs](https://docs.devin.ai/cli/extensibility/mcp/configuration),
[Skills docs](https://docs.devin.ai/cli/extensibility/skills/overview) and
[precedence](https://docs.devin.ai/cli/reference/configuration/global-vs-local)
support these *formats*. They do not supply the missing runtime observation.

## Atomic integrator request (same leaf only)

Provide the narrow selected-host/profile authority contract that carries
**which exact executable was selected, observed exact version, and original
canonical XDG/APPDATA root** consistently through public CLI/management, plan,
native mutation, binding/receipts and cold-process replay. Keep legacy Windsurf
as the default when ambiguous, and preserve existing Project eligibility.
Never implement `ClientWindsurf` blanket user capability or move old Cascade
receipts to Devin. A user-scope route must be admitted by its **recorded Devin
profile**; exact owned removal must continue working offline after a client
upgrade/uninstall. Reuse shared native components, kernel and launcher.
The integrator's central traits and the adapter implementation must commit in
this #710 leaf, **not** as a separate early main capability change.

## D0 unblock oracle, before production edits

On a disposable host with the hash-verified genuine binary, capture the exact
`PATH` selection, selected binary hash/version, original HOME/XDG/APPDATA
authority, clean A/B project roots and immutable source digest. With one mixed
source (Skill, inert stdio MCP and inert Streamable HTTP MCP), capture:

- Genuine Devin-driven Global Skill discovery for A and cold B, and selected
  Project A/B control; no mere file-exists or forged `--json` success.
- The genuine client's source of each effective MCP name (Global vs Project vs
  local override), observable `initialize`, `tools/list`, and
  `tools/call` for both transports with distinct nonces. If client auth,
  trust or any observer is unavailable, mark that cell BLOCKED, not PASS.
- Strict path/receipt snapshots before and after add, update, independent A/B
  replay, foreign-name collision, repair and exact offline removal. Legacy
  `~/.codeium` and unrelated user/project configs must remain byte-identical.

Only after this first source/call spike should the leaf implement admitted
Global logic, add focused facade assertions and request independent code
review. Avoid rerunning overlapping CI waves or initiating a release.
