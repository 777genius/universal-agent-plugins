# Project installation support

**Current published CLI:** Signed Directory short names work in explicit Project
scope for eligible clients and all mandatory components (merged in [#725](https://github.com/777genius/universal-agent-plugins/pull/725)).
Signed Directory v1 User scope evidence alone is **not** native Project runtime
proof. A full-SHA direct Git source remains an alternative for reproducible installs.

**Released in CLI 0.1.93:** explicit project scope prepares the client components
listed below. Select `--scope project` and an existing project directory; the
add and maintenance commands default to `user`. Argumentless install in CLI
0.1.94 selects project scope for a regular `uap.toml` in the exact current
directory; explicit scope wins. Native observation records are pinned to their
recorded client versions and runs; they do not prove arbitrary runtime behavior.
[Project installation](PROJECT_INSTALLATION.md) explains the
portable agreement, maintenance and recovery commands.

Every admitted target keeps project state, receipts, locks and retained data
under that project's `.uap/`, with the original user profile remaining user
authority. Client paths below are relative to the selected project. Configuration
ownership and native discovery are separate: Prepared means further client
selection, reload, trust or session work can remain.

## Delivery by client

| Client | Project files or preparation | Result and native boundary |
| --- | --- | --- |
| Claude Code | Projected plugin in `.claude/skills/<artifact>` | Exact observed native identity controls readiness. Known untrusted/suppressed results remain Prepared with installed-file verification. See [Claude guidance](CLAUDE_PROJECT_INSTALLATION.md). |
| Codex | Skills in `.agents/skills`; exact MCP members in `.codex/config.toml` | Prepared component delivery. The native user plugin catalog/cache is separate; untrusted project MCP can remain disabled. |
| Cursor | `.cursor/skills` and owned `.cursor/mcp.json` members | Prepared. Editor/plugin UI activation, effective discovery and MCP startup require their own evidence. |
| Gemini | `.gemini/skills` and owned `.gemini/settings.json` members | Prepared components; no invented project extension registry. Native trust and higher-priority settings can affect selection. |
| OpenCode | `.opencode/skills` and root `opencode.json` or `opencode.jsonc` | Prepared; simultaneous competing config files are refused. User, ancestor and explicit settings can affect effective selection. See [OpenCode guidance](OPENCODE_PROJECT_SCOPE.md). |
| Copilot CLI | `.github/skills`; existing ordinary `.mcp.json`, otherwise `.github/mcp.json` | Prepared. Project MCP ownership is distinct from VS Code; overlapping skill names require exact ownership and cannot be adopted silently. |
| Kiro | `.kiro/skills` and `.kiro/settings/mcp.json` | Prepared. Custom-agent resource selection can change which workspace configuration is loaded. |
| Grok Build | Skills-only prepared plugin in `.grok/plugins/<artifact>` | Prepared with package validation; project MCP is unsupported. Disabled/untrusted native evidence cannot become Active by attestation. |
| Qoder | Managed `.uap/managed/clients/qoder/<artifact>`; Project Skills in `.qoder/skills`, owned stdio/HTTP MCP entries in `.mcp.json`, plus legacy manual import view | Prepared native Project Skills/MCP configuration. Restart and review session trust/activation; MCP listing is not `tools/call`. Independently imported User-native cache remains outside UAP ownership. See [Qoder guidance](QODER_PROJECT_INSTALLATION.md). |
| Cline | `.cline/skills`; prepared `.uap/clients/cline/mcp.json` | Always Prepared. Explicit `CLINE_MCP_SETTINGS_PATH` selects the whole MCP file for one process; a process without that override uses its ordinary selection. The observed same-name user skill shadows project copies. See [Cline guidance and exact-source evidence](CLINE_PROJECT_INSTALLATION.md). |
| Windsurf / Devin | Current observed Devin Local: `.devin/skills` and `.devin/mcp_config.json` | Prepared; new delivery requires the observed `devin` 3000.11.205 harness. Legacy Cascade and an unrelated UI executable are not substitutes. |
| Kimi Code | `.kimi-code/skills` and `.kimi-code/mcp.json` | Prepared; new delivery admits selected `kimi` 2.3.1, 2.3.2 or source-qualified stable 2.5.2 with its nearest Git-root project Skills/MCP. Version 2.5.2 has 30/30 genuine source-backed Project Skills add/repeat/remove evidence in [#811](https://github.com/777genius/universal-agent-plugins/pull/811); MCP native startup on 2.5.2 is not independently proven by that Skills audit. The separate user plugin registry is unchanged. |
| ChatGPT | No supported native project filesystem installation | Explicit refusal before source acquisition, including `--no-save`; the existing user/manual path is preserved. See [ChatGPT scope](CHATGPT_PROJECT_SCOPE.md). |
| VS Code | `.github/skills` and exact `servers` members in `.vscode/mcp.json`, including native JSONC syntax | Prepared with installed-file verification for one local folder. Open that folder and review workspace/server trust; observed metadata does not prove MCP startup or model execution. See [VS Code preparation](PROJECT_INSTALLATION.md#vs-code-project-preparation) and [pinned metadata evidence](evidence/project-scope-research/vscode-1.143.1-untrusted-workspace-linux-amd64-40188310034/README.md). |
| Pi | `.pi/extensions/<artifact>`, managed `.pi/skills/<skill>`, exact owned `mcpServers` members in `.pi/mcp.json` | Project-scoped Extensions, Skills and stdio/Streamable HTTP MCP preparation, preserving user/project B and foreign entries; **Prepared** until Pi separately grants trust. Native `tools/call` is now qualified only with a local scripted model, not vendor authentication. [Extensions #631](https://github.com/777genius/universal-agent-plugins/pull/631), [Skills/MCP #640](https://github.com/777genius/universal-agent-plugins/pull/640), [real Pi call #656](https://github.com/777genius/universal-agent-plugins/pull/656). |

## Native project evidence matrix

The table below distinguishes **project file preparation** from a **genuine native
client run**. It tracks source integrated since [PR #520](https://github.com/777genius/universal-agent-plugins/pull/520),
including later scoped client qualification. These source-level observations do
not mean that every tested change has been published in a CLI release.
Native observations apply only to their pinned client versions, test modes and
supported surfaces. A *qualified* lane proves the named behavior through
the actual client executable, not a live vendor model or general-purpose access.

| Client | Project delivery | Native project lifecycle / restart / removal | MCP observed through native client | Evidence and limit |
| --- | --- | --- | --- | --- |
| Claude Code | Yes | Pinned genuine native Prepared lifecycle and trusted project discovery/removal qualified | Pinned Claude Code completed **UAP-managed Project stdio** MCP `initialize`, `tools/list`, `tools/call` and returned `uap-pong` on Linux/macOS/Windows; a separate **User stdio** native test called the UAP-owned tool from projects A and B on all three OS, using a local scripted model. Streamable HTTP and vendor authentication remain unqualified | [Project native 3-OS](https://github.com/777genius/universal-agent-plugins/actions/runs/42358671316), [merged #765](https://github.com/777genius/universal-agent-plugins/pull/765), [User native 3-OS](https://github.com/777genius/universal-agent-plugins/actions/runs/42380094580), [merged #770](https://github.com/777genius/universal-agent-plugins/pull/770), [foreign-server baseline #664](https://github.com/777genius/universal-agent-plugins/pull/664), [guide](CLAUDE_PROJECT_INSTALLATION.md); actual selected trust, precedence and publication remain separate |
| Codex | Yes | Pinned native project skills lifecycle and trusted MCP restart/removal qualified | Pinned native trusted project MCP initialize and direct tool-call fixture; no vendor model authentication | [Native Client Runtime](https://github.com/777genius/universal-agent-plugins/actions/runs/40209659575), [#499](https://github.com/777genius/universal-agent-plugins/pull/499) |
| OpenCode | Yes; V1 preserved, V2 priority | Pinned native V1 and genuine V2 v2.2.5 A/B lifecycle and Skills/config discovery on Linux/macOS/Windows | V2 real client + controlled stdio server **initialize, tools/list and tools/call** qualified through a local scripted model on Linux/macOS/Windows; vendor login and real provider models remain unqualified | [Native V2 #626](https://github.com/777genius/universal-agent-plugins/pull/626), [V2 MCP #638](https://github.com/777genius/universal-agent-plugins/pull/638), [real call #655](https://github.com/777genius/universal-agent-plugins/pull/655), [V1 baseline](https://github.com/777genius/universal-agent-plugins/actions/runs/40209659575) |
| Cursor | Yes | Qualified for isolated A/B lifecycle, restart and removal | Initialize and tools/list qualified; tools/call **not** qualified | [#521](https://github.com/777genius/universal-agent-plugins/pull/521) |
| Gemini CLI | Yes | Qualified A/B lifecycle, restart and removal | Initialize, tools/list, tools/call qualified with controlled probe | [#526](https://github.com/777genius/universal-agent-plugins/pull/526) |
| GitHub Copilot CLI | Yes | Qualified A/B lifecycle, restart and removal | Initialize, tools/list, tools/call qualified with scripted loopback provider | [#527](https://github.com/777genius/universal-agent-plugins/pull/527) |
| Kimi Code | Yes | Qualified A/B lifecycle, restart and removal | Initialize, tools/list, tools/call qualified across **two separate same-source native lanes** | [#524](https://github.com/777genius/universal-agent-plugins/pull/524) |
| Cline | Yes | Qualified A/B lifecycle, restart and removal | Initialize, tools/list, tools/call qualified under explicit process-wide MCP path selection | [#525](https://github.com/777genius/universal-agent-plugins/pull/525); default IDE forwarding not qualified |
| Grok Build | Skills only | Qualified supported skills lifecycle, restart and removal | Project MCP unsupported | [#522](https://github.com/777genius/universal-agent-plugins/pull/522) |
| Qoder | Project Skills and stdio/HTTP MCP | Composed A/B add/update/repair/remove; pinned native MCP list observation is supported | MCP `tools/call` is not qualified; native import cache is separate | [Composed E2E](../cli/cmd/agentplugins/project_qoder_e2e_test.go), [native collector](../scripts/project_native_qoder_installed_mcp.py), [guide](QODER_PROJECT_INSTALLATION.md) |
| Windsurf / Devin | Yes | Qualified supported project skills lifecycle, restart and removal | Not qualified | [#528](https://github.com/777genius/universal-agent-plugins/pull/528); separate strict cache-frame follow-up [#611](https://github.com/777genius/universal-agent-plugins/pull/611) |
| Kiro | Prepared | Authenticated native lifecycle **not qualified** | Not qualified | [Draft #529](https://github.com/777genius/universal-agent-plugins/pull/529); dedicated test authentication needed |
| VS Code | Prepared | Trusted native project lifecycle **not qualified** | Not qualified | [Deferred Draft #530](https://github.com/777genius/universal-agent-plugins/pull/530) |
| Pi | Native extensions, per-project Skills and exact owned `.pi/mcp.json` entries | Genuine Pi v1.4.0 A/B trust, fresh extension loading, Skill discovery, scoped add/update/repair/remove and foreign preservation on Linux/macOS/Windows; **Prepared** until trust | Real Pi stdio `initialize`/`tools/list` and exactly one client-driven `tools/call` observed through a local scripted model on all three OS; vendor-model login/provider authorization not qualified | [Extensions #631](https://github.com/777genius/universal-agent-plugins/pull/631), [Skills/MCP #640](https://github.com/777genius/universal-agent-plugins/pull/640), [ownership kernel #639](https://github.com/777genius/universal-agent-plugins/pull/639), [real native call #656](https://github.com/777genius/universal-agent-plugins/pull/656) |
| ChatGPT | **No** native project filesystem adapter | Not supported | Not applicable | [ChatGPT scope](CHATGPT_PROJECT_SCOPE.md); user/manual app flow is separate |

All project-native lanes in this table leave **vendor authentication and real
model execution unqualified** unless separately stated; no such qualification is
asserted here. MCP fixture calls are real native-client/probe exchanges, not
proof of a third-party remote service. A *Prepared* delivery is not automatically
*Active*. Project UI/reload/trust, configuration precedence and per-client
restrictions remain as listed above.

Pi project MCP `tools/call` is qualified in current source by [merged #656](https://github.com/777genius/universal-agent-plugins/pull/656), closing [#623](https://github.com/777genius/universal-agent-plugins/issues/623). The merged [OpenCode V2 #655](https://github.com/777genius/universal-agent-plugins/pull/655) proves a real selected Project stdio MCP call. [Claude #664](https://github.com/777genius/universal-agent-plugins/pull/664) proved a foreign trusted MCP call; subsequent [merged #765](https://github.com/777genius/universal-agent-plugins/pull/765) proves an **actual UAP-managed Claude Project stdio** `tools/call` returning `uap-pong` via a selected native process on Linux/macOS/Windows. [Merged #770](https://github.com/777genius/universal-agent-plugins/pull/770) separately proves UAP-managed **User stdio** `tools/call` through native Claude processes in both project A and project B on Linux/macOS/Windows, followed by scoped removal and foreign-state preservation. [Issue #624](https://github.com/777genius/universal-agent-plugins/issues/624) tracks evidence and remaining gaps; Streamable HTTP, vendor-model authentication, release and other client behavior require separate evidence. A configured MCP entry or successful `tools/list` alone never establishes tool execution; scripted-model calls are not vendor-model authentication.

For releases, explicit project scope first shipped in CLI 0.1.93 and automatic
project selection for argumentless `install` in 0.1.94. For client-specific project integration such as #520, consult its exact
Git source and CI evidence rather than treating an installer release as blanket
native activation or MCP runtime proof.

## What the verification establishes

Local ownership checks run even when optional live native discovery is disabled.
They verify exact recorded files/members before mutation and preserve foreign
content, other projects and retained data. Repair can restore missing owned
components or a damaged managed package. An externally edited native component
requires review; it is not silently overwritten. Freshly reopened clients of the
SDK use the same persisted ownership and recovery rules.

A fixture that exercises the public CLI, real source acquisition, transactions
and saved state establishes those installer contracts. Actual native inventory,
MCP initialization, tool calls, authentication and model behavior require separate
observed evidence. User-scope native passes do not qualify project behavior.

Pinned observations and per-client limits are preserved in the
[implementation and research records](plans/project-install-e2e/client-plans.md).
Claude's observed untrusted suppression, Codex's trust-gated configuration,
Cline's process-wide source selection and Qoder's global native cache remain
explicit restrictions. Installation does not grant trust or promise arbitrary
user/admin/session configuration is overridden.
