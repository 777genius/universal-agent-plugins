# Windsurf / current Devin Local project pre-code evidence

2026-10-23, cursor_clients. The original client plan and both independent GO
rounds remain authoritative. No Windsurf/Devin production code is changed at
this checkpoint. The implementation is the single newer dedicated-MCP lane,
separate from preserved legacy Cascade user behavior.

## Official pin and genuine bounded version/help

Root captured the official stable manifest from the vendor installer metadata,
without executing the installer. Exact version **3000.11.205**, Linux x86_64:

- https://static.devin.ai/cli/3000.11.205/devin-3000.11.205-x86_64-unknown-linux.tar.gz
- Vendor manifest and measured archive SHA256:
  `2e65c8b93682258fc91f5e4cead92e3698338cebf1b31dbc32a3201cb63705c4`
- Measured archive length: **73,679,375 bytes**.
- Complete official inventory: 250 directory/regular-file members,
  **218,025,070 bytes** total payload, including the single `bin/devin`
  executable and its exact-version official documentation.
- Binary length **217,494,824 bytes**, measured SHA256
  `642c82e32cb3dec90cf5ab01975520b425307872c7a650c9fb5b0653f37681f4`.

Ten actual version/help-only probes all exited zero: --version, --help,
mcp --help, skills --help, mcp list/get/add/remove --help, and skills
list/paths --help. Version is `devin 3000.11.205 (6dea39440ae3)`.

The complete distribution was mounted read-only in a fresh networkless
unshare-all namespace for each process, with empty /proc, fresh HOME/XDG/project,
DEVNULL stdin, minimal literal environment, bounded logs and deadlines. No host
user configuration or credentials were mounted/inherited. Exact argv and all
outputs/provenance are retained under task tools/native-probes/devin/3000.11.205.
The verified archive and tiny pinned official docs are retained; only the
reproducible218MB extraction was reclaimed after those observations.

No list/discovery, session, model, configured MCP server, auth, trust, migration,
plugin registry or account operation was run. Help is not native project proof.

## Exact-version and current official configuration references

The exact distribution's docs agree with the current canonical references:

- https://docs.devin.ai/cli/extensibility/mcp/configuration
- https://docs.devin.ai/cli/reference/configuration/config-file
- https://docs.devin.ai/cli/reference/configuration/global-vs-local
- https://docs.devin.ai/cli/reference/configuration/read-config-from
- https://docs.devin.ai/cli/extensibility/skills/overview
- https://docs.devin.ai/desktop/devin-local

Since3000.3/Local3.6 the current lane uses `.devin/mcp_config.json`, with a
separate higher local override `.devin/mcp_config.local.json`; user config is
independent. Native startup migrates old mcpServers in main config files. UAP
must not initiate migration or write the obsolete main config layout.

Native current MCP is nested mcpServers: command/args/env for stdio and
url/headers plus optional transport http/sse for remote. There is no documented
native cwd field; use the existing trusted launcher. Native MCP remove defaults
to local scope, so project removal uses exact file receipts and never invokes
that command. MCP list help exposes no --json flag. Skills list help does expose
--json, but no actual list was run. Skills can come from .devin, .windsurf,
.agents and configured/imported sources; configuration delivery alone cannot
prove effective names, permissions or selected Desktop harness behavior.

## Implementation boundary

Keep original user detection/channel selection and legacy Cascade mutation
unchanged. Require explicit selected primary Devin executable with exact
observed3000.11.205 version evidence for the new automatic project lane.
Legacy Windsurf primary, older/unknown/unobserved version or ambiguous selection
must not be reinterpreted as current Devin. The narrow host-version propagation
amendment is separately reviewed before code because the public installer
currently reconstructs a client without its observed Version and rejects an
absent original legacy profile even for validated project-only authority.

Project package/state/data stay under .uap; native skills use .devin/skills and
MCP uses only .devin/mcp_config.json. Validate exact context/artifact/binding/
receipt/data authority and reuse the existing reviewed no-follow native kernel
and component transaction. Refuse legacy main-config MCP requiring migration;
preserve local overrides and refuse same-name local shadowing rather than
changing policy. Other imported/user/ancestor/plugin policy remains unobserved.
Prepared+installed files cannot become active via generic attestation. Native
project discovery/runtime and Desktop UI qualification remain separate gates.
