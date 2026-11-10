# Cline native configuration routes

User-scope installation already writes and verifies owned Skills and MCP entries
through the existing native transaction and receipts. The default detected
profile is `~/.cline`. `ClineMCPSettingsPath` selects, in order,
`CLINE_MCP_SETTINGS_PATH`, `CLINE_DATA_DIR/settings/cline_mcp_settings.json`, or
`<profile>/data/settings/cline_mcp_settings.json`. Removal preserves foreign
entries and refuses ownership/path mismatches.

The [publisher's MCP documentation](https://github.com/cline/cline/blob/main/docs/mcp/mcp-overview.mdx)
(observed blob `ffaedb4e42ded2689c127b79710c36b6b327b74e`) identifies the
default CLI settings file and its explicit override. IDE extensions select
their actual file through **MCP Servers > Configure > Configure MCP Servers**.
Detecting an installed extension does not establish that it reads the CLI file.
The installer therefore asks the user to confirm the IDE's selected file; it
does not rewrite an editor profile or claim an IDE reload was observed.

Project MCP remains a separate prepared route:
`<project>/.uap/clients/cline/mcp.json`, selected for one CLI process with
`CLINE_MCP_SETTINGS_PATH`. It does not replace the user's MCP source or merge
credentials. Existing project launch guidance disables automatic approval.

## Observed native lifecycle

Pinned Cline **3.0.73**, Linux amd64, source
`6f19b03ac44f877a78433755023ce3592bb4f37e` completed the existing genuine
[native cycle](https://github.com/777genius/universal-agent-plugins/actions/runs/42679586506/job/129380490762).
Original native artifact `13704976222` contains `runtime-attempt.json`, exact
public receipts, native listings and ACP records. It records fresh Project A/B
installation, repeat add, unchanged update, changed-source update, missing-file
repair, removal and foreign preservation; User Skills remain independent.
Project MCP listings and sessions explicitly select the project settings file.
The User listing reads its foreign sentinel from
`~/.cline/data/settings/cline_mcp_settings.json` without an MCP settings override.
The controlled loopback fixture observed MCP initialize/list/call; vendor login
and a real model were not qualified. The reported activation remains Prepared.
The User fixture is Skills-only, so this run does not qualify a complete default
User MCP lifecycle or IDE discovery.

The publisher's [SDK README](https://github.com/cline/cline/blob/main/sdk/README.md)
also describes User Agent Plugins discovery under `~/.agents/plugins` and
explicit host opt-in through `agentPluginPaths` for additional roots. That is a
distinct host facility, not evidence that a pinned published CLI automatically
discovers Project plugins. This adapter keeps its existing owned delivery route.
