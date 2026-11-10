# Google Antigravity native plugins (draft source)

**Release status: Draft PR #815; no published CLI or runtime-activation claim yet.**

The Antigravity target ID is `antigravity`. The adapter prepares one
source-verified package in the vendor's documented **plugin discovery folder**,
rather than calling `agy plugin install` with untracked filesystem effects:

| Scope | Native plugin directory |
| --- | --- |
| User | `~/.gemini/config/plugins/<managed-artifact>/` |
| Project | `<project>/.agents/plugins/<managed-artifact>/` |

Inside the isolated verified staging copy it projects the portable `plugin.json`
into one schema-tagged native root `plugin.json`, and writes `mcp_config.json` with
`mcpServers` (stdio or Streamable HTTP with `serverUrl`).
Portable Skills remain in `skills/<name>/SKILL.md`. The immutable source
package is not modified. One native manifest avoids strict validation warnings
about a second, ignored root manifest.
No Project command falls back to mutating User configuration.

**Security:** Reject unqualified hooks, rules, subagents, unknown components,
unexpected raw `mcp_config.json` and unsafe remote MCP destinations.
Never silently execute downloaded hook scripts or copy foreign native files.
An Antigravity plugin folder being present does not prove it was indexed,
enabled, trusted or authenticated. Report **Prepared**, not Active.

**Still to qualify before Ready:** pinned real `agy plugin validate --strict`,
separate CLI local import/list/uninstall in a disposable profile,
project-scoped UAP add/remove and foreign-state preservation,
complete CI on applicable OS targets, support metadata/site compatibility,
and published CLI tests. The CLI installed-plugin list is **not** evidence
of workspace IDE discovery: project `.agents/plugins/` loading requires an
actual IDE observation and must not be reported as proven by shell tests.
There is no claim of support for all Antigravity native hook and agent kinds.

Sources: [Build plugins](https://antigravity.google/docs/build-plugins),
[Customization discovery](https://antigravity.google/docs/customization-discovery),
[MCP](https://antigravity.google/docs/mcp).
Implementation criteria: [reviewed plan](plans/2026-11-09-antigravity-native-integration.md).

### Native CI acceptance

`Antigravity Native Plugin E2E` builds the UAP CLI from the PR and uses the real `agy` release 1.5.3 from the vendor GitHub repository, requiring SHA-256 `1b4850599cce73a17f94ffb422df3bf9531531b77dbddc40c1a435e0c2c16807` on the Linux x64 archive. It verifies strict native plugin validation, local CLI import/list/uninstall in a **separate disposable CLI profile**, UAP project-scoped removal and preservation of foreign User state. CLI listing is not presented as project-workspace discovery or host activation evidence. A queued or failed run cannot be cited as passed, and a validated package alone does not prove authentication or MCP `tools/call`.
