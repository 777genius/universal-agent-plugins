# Project installation support

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
| Qoder | Managed bundle in `.uap/managed/clients/qoder/<artifact>` with a closed `.agentplugins-qoder-native` manual import view | Prepared skills only. Explicit native import creates a separate globally cached copy; UAP removal removes only its owned preparation. See [Qoder guidance](QODER_PROJECT_INSTALLATION.md). |
| Cline | `.cline/skills`; prepared `.uap/clients/cline/mcp.json` | Always Prepared. Explicit `CLINE_MCP_SETTINGS_PATH` selects the whole MCP file for one process; a process without that override uses its ordinary selection. The observed same-name user skill shadows project copies. See [Cline guidance and exact-source evidence](CLINE_PROJECT_INSTALLATION.md). |
| Windsurf / Devin | Current observed Devin Local: `.devin/skills` and `.devin/mcp_config.json` | Prepared; new delivery requires the observed `devin` 3000.11.205 harness. Legacy Cascade and an unrelated UI executable are not substitutes. |
| Kimi Code | `.kimi-code/skills` and `.kimi-code/mcp.json` | Prepared; new delivery requires observed `kimi` 2.3.1 and the selected skill directory's nearest Git root. The user plugin registry stays separate. |
| ChatGPT | No supported native project filesystem installation | Explicit refusal before source acquisition, including `--no-save`; the existing user/manual path is preserved. See [ChatGPT scope](CHATGPT_PROJECT_SCOPE.md). |
| VS Code | `.github/skills` and exact `servers` members in `.vscode/mcp.json`, including native JSONC syntax | Prepared with installed-file verification for one local folder. Open that folder and review workspace/server trust; observed metadata does not prove MCP startup or model execution. See [VS Code preparation](PROJECT_INSTALLATION.md#vs-code-project-preparation) and [pinned metadata evidence](evidence/project-scope-research/vscode-1.143.1-untrusted-workspace-linux-amd64-40188310034/README.md). |
| Pi | `.pi/extensions/<artifact>`, managed `.pi/skills/<skill>`, exact owned `mcpServers` members in `.pi/mcp.json` | Project-scoped Extensions, Skills and stdio/Streamable HTTP MCP preparation. UAP preserves user/project B and foreign entries; **Prepared** until Pi separately grants trust. Never changes trust or assumes live tool execution. [Extensions #631](https://github.com/777genius/universal-agent-plugins/pull/631), [Skills/MCP #640](https://github.com/777genius/universal-agent-plugins/pull/640). |

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
| Claude Code | Yes | Pinned genuine native Prepared lifecycle and trusted project discovery/removal qualified | Project MCP initialize/list/call not qualified | [Native Client Runtime](https://github.com/777genius/universal-agent-plugins/actions/runs/40209659575), [guide](CLAUDE_PROJECT_INSTALLATION.md); trust and shadowing matter |
| Codex | Yes | Pinned native project skills lifecycle and trusted MCP restart/removal qualified | Pinned native trusted project MCP initialize and direct tool-call fixture; no vendor model authentication | [Native Client Runtime](https://github.com/777genius/universal-agent-plugins/actions/runs/40209659575), [#499](https://github.com/777genius/universal-agent-plugins/pull/499) |
| OpenCode | Yes; V1 preserved, V2 priority | Pinned native V1 and genuine V2 v2.2.5 A/B lifecycle and Skills/config discovery on Linux/macOS/Windows | V2 real client+controlled stdio server **initialize and tools/list** qualified; `tools/call`, provider login and real models still unqualified | [Native V2 #626](https://github.com/777genius/universal-agent-plugins/pull/626), [V2 MCP #638](https://github.com/777genius/universal-agent-plugins/pull/638), [V1 baseline](https://github.com/777genius/universal-agent-plugins/actions/runs/40209659575) |
| Cursor | Yes | Qualified for isolated A/B lifecycle, restart and removal | Initialize and tools/list qualified; tools/call **not** qualified | [#521](https://github.com/777genius/universal-agent-plugins/pull/521) |
| Gemini CLI | Yes | Qualified A/B lifecycle, restart and removal | Initialize, tools/list, tools/call qualified with controlled probe | [#526](https://github.com/777genius/universal-agent-plugins/pull/526) |
| GitHub Copilot CLI | Yes | Qualified A/B lifecycle, restart and removal | Initialize, tools/list, tools/call qualified with scripted loopback provider | [#527](https://github.com/777genius/universal-agent-plugins/pull/527) |
| Kimi Code | Yes | Qualified A/B lifecycle, restart and removal | Initialize, tools/list, tools/call qualified across **two separate same-source native lanes** | [#524](https://github.com/777genius/universal-agent-plugins/pull/524) |
| Cline | Yes | Qualified A/B lifecycle, restart and removal | Initialize, tools/list, tools/call qualified under explicit process-wide MCP path selection | [#525](https://github.com/777genius/universal-agent-plugins/pull/525); default IDE forwarding not qualified |
| Grok Build | Skills only | Qualified supported skills lifecycle, restart and removal | Project MCP unsupported | [#522](https://github.com/777genius/universal-agent-plugins/pull/522) |
| Qoder | Prepared skills bundle, explicit native import | Qualified explicit skills lifecycle and restart/removal | Project MCP unsupported by this UAP route | [#523](https://github.com/777genius/universal-agent-plugins/pull/523); global native import has separate ownership |
| Windsurf / Devin | Yes | Qualified supported project skills lifecycle, restart and removal | Not qualified | [#528](https://github.com/777genius/universal-agent-plugins/pull/528); separate strict cache-frame follow-up [#611](https://github.com/777genius/universal-agent-plugins/pull/611) |
| Kiro | Prepared | Authenticated native lifecycle **not qualified** | Not qualified | [Draft #529](https://github.com/777genius/universal-agent-plugins/pull/529); dedicated test authentication needed |
| VS Code | Prepared | Trusted native project lifecycle **not qualified** | Not qualified | [Deferred Draft #530](https://github.com/777genius/universal-agent-plugins/pull/530) |
| Pi | Native extensions, per-project Skills and exact owned `.pi/mcp.json` entries | Genuine Pi v1.4.0 A/B trust, fresh extension loading, Skill discovery, scoped add/update/repair/remove and foreign preservation on Linux/macOS/Windows; **Prepared** until trust | Real Pi `mcp list` observed stdio `initialize` and `tools/list`; no native `tools/call` execution or model/provider authentication claimed | [Extensions #631](https://github.com/777genius/universal-agent-plugins/pull/631), [Skills/MCP #640](https://github.com/777genius/universal-agent-plugins/pull/640), [ownership kernel #639](https://github.com/777genius/universal-agent-plugins/pull/639) |
| ChatGPT | **No** native project filesystem adapter | Not supported | Not applicable | [ChatGPT scope](CHATGPT_PROJECT_SCOPE.md); user/manual app flow is separate |

All project-native lanes in this table leave **vendor authentication and real
model execution unqualified** unless separately stated; no such qualification is
asserted here. MCP fixture calls are real native-client/probe exchanges, not
proof of a third-party remote service. A *Prepared* delivery is not automatically
*Active*. Project UI/reload/trust, configuration precedence and per-client
restrictions remain as listed above.

For releases, explicit project scope first shipped in CLI 0.1.93 and automatic
project selection for argumentless `install` in 0.1.94. The #520 integration
postdates the published 0.1.95 tag, so use its exact Git source/CI evidence
until a subsequent release explicitly includes it.

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
