# Hermes Agent integration

**Source status (2026-11-10):** the native lifecycle passed on Linux x64,
macOS arm64 and native Windows x64 with Hermes 0.23.0 at UAP head
`a50c8a245f1c273efb3d171f4c54ed540475336a` in
[run 42813277862](https://github.com/777genius/universal-agent-plugins/actions/runs/42813277862).
This is source-qualified evidence. Published CLI `0.1.102` does not contain
Hermes or Antigravity. Qualification of an immutable public release artifact
and signed Directory eligibility remain separate, pending boundaries; native
CI does not authorize additional Directory targets.

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

**Source acceptance gate:** the recorded three-platform run satisfies the native
lifecycle gate for its exact UAP head only. A later candidate and the frozen
release binary need their own applicable qualification. The native smoke proves
MCP listing, connection and tool discovery over loopback Streamable HTTP; it
does not perform `tools/call`, a model turn or an authenticated provider session.
Project discovery is observed without granting trust, so it does not establish
trusted Project Skill execution. Portable commands are projected as Skills;
explicit-only commands are withheld because Hermes cannot enforce explicit
invocation. SSE and hooks remain unsupported.

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
| Linux x64 | PASS in [run 42813277862](https://github.com/777genius/universal-agent-plugins/actions/runs/42813277862), at the exact UAP head above; [Linux evidence](https://github.com/777genius/universal-agent-plugins/actions/runs/42813277862/artifacts/13762793764). |
| macOS arm64 | PASS in the same run and at the same UAP head; [macOS evidence](https://github.com/777genius/universal-agent-plugins/actions/runs/42813277862/artifacts/13763179484). |
| Native Windows x64 | PASS through the pinned native PowerShell installation in the same run and at the same UAP head; [Windows evidence](https://github.com/777genius/universal-agent-plugins/actions/runs/42813277862/artifacts/13763404124). |

Each OS passed User Skills/MCP first and repeated add, update, repair and
removal, Project Skills untrusted discovery and A/B isolation, and four native
loopback HTTP `initialize`/`tools/list` exchanges. The result does not extend
to Project MCP, trusted execution, tool calls, model use or provider login.

### Earlier Linux checkpoint

[Run 42802367363](https://github.com/777genius/universal-agent-plugins/actions/runs/42802367363)
passed on Linux at `b3ed848effaca7d436c1306e06f2d24e67b9e0b2` with Hermes 0.23.0;
its [retained evidence](https://github.com/777genius/universal-agent-plugins/actions/runs/42802367363/artifacts/13757427815)
records 13 native Skill observations and four MCP initialize/tools/list
exchanges. This historical result retains its original identity; the newer
three-platform run supplies the current source checkpoint.

The pinned Hermes [installation documentation](https://github.com/NousResearch/hermes-agent/blob/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19/README.md#quick-install)
supports Linux, macOS and native Windows; Windows is not WSL-only. Matrix artifacts
record the actual UAP checkout and PR head separately, the Hermes source pin,
runner OS/architecture, lifecycle exit and observed MCP methods. A queued,
skipped or failed run is not qualification, and earlier Linux evidence cannot
qualify a newer candidate head or another operating system. Keep platform and
release promotion gated on the applicable current-head results.
