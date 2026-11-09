# Hermes Agent integration (draft source)

**Release status: Draft PR #814; no shipping or native runtime claim yet.**

The Hermes Agent target ID is `hermes`. This first implementation preserves UAP's
immutable-pinned package source, transaction journal and exact ownership while
materializing **Skills** into the directories Hermes documents:

| Scope | Native Skills destination | MCP handling |
| --- | --- | --- |
| `--scope user` | `~/.hermes/skills/<name>/SKILL.md` (or selected `HERMES_HOME`) | **Not yet implemented in this PR** |
| `--scope project` | `<project>/.hermes/skills/<name>/SKILL.md` | Explicitly refused: Hermes' current `mcp_servers` authority is user `config.yaml` |

Hermes honors project Skills only after its own project trust conditions. UAP
never grants trust or equates installed filesystem bytes with live availability.

**Strict scope:** source-verified Skill install/update/repair/remove, no reuse of
foreign skill names, no rewriting the user's YAML credentials. Packages that
require MCP or another unqualified component must fail rather than silently
dropping the component. Use a package whose required features are only Skills.

**Not yet qualified:** profile-YAML MCP mutation, native Hermes observation,
cross-platform E2E with real Hermes executable, end-user release and full
Directory compatibility. These are required before the PR can be promoted or
advertised as complete Hermes support.

Sources: Hermes [Skills system](https://hermes-agent.nousresearch.com/docs/user-guide/features/skills/)
and [MCP](https://hermes-agent.nousresearch.com/docs/user-guide/features/mcp/).
Implementation criteria: [reviewed plan](plans/2026-11-09-hermes-native-integration.md).
