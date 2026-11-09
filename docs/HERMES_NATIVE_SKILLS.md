# Hermes Agent integration (draft source)

**Release status: Draft PR #814; no shipping or native runtime claim yet.**

The Hermes Agent target ID is `hermes`. This first implementation preserves UAP's
immutable-pinned package source, transaction journal and exact ownership while
materializing **Skills and a scoped subset of MCP servers** into the native locations Hermes documents:

| Scope | Native Skills destination | MCP handling |
| --- | --- | --- |
| `--scope user` | `~/.hermes/skills/<name>/SKILL.md` (or selected `HERMES_HOME`) | Exact-member `mcp_servers` edits in `config.yaml` (stdio and Streamable HTTP) |
| `--scope project` | `<project>/.hermes/skills/<name>/SKILL.md` | Explicitly refused: Hermes' current `mcp_servers` authority is user `config.yaml` |

Hermes honors project Skills only after its own project trust conditions. UAP
never grants trust or equates installed filesystem bytes with live availability.

**Strict scope:** source-verified Skills and User MCP install/update/repair/remove,
no reuse of foreign skill/MCP names. User MCP uses the existing atomic native
config kernel, strict bounded YAML parsing, per-entry receipts, and
post-write verification while preserving unrelated settings and comments.
An ambiguous YAML document, foreign edit, symlink, unknown tag/anchor or
unsupported transport fails closed. No user YAML is ever changed by Project
MCP. Packages requiring hooks or unqualified components are refused.

**Not yet qualified:** real Hermes CLI MCP listing / tool call,
cross-platform E2E with real Hermes executable, end-user release and full
Directory compatibility. These are required before the PR can be promoted or
advertised as complete Hermes support.

Sources: Hermes [Skills system](https://hermes-agent.nousresearch.com/docs/user-guide/features/skills/)
and [MCP](https://hermes-agent.nousresearch.com/docs/user-guide/features/mcp/).
Implementation criteria: [reviewed plan](plans/2026-11-09-hermes-native-integration.md).

### Native CI acceptance

`Hermes Native Skills and MCP E2E` checks out NousResearch/hermes-agent at exact commit `2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19` (v0.23.0), installs it inside an isolated CI home with browser/computer use disabled, builds the UAP CLI from the PR, and verifies `hermes mcp list`, User Skills, Project Skills, Project MCP refusal and owned removal. A queued or failed run is **not** qualification; keep this PR Draft until it passes and the actual client/tool states are checked.
