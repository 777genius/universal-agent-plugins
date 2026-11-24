# Kiro: narrowly scoped automatic MCP tool approval

Kiro already receives automatically installed, UAP-owned user Skills and MCP entries when its native setup is available. A project installation is **Prepared** until Kiro itself establishes workspace trust and discovery. The Kiro permissions system intentionally keeps workspace trust outside the cloned repository; UAP must not forge the `workspaceTrusted` record.

## One-command approved tools for publisher-declared packages

A package author who has reviewed the actual safe MCP tool set can add this optional Kiro-specific extension to the standard **plugin.json** (the portable **mcp.json** schema does not permit Kiro's `autoApprove` field):

```json
{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "example-docs",
  "version": "1.0.0",
  "extensions": {
    "com.universal-agent-plugins.kiro-mcp-approval": {
      "version": 1,
      "mcp_auto_approve": {
        "docs": ["read_docs", "find_docs"]
      }
    }
  }
}
```

The same package's portable `mcp.json` must declare the **docs** server. The package's source digest includes the approved names. An ordinary `agentplugins add <source> --target kiro` uses the confirmed package selection and projects the exact tool allowlist into **only the selected server's** Kiro-native `autoApprove` array in `~/.kiro/settings/mcp.json` (User) or `.kiro/settings/mcp.json` (Project). No Kiro-native JSON editing by the user is needed for these tools. The plan displays `docs/read_docs, docs/find_docs` before confirmation, with an explicit warning that it is granting per-tool consent.

The extension is optional. If absent, UAP does not automatically approve any tools. A package cannot set `"*"`, a glob pattern, duplicate tool names, unexpected extension fields, an unselected server, or a foreign extension and receive auto-approval. Tool names are exact, not pattern-based; vendor-level `deny` and `ask` rules retain priority. Never use this to approve tools with privileged writes, sensitive data access, or unreviewed side effects. Safe publication requires the publisher to verify those tool contracts.

On update/repair, the approval is regenerated from the exact selected and verified package source; a removed server loses its owned approval along with the owned MCP entry. UAP's exact native ownership digest protects user modifications and existing unrelated MCP servers, including their approvals.

## What this does **not** automate

**Workspace trust remains a separate Kiro-native decision.** Gated interactive Kiro CLI V3 sessions do not load project Skills or MCP servers until the workspace is trusted, even with an `autoApprove` list. Kiro's `--no-interactive` launches and older V1/V2 sessions are not gated and can load workspace configuration without asking; this does not grant persistent workspace trust. Kiro's documented `--trust-workspace` only skips the prompt for **one CLI invocation**, and does not persist a trust decision or grant IDE trust. There is no documented persistent, headless workspace-trust grant to use for later sessions. UAP does not synthesize the private workspace-roots hash or edit `workspaceTrusted`, grant global all-tools access, obtain credentials, or claim native tool execution without actual authenticated evidence.

Security references: https://kiro.dev/docs/permissions/ and https://kiro.dev/docs/mcp/configuration/ and https://kiro.dev/docs/mcp/security/.

## E2E acceptance

- [ ] Real package schema accepts the new Kiro extension without changing the portable MCP schema.
- [ ] CLI/SDK selected User and Project files contain only exact approved tools; no wildcard.
- [ ] Fresh, unchanged repeat, update, repair, removal, and user-edited/foreign collision cases preserve native receipts and third-party bytes.
- [ ] Native no-auth Linux, macOS and Windows checks pass on one exact SHA; evidence must not equate *prepared* with *runtime verified*.
- [ ] No unrelated client changes, global trust file writes, or automatic elevation through other targets.
- [ ] All review findings addressed; merge only on successful final-HEAD checks.
