# Hermes Agent integration

**Source status:** [PR #814](https://github.com/777genius/universal-agent-plugins/pull/814)
is merged. The extended native lifecycle passed on Linux at UAP head
`b3ed848effaca7d436c1306e06f2d24e67b9e0b2` with Hermes 0.23.0; exact evidence and
platform acceptance criteria are recorded below. This source result does not establish
qualification of released packages or full Directory compatibility.

The Hermes Agent target ID is `hermes`. This first implementation preserves UAP's
immutable-pinned package source, transaction journal and exact ownership while
materializing **Skills and a scoped subset of MCP servers** into the native locations Hermes documents:

| Scope | Native Skills destination | MCP handling |
| --- | --- | --- |
| `--scope user` | `$HERMES_HOME/skills/<name>/SKILL.md` | Exact-member `mcp_servers` edits in `config.yaml` (stdio and Streamable HTTP) |
| `--scope project` | `<project>/.hermes/skills/<name>/SKILL.md` | Explicitly refused: Hermes' current `mcp_servers` authority is user `config.yaml` |

The smoke selects an explicit isolated, nondefault `HERMES_HOME`. UAP captures
this override and `HERMES_DATA_DIR_SUFFIX` from the environment. Without an
override, the native defaults are `~/.hermes` on POSIX and
`%LOCALAPPDATA%\hermes` on Windows (or `~/AppData/Local/hermes` when
`LOCALAPPDATA` is empty), with the literal suffix appended to the directory
name. Platform-default selection has separate injected-host tests; the native
lifecycle verifies the explicit profile. Existing legacy directories do not
change the selected authority. See the [profile correction source review](plans/2026-11-10-hermes-windows-profile.md).

Hermes honors project Skills only after its own project trust conditions. UAP
never grants trust or equates installed filesystem bytes with live availability.

**Strict scope:** source-verified Skills and User MCP install/update/repair/remove,
no reuse of foreign skill/MCP names. User MCP uses the existing atomic native
config kernel, strict bounded YAML parsing, per-entry receipts, and
post-write verification while preserving unrelated settings and comments.
An ambiguous YAML document, foreign edit, symlink, unknown tag/anchor or
unsupported transport fails closed. No user YAML is ever changed by Project
MCP. Packages requiring hooks or unqualified components are refused.

**Source acceptance gate:** the same native lifecycle must pass on Linux,
macOS and Windows at the candidate head. End-user release and full Directory
compatibility remain separate qualification gates before promoting or
advertising complete Hermes support; native CI alone does not satisfy them.
The native smoke proves MCP listing, connection and tool discovery; it does not
perform a tool call, a model turn or an authenticated provider session. Project
discovery is observed without granting trust, so it does not establish trusted
Project Skill execution.

Sources: Hermes [Skills system](https://hermes-agent.nousresearch.com/docs/user-guide/features/skills/)
and [MCP](https://hermes-agent.nousresearch.com/docs/user-guide/features/mcp/).
Implementation criteria: [reviewed plan](plans/2026-11-09-hermes-native-integration.md).

## Native CI acceptance

`Hermes Native Skills and MCP E2E` checks out NousResearch/hermes-agent at exact
commit `2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19` (v0.23.0), installs it inside an
isolated CI home with browser/computer use disabled, and builds UAP from the PR.
The workflow acceptance contract uses the same lifecycle on Linux, macOS and
native Windows, with the pinned official installer for each platform:

- User first/repeated add, update, repair and removal, with actual native Skill
  catalog lookup and content reading through the installed Hermes runtime.
- Native `hermes skills list`, `hermes mcp list`, and four `hermes mcp test`
  connections to a bounded loopback fixture, observing initialize/tools/list.
- Project A/B isolation, update and repair of A while B retains its original
  Skill, native untrusted-project discovery and removal.
- Structured unsupported Project MCP refusal with `mutated: false`, unchanged
  User YAML bytes and trust, and preservation of foreign Skills/MCP entries.

| Native platform | Evidence |
| --- | --- |
| Linux x64 | [Run 42802367363](https://github.com/777genius/universal-agent-plugins/actions/runs/42802367363) passed at the UAP head above; [retained evidence](https://github.com/777genius/universal-agent-plugins/actions/runs/42802367363/artifacts/13757427815) records 13 native Skill observations and four MCP initialize/tools/list exchanges. |
| macOS arm64 | The same pinned lifecycle is included in the workflow; qualification requires its successful run at the candidate head. |
| Native Windows | Qualification requires the same lifecycle through Hermes' native PowerShell installation at the candidate head; WSL or POSIX-lane success is not Windows evidence. |

The pinned Hermes [installation documentation](https://github.com/NousResearch/hermes-agent/blob/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19/README.md#quick-install)
supports Linux, macOS and native Windows; Windows is not WSL-only. Matrix artifacts
record the actual UAP checkout and PR head separately, the Hermes source pin,
runner OS/architecture, lifecycle exit and observed MCP methods. A queued,
skipped or failed run is not qualification, and earlier Linux evidence cannot
qualify a newer candidate head or another operating system. Keep platform and
release promotion gated on the applicable current-head results.
