# Current source project support

**Source only / unreleased.** This table describes the project-installation
feature source. It does not change the availability or evidence of published
releases. Select `--scope project` and an existing project directory; the default
remains `user`. [Project installation](PROJECT_INSTALLATION.md) explains the
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
