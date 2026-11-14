# Windsurf and current Devin project delivery

The user lane preserves legacy Cascade channel selection and its exact owned
MCP configuration. User skills remain prepared as before. Detection ordering,
Devin/Windsurf coexistence selection and cloud configuration are unchanged.

The project lane requires the explicitly selected primary `devin`/`devin.exe`
with host-observed normalized exact version `3000.11.205`. Other/unknown versions
and a primary legacy Windsurf executable refuse before native effects. The
optional SDK `Request.ClientVersion` carries the embedding host's observation;
SDK Discover/Prepare and this adapter perform no version probe. This assertion
qualifies only the configuration format, not the executable's provenance.

| Delivery | Selected project location |
| --- | --- |
| Managed package, state, receipts and persistent data | `.uap` |
| Skill directories | `.devin/skills/<name>` |
| MCP members | `.devin/mcp_config.json` under `mcpServers` |

Current Devin uses command/args/env for stdio without a native cwd field. The
existing trusted managed launcher carries the package/data/cwd contract. Remote
servers use url/headers and explicit transport http or sse through a separate
native codec; legacy Cascade serverUrl behavior is preserved. Active projection
is read-only and rejects stored launcher/env locators that disagree with the
selected package/data receipt instead of silently accepting old data authority.

A current-only installation can legitimately have no legacy Cascade user root.
That absence is allowed only in the validated project SDK context; nonempty
roots and stricter adapter profile resolvers retain their existing validation.
Exact original user context, selected project/native root, artifact path,
binding and every native receipt remain checked. Sibling facts keep their
existing strict root/executable/binding requirements.

UAP refuses project main-config legacy mcpServers requiring native migration
and same-name higher local MCP overrides. It preserves local overrides, disabled
foreign members, skills, hooks, imported roots, trust and permissions. The shared
native kernel and component transaction supply exact owned updates/removal and
skill rollback. Recorded exact removal is version independent and launches no
native command; Devin's default-local `mcp remove` is never used.

Installed files report prepared activation with installation verification.
Generic activation attestation cannot claim native discovery. Effective aliases,
imports, local/user/cloud/team policy, trust, account state, Desktop UI, MCP
initialization and runtime remain unobserved. Genuine pinned Linux version/help
outputs and artifact hashes are recorded in the project evidence document;
composed inert UAP tests do not qualify native discovery or execution.
