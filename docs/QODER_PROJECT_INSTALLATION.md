# Qoder Project Skills and MCP delivery

**Current published CLI 0.1.101** supports Qoder Project delivery for eligible
Agent Plugins 1.0 Skills and stdio / Streamable HTTP MCP servers. This is a
client-specific **Project** path, not permission to rewrite the User profile or
register every plugin feature. SSE, hooks and unqualified extensions remain
unsupported. The published signed Directory is still the source authority
for short names, and the CLI validates exact dependencies and source bytes.

## Files, status and authority

For `--target qoder --scope project`, UAP manages the selected package in:

`<project>/.uap/managed/clients/qoder/<physical-artifact-id>/`

It can project selected Skills into `<project>/.qoder/skills/<name>/` and the
selected stdio / HTTP MCP servers into exact owned `mcpServers` members of
`<project>/.mcp.json`. Existing foreign config members and other projects
are preserved; conflicts with higher-priority settings are refused before
mutating files. No model execution, OAuth, or native trust is assumed.

A Skill package can additionally retain the legacy importable view:

`<managed-package>/.agentplugins-qoder-native/`

That view contains only the reviewed `.qoder-plugin/plugin.json` and eligible
`skills/<name>/` trees. This is a **separate optional native import route**:
UAP does not execute `qoder plugins install`. Do not import the whole managed
package or mistake a manual native cache for an owned UAP Project installation.

The result remains **Prepared** until the selected Qoder session performs
its own discovery and trust checks. File delivery and fresh native list
observations do not prove `tools/call`, provider authentication, or loaded
model execution. An activation attestation does not override this limit.

## Optional legacy import

From the selected project, after checking existing native registrations and
the shared cache, an operator may separately run:

```text
qoder plugins install <exact-native-view> --scope project --json
```

The older [Qoder 1.1.78 observation](evidence/project-scope-research/qoder-1.1.78-linux-amd64-40135008690/README.md)
found imports in the original user's global registry/cache. Such manually
imported copies have independent ownership and removal. They must not be
confused with the newer native Project Skills/MCP files.

## Maintenance and evidence

Project add, repeat, update, missing-component repair and remove use exact
recorded ownership. Removal deletes only owned Project Skills, exact selected
MCP members, and the managed package. It never removes foreign configuration,
the original User profile, or independently imported global Qoder copies.

The [composed CLI E2E](../cli/cmd/agentplugins/project_qoder_e2e_test.go)
covers Project A/B, mixed Skills+stdio/HTTP MCP, data receipts, foreign config,
update/repair/remove; the [native installed-source collector](../scripts/project_native_qoder_installed_mcp.py)
checks actual Qoder MCP listing in bounded pinned environments. Neither
provides blanket authorization to run third-party MCP tools or models.
