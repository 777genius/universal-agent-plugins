# Google Antigravity native plugins

**Source tracking:** PR #815. Merging source or passing native CLI tests does not establish a published UAP release, model authentication, or Antigravity IDE activation.

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
The staged portable `mcp.json` is removed after native projection because the
native manifest loads only `mcp_config.json`; retaining both fails strict vendor
validation. The original portable MCP file remains in the verified source.
Portable Skills remain in `skills/<name>/SKILL.md`. The immutable source
package is not modified. One native manifest avoids strict validation warnings
about a second, ignored root manifest.
No Project command falls back to mutating User configuration.

For stdio MCP, UAP validates and expands authored `${PLUGIN_ROOT}` and
`${PLUGIN_DATA}` references in arguments, working directory and custom
environment values to its committed plugin and owned data paths. Native
configuration omits reserved `PLUGIN_ROOT` / `PLUGIN_DATA` environment entries:
Antigravity supplies those process variables itself and ignores overrides.
Its runtime `PLUGIN_DATA` identifies a vendor-managed persistent directory,
which is separate from UAP's explicitly expanded data paths. UAP does not claim
that these directories are identical or manage the vendor's runtime data.

These targets remain source capabilities until a qualified CLI release includes
them. Public install actions retain the released target set; the SDK-generated
capability inventory still describes every source adapter.

**Security:** Reject unqualified hooks, rules, subagents, unknown components,
unexpected raw `mcp_config.json` and unsafe remote MCP destinations.
Never silently execute downloaded hook scripts or copy foreign native files.
An Antigravity plugin folder being present does not prove it was indexed,
enabled, trusted or authenticated. Report **Prepared**, not Active.
A pinned `agy 1.5.3` User-scope smoke confirms that native validation can
succeed while `agy plugin list --json` does **not** show the UAP-prepared User
bundle. UAP does not silently invoke `agy plugin install`: that vendor
registration can have additional, separately owned side effects and must
remain a deliberate user-managed action until a safe transactional lifecycle
is independently qualified. Do not report automatic User activation from a
successful UAP file install; UAP removes only the directory it owns.

**Merge qualification:** Require passing pinned `agy plugin validate --strict`,
separate disposable vendor CLI import/list/uninstall, UAP User and Project
add/remove with independent ownership, complete applicable cross-OS CI, support
metadata/site compatibility and published-CLI checks on the exact final commit. The CLI installed-plugin list is **not** evidence
of workspace IDE discovery: project `.agents/plugins/` loading requires an
actual IDE observation and must not be reported as proven by shell tests.
There is no claim of support for all Antigravity native hook and agent kinds.

Sources: [Build plugins](https://antigravity.google/docs/build-plugins),
[Customization discovery](https://antigravity.google/docs/customization-discovery),
[MCP](https://antigravity.google/docs/mcp).
Implementation criteria: [reviewed plan](plans/2026-11-09-antigravity-native-integration.md).

### Native CI acceptance

`Antigravity Native Plugin E2E` builds the UAP CLI from the PR and uses the real `agy` release 1.5.3 from the vendor GitHub repository, requiring SHA-256 `1b4850599cce73a17f94ffb422df3bf9531531b77dbddc40c1a435e0c2c16807` on the Linux x64 archive. It verifies strict native plugin validation, local CLI import/list/uninstall in a **separate disposable CLI profile**, UAP project-scoped removal and preservation of foreign User state. CLI listing is not presented as project-workspace discovery or host activation evidence. A queued or failed run cannot be cited as passed, and a validated package alone does not prove authentication or MCP `tools/call`.

Cross-platform qualification additionally pins the official agy 1.5.3 macOS
x64/arm64 and Windows x64 release archives by SHA-256. These jobs repeat the
native CLI lifecycle in a separate disposable vendor profile and UAP's scoped
User and Project add/remove, while preserving a foreign User plugin. None of these
shell/CLI checks claims that the Antigravity IDE loaded a project plugin.

The native fixtures include Skills plus stdio and Streamable HTTP MCP. They
exercise idempotent update, refusal of partially modified packages without
changing their bytes, and exact-revision recovery of an entirely missing owned
bundle. A partial digest mismatch remains an ownership refusal; the smoke does
not weaken that gate or report partial-file repair as supported.

### Actual IDE checkpoint: sign-in boundary (2026-11-10)

A disposable Linux x64 session using the [official IDE download](https://antigravity.google/download)
reached the native **Continue with Google** view. The exact observed product was
Antigravity IDE **2.6.0**, commit
`9e3a5da5d6731effd57d47d88bbe5df14ac6ccdd`, with the real backend **1.5.5**
(binary SHA-256
`89f16bdb87f62e6d07484c52e6bafe76a81b0fd13d740110c77dc3c3c8d8d8b6`).
[Sanitized checkpoint metadata](evidence/antigravity-ide-2026-11-10/ide-qualification.json)
records the IDE archive digest, official backend URL and verified archive
SHA-512, versions, observations and proof limits.

The initial backend updater network failure was resolved by ordinary cache
prefetch of the verified official archive. The native UI then remained signed
out (`signedOut`), and a command-palette search for Customizations returned no
matching commands. No vendor patch, authentication bypass, disabled TLS or
credentials were used. A UAP source binary was available but was **not executed**
in this IDE session; this checkpoint is not a UAP installation failure.

**Result: blocked on legitimate owner Google sign-in.** No User or Project
plugin discovery, enablement, A/B isolation, update/restart or removal lifecycle
was observed. No MCP tool call or model turn is qualified. After sign-in, actual
IDE observations must separately exercise the documented
[Customization discovery](https://antigravity.google/docs/customization-discovery)
and [plugin](https://antigravity.google/docs/build-plugins) surfaces in both
scopes. This Linux IDE checkpoint does not extend to macOS or Windows.

Keep this result distinct from the successful three-OS **`agy` 1.5.3 CLI**
strict validation and disposable import/list/uninstall evidence in
[run 42804680737](https://github.com/777genius/universal-agent-plugins/actions/runs/42804680737).
That CLI run does not qualify IDE 2.6.0/backend 1.5.5 discovery or activation;
this signed-out IDE checkpoint does not invalidate that narrower CLI proof.
