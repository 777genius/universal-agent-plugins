# Cline 3.0.73: genuine hosted configuration observations

> **Historical snapshot.** The observations and qualifications below belong to
> their recorded source. The [original repository directory](https://github.com/777genius/universal-agent-plugins/tree/3b93148c356fc5db6fe101d81d943160a5176b6d/docs/evidence/project-scope-research/cline-3.0.73-linux-amd64-40017958131) preserves the
> files present before raw-output cleanup. Adjacent metadata and source remain;
> original manifests describe that snapshot, not the current directory inventory.
> Retrieve removed raw files there; see the [project evidence index](../../PROJECT_INSTALLATION.md).

Recorded and independently interpreted on 2026-10-24. This is historical
pre-adapter research, measured at source
`bb8c89bf62542096a0af3aff26f21a16fee4d595`, from
[run 40017958131](https://github.com/777genius/universal-agent-plugins/actions/runs/40017958131)
and [job 120634802143](https://github.com/777genius/universal-agent-plugins/actions/runs/40017958131/job/120634802143).
The successful artifact is `cline-3.0.73-precode-config-linux-amd64`, ID
12532307825. Its ZIP was 9,915 bytes, SHA256
`cec4b3dd3bf2aa7a8572c7d517f06c3529837d5e01445326d00d4a2d4f27c68c`.
The 29 flat text members are preserved byte-for-byte in the original repository
snapshot; the binary ZIP is not committed. `provenance.json` records each member's length and SHA256.

## Verification and source attribution

The implementing worker and root independently checked all 12 native stdout
and stderr length/hash pairs, exact native version 3.0.73, successful process
results, clean JSON-command stderr and the before/after fixture inventories.
The worker also checked both Git-preparation stream pairs. All 28 recorded
streams match. The inventories are equal; native-created state is empty.
The original summary remains unchanged, including both false qualification
fields. No native process or acquisition was rerun during this interpretation.

The Cline collector, acquisition pins, dedicated workflow and Grok inventory/
budget module are byte-identical to comparison checkpoint
`4cba9df5cab26f12fafa4c0e68a119953485e652`. The imported shared capture helper
is **different**: the current reviewed helper enforces separate live stream
caps. The imported native matrix module also differs through added project
suite selection; its hosted admission function was not changed by that diff.
`provenance.json` records every compared blob. These observations belong to the
historical source closure, not the current head's native gate.

## Observed native behavior

All fixture registrations are disabled remote loopback markers; skills contain
inert text. Every child used literal `CLINE_NO_AUTO_UPDATE=1`, selected cwd and
`--auto-approve false`. The following table reports actual populated paths;
`<root>` is `/tmp/cline-native-research-3c04fr3x` in the raw output.

| Probe | Actual native selection | Consequence for the adapter |
| --- | --- | --- |
| `default-a`, `default-b`, `default-a-again` | Only `uap-cline-user` at `<root>/home/.cline/data/settings/cline_mcp_settings.json` | Working directory alone did not select either prepared project MCP file. A later ordinary process returned to the user source. |
| `override-a` | `uap-cline-a` at `<root>/project-a/.uap/clients/cline/mcp.json` | An explicit process override can select A's prepared file. |
| `override-b` | `uap-cline-b` at `<root>/project-b/.uap/clients/cline/mcp.json` | An explicit process override can select B's prepared file. |
| `cross-a-from-b` | A's marker and A's exact file, while cwd is B | MCP source selection follows the override independently of cwd. A launch action must fix both. |
| `skills-a` | A's exact `.cline/skills/uap-cline-a/SKILL.md` and the user skill | Native skill inventory found A plus user and omitted B. |
| `skills-b` | B's exact `.cline/skills/uap-cline-b/SKILL.md` and the user skill | Native skill inventory found B plus user and omitted A. |

MCP rows contain `name`, `transportType`, `disabled` and `path`. A/user uses
`streamableHttp`; B uses `sse`; all are disabled. Skills contain actual path,
frontmatter/instructions and description. The exact-tag primary implementation
supports those observed config branches:
[config.ts](https://github.com/cline/cline/blob/cli-v3.0.73/apps/cli/src/commands/config.ts).
An empty skills list would not establish discovery because that source catches
some enumeration errors; the positive path-bearing markers here matter.

## Independent interpretation

Both `skillshare_scope` and root concluded **GO for a concrete post-observation
project adapter plan only**. The observed component boundary is project skill
files plus explicitly process-selected prepared MCP configuration. It does not
establish a native project plugin registry or ordinary project MCP startup.

No MCP initialize/tool call, enabled skill execution, IDE behavior, trust,
authentication, model request, hook/JS/TS plugin, ancestor lookup, same-name
user/project precedence or runtime network isolation is proved. Unique markers
were used; do not infer conflict precedence from their coexistence. UAP
installation/receipts/update/repair/remove were not exercised by this collector.
A new current-source hosted fixture remains necessary for those product claims.
