# VS Code workspace evidence: primary source notes

2026-10-24. Research only, pending the genuine pinned publisher help/version
artifact. This document does not approve a workspace collector or adapter.
The narrow owner exception permits project installation work; all unrelated
VS Code user/Local/TEST/hook implementation remains frozen and preserved.

## Current documented harness boundaries

Official documentation was reread with two search systems and checked against
source at the actual publisher commit
`e646868227c3e3a9e3f24277c8c7220be42c0be7` (VS Code 1.143.1).

| Primary reference | Consequence for the next plan |
| --- | --- |
| [MCP configuration reference](https://code.visualstudio.com/docs/agents/reference/mcp-configuration) | Workspace `.vscode/mcp.json` uses `servers`; portable workspace `.mcp.json` uses `mcpServers`. Copilot reads the portable format and receives eligible editor-format servers through forwarding. `code --add-mcp` targets the user profile. Neither a CLI user install nor a portable Copilot file qualifies the selected Local workspace route. |
| [MCP destinations](https://code.visualstudio.com/docs/agent-customization/mcp-servers) | The legacy workspace destination remains supported for Local capabilities; the guided flow labels it deprecated. Preserve that compatibility boundary rather than silently migrate or toggle an experimental setting. |
| [Agent skills](https://code.visualstudio.com/docs/agent-customization/agent-skills) | Documented repository skill directories include `.github/skills`, `.agents/skills` and `.claude/skills`. Duplicate names across workspace roots and parent-repository discovery have their own selection rules. File delivery cannot prove effective skill use. |
| [MCP operations](https://code.visualstudio.com/docs/agents/run/manage-mcp-servers) | A setting controlling editor autostart does not stop another harness from starting its independently configured servers. A future fixture must keep the harness and real observation explicit. |

No literal credentials, user inputs or profile secrets should be copied into a
workspace. Existing inputs, sandbox fields, unrelated servers and enablement
must be preserved under exact member ownership. A native help/version result
cannot prove any of these workspace policies.

## Pinned source checked, without execution

The following files were retrieved directly from the immutable publisher commit.
These source observations support future design; they are not runtime evidence.

| Source at `e6468682` | Checked boundary |
| --- | --- |
| [mcpConfigurationDestination.ts](https://github.com/microsoft/vscode/blob/e646868227c3e3a9e3f24277c8c7220be42c0be7/src/vs/workbench/contrib/mcp/browser/mcpConfigurationDestination.ts) | Destination validation requires a file rooted in an open workspace folder; root-format add is gated while legacy-format choice remains available. The legacy guided destination receives its deprecated label. |
| [mcpConfiguration.ts](https://github.com/microsoft/vscode/blob/e646868227c3e3a9e3f24277c8c7220be42c0be7/src/vs/workbench/contrib/mcp/common/mcpConfiguration.ts) | Editor-format schema defines stdio cwd/default workspace handling, HTTP/SSE transport, inputs and sandbox capabilities. Do not treat the portable schema as interchangeable. |
| [workspaceDotMcpDiscovery.ts](https://github.com/microsoft/vscode/blob/e646868227c3e3a9e3f24277c8c7220be42c0be7/src/vs/workbench/contrib/mcp/common/discovery/workspaceDotMcpDiscovery.ts) | Watches each workspace-root portable file with an explicit collection identity, folder provenance and workspace storage scope. Its parser is separate from the editor-format route. |
| [installedMcpServersDiscovery.ts](https://github.com/microsoft/vscode/blob/e646868227c3e3a9e3f24277c8c7220be42c0be7/src/vs/workbench/contrib/mcp/common/discovery/installedMcpServersDiscovery.ts) | Installed editor-format registrations are attributed to actual configuration resource and workspace folder; stdio default cwd and roots use that folder. User/unknown defaults are explicit fallbacks, not evidence of a selected project. |
| [mcpTypes.ts](https://github.com/microsoft/vscode/blob/e646868227c3e3a9e3f24277c8c7220be42c0be7/src/vs/workbench/contrib/mcp/common/mcpTypes.ts) | User profile, remote, workspace-folder editor format and portable workspace format have distinct provenance. Exact collection/resource attribution matters more than a matching displayed server name. |
| [mcpRegistry.ts](https://github.com/microsoft/vscode/blob/e646868227c3e3a9e3f24277c8c7220be42c0be7/src/vs/workbench/contrib/mcp/common/mcpRegistry.ts) | Workspace server use checks workspace trust before per-server trust behavior. A collection marked trusted is not permission to bypass the workspace decision. |
| [promptFileLocations.ts](https://github.com/microsoft/vscode/blob/e646868227c3e3a9e3f24277c8c7220be42c0be7/src/vs/workbench/contrib/chat/common/promptSyntax/config/promptFileLocations.ts) | Built-in skill roots distinguish the three workspace directories from three personal directories. Static availability does not prove the selected Local harness loaded an exact skill. |

## Existing UAP boundary to preserve

Current `vscode.New()` delegates plugin lifecycle/identity to the user Copilot
backend and must remain intact. `NewLocal` is a separate explicitly TEST-qualified
selected-profile plugin/hook route (old 1.140.0 tuples), not a project workspace
MCP adapter. Reusing that TEST qualification for 1.143.1 or relabeling it as native
workspace proof is invalid. The approved original all-client plan calls for a
new dedicated project adapter registered only in project composition, one local
filesystem folder initially, explicit Local harness, exact component receipts
and no user Copilot install/uninstall side effects.

After the genuine publisher artifact is available, independently interpret its
exact version/commit/help/namespace results. Then freeze a new concrete workspace
observation and adapter plan, obtain both plan-review rounds, and independently
review any new collector before hosted execution. A shipped-host extension/UI
observation needs its own bounded execution and authority contract; a synthetic
extension-host protocol or `code --version` cannot substitute. Do not use new
trust grants, default editor launch, `--add-mcp`, profile redefinition or another
harness to hide that remaining evidence gate.
