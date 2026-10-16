# Portable commands

**Unreleased:** commands are available in current source and have not been
qualified as a published CLI release.

A portable command is a Markdown prompt the user starts by hand. The package
author writes it once, and the `agentplugins` installer delivers it to every
client as a skill. The package stays an Agent Plugins 1.0 package: commands
live in the installer's own extension directory, which other tools ignore.

## Where commands live

```text
my-plugin/
  plugin.json
  skills/
    review/SKILL.md
  io.github.777genius.agentplugins/
    commands/
      deploy.md
      setup.md
```

The installer reads only `io.github.777genius.agentplugins/commands/`. Other
entries under `io.github.777genius.agentplugins/` keep their own meaning and
are not checked as commands. Inside `commands/`, any entry whose name starts
with `.` is ignored, so `.gitkeep` is fine.

`commands/` itself, and the namespace directory above it, must be real
directories. A symlink, a file, or more than 10,000 entries reports
`commands_root_invalid` and no command from the package is delivered.

## File format

```markdown
---
description: Deploy the current branch to the staging environment.
disable-model-invocation: true
---

Deploy the current branch to staging. Ask before running any command that
changes remote state, and report the deployment URL when you finish.
```

**File name.** `^[a-z0-9]+(-[a-z0-9]+)*\.md$`, and the stem must not be a
reserved Windows name (`con`, `prn`, `aux`, `nul`, `clock$`, `com1`-`com9`,
`lpt1`-`lpt9`). Anything else in `commands/` (other files, subdirectories,
symlinks, `.MD`) reports `command_name_invalid`.

**Frontmatter.** It is closed: exactly the two fields below, nothing else.
Client-specific fields such as `allowed-tools`, `argument-hint` or `model` are
rejected rather than dropped, because a field that one client honors and the
others ignore would change what the command does from client to client.

| Field | Rule |
| --- | --- |
| `description` | Required string, one line, not blank, at most 1,024 characters. Clients show it and may use it to choose the skill. |
| `disable-model-invocation` | Required YAML boolean. `true` makes the command explicit-only (see below). The quoted string `"true"` is rejected. |

**Body.** Everything after the frontmatter, passed through unchanged except
that CRLF becomes LF. The body must work the same in every client, so the
installer rejects syntax that clients expand differently. The checks cover the
whole text, fenced code included.

The file is at most 1 MiB and the frontmatter at most 64 KiB.

### Diagnostics

Every problem is an error with `boundary: command` and the file stem as
`item`. Each code is reported at most once per command, in this order, and a
frontmatter error does not stop the body checks.

| Code | Cause |
| --- | --- |
| `command_name_invalid` | File name or entry type, see above. No other check runs for that entry. |
| `command_read_failed` | The file could not be read. |
| `command_frontmatter_invalid` | No frontmatter, unreadable YAML, not a mapping, duplicate keys, or a size or nesting limit exceeded. |
| `command_unknown_field` | Any field other than the two above. |
| `command_description_invalid` | `description` missing, not a string, blank, longer than 1,024 characters, or more than one line. |
| `command_invocation_flag_invalid` | `disable-model-invocation` missing or not a YAML boolean. |
| `command_body_encoding` | Invalid UTF-8 or a NUL byte. |
| `command_body_empty` | Empty or whitespace-only body. |
| `command_plugin_variable` | A plugin root or data variable: `CLAUDE_PLUGIN_ROOT`, `CLAUDE_PLUGIN_DATA`, `CLAUDE_SKILL_DIR`, `COPILOT_PLUGIN_ROOT`, `CURSOR_PLUGIN_ROOT`, `DEVIN_PLUGIN_ROOT`, `PLUGIN_ROOT` or `PLUGIN_DATA` after `$` or `${`, or `${extensionPath}`. |
| `command_client_variable` | Any other `${CLAUDE_...}`, `${COPILOT_...}`, `${CURSOR_...}` or `${DEVIN_...}`. The braceless form, such as `$CLAUDE_NOTIFICATIONS_BIN`, is allowed. |
| `command_argument_placeholder` | `$ARGUMENTS`, `${ARGUMENTS}`, `$` directly followed by a digit (`$1`, also `$5`), or `{{args}}`. Describe the input in prose instead: "the text the user wrote after the command". |
| `command_shell_injection` | `` !` ``, `!{`, or a fence line opened with ```` ```! ```` or `~~~!`. |

The package itself adds three checks on the skill name the command becomes:
`command_projected_name_invalid`, `command_projected_name_too_long` (more than
64 characters) and `command_skill_name_collision` (the name matches an entry
under `skills/`, ignoring case).

An invalid command is never delivered. The rest of the package still installs
unless nothing else is left, and the errors appear in `agentplugins validate`
and in the plan of `add` and `update`.

## What a client receives

Each valid command becomes a skill named `<slug>-<name>`, where `<slug>` is
the plugin name with dots replaced by dashes. Command `deploy.md` in plugin
`acme.tools` becomes the skill `acme-tools-deploy` in every client. The name
never changes per client.

```markdown
---
name: "acme-tools-deploy"
description: "Deploy the current branch to the staging environment."
---

Deploy the current branch to staging. ...
```

The skill appears in plans as `kind: skill` with `source: command` and
`source_name: deploy`. The package never contains these skill files; the
installer writes them into the client's copy.

**Ordinary commands** (`disable-model-invocation: false`) reach every client
that takes skills, which today is all 13 supported clients. The user runs the
skill the way the client runs skills by hand. Like any skill, the model may
also choose it on its own.

**Explicit-only commands** (`disable-model-invocation: true`) reach a client
only when its adapter declares that the client enforces explicit invocation.
Delivering such a command as an ordinary skill would let the model run it
uninvited, so the installer withholds it instead:

- the command is no component of the plan;
- the plan carries the warning `explicit_invocation_not_enforceable` naming
  the command, and the compatibility row reports it unsupported with the same
  limitation;
- if the client would receive nothing else from the package, the plan is
  rejected with `no_supported_components`.

Authoring reports treat a withheld command as the warning
`target_command_withheld`, and as an error only when the client gets nothing
else.

## Client support for explicit-only commands

**No client enforces explicit invocation in this release, so explicit-only
commands are currently delivered nowhere.** A client is enabled only after a
probe with the real client shows that the skill still runs when the user
starts it and that the model does not pick it on its own. Documentation alone
is not enough: Cursor documents the mechanism, but in a reported bug it hides
plugin skills that use it. The Claude Code and Codex adapters already render their mechanisms
(`disable-model-invocation: true`, and `agents/openai.yaml` with
`policy.allow_implicit_invocation: false`), and stay disabled until their
probes pass.

The table records what each client documents or ships, as reviewed on
2026-10-16. It is not a statement of what `agentplugins` delivers.

| Client | Manual skill invocation | Explicit-only mechanism | Explicit-only commands delivered |
| --- | --- | --- | --- |
| Claude Code | `/skill`, plugin skills `/plugin:skill` | documented: `disable-model-invocation: true` | no, probe pending |
| OpenAI Codex | `$skill`, plugin skills `plugin:skill` | documented: `agents/openai.yaml`, `policy.allow_implicit_invocation: false` | no, probe pending |
| ChatGPT | `@` | not confirmed | no |
| Cursor | `/`, `@` | documented; plugin skills that use it are hidden (reported bug) | no |
| GitHub Copilot CLI | `/SKILL-NAME` | documented: `disable-model-invocation` | no, not probed |
| Visual Studio Code | `/` | documented: `disable-model-invocation` | no, not probed |
| Kiro | `/skill` | none found | no |
| Gemini CLI | `/skill` in source v0.64.0 | none found | no |
| OpenCode | the stable release exposes a skill as a command | not in the stable release; development source reads `disable-model-invocation` | no |
| Cline | `/skill` | none; its parser drops unknown fields | no |
| Windsurf / Devin | Devin: `/skill` | a different field (`triggers: [user]`) | no |
| Grok Build | `/<skill-name>`, a qualified form such as `/local:commit` on a name collision | not documented | no |
| Kimi Code | `/skill:<name>` | documented: `disableModelInvocation`, also spelled `disable-model-invocation` | no, not probed |

## Checking commands in CI

`agentplugins validate` keeps `conformant` about the Agent Plugins
specification and exits with success even when a command is invalid. Add this
check to fail the job on any invalid command (requires `jq`):

```bash
agentplugins validate ./package --format json \
  | jq -e '.result=="success" and ((.data.components.invalid_commands // 0) == 0) and ([(.data.diagnostics // [])[] | select(.boundary=="command" and .severity=="error")] | length == 0)'
```

This is the only copy of the snippet; link here rather than copying it.

## Updates, repair and saved dependencies

The installed skill depends on the package and on the installer's rendering
rules, so both kinds of change have to reach the client.

- **A new package revision.** `agentplugins update` stages it in full. A
  deleted command disappears from the client, and a changed flag takes effect:
  a command that became explicit-only is removed from clients that cannot
  enforce it.
- **The same revision after an installer upgrade.** `agentplugins update
  <name>` first refreshes the command skills with the current rules, then runs
  the ordinary update. When the rules produce the same tree, the installed
  files stay as they are. An embedded installer's update preview for such a
  package reports that it requires confirmation.
- **Repair** is for restoring a damaged installation, not for applying new
  rules; use `agentplugins update <name>` for that. Through the embedded
  installer, a single-target repair of an intact package whose command skills
  would now render differently stops with `usecase.ErrProjectionRefreshRequired`.
  If the package is damaged and the rules changed as well, remove and add it
  again.
- **Saved dependency environments** (`agentplugins deps`) do not use the
  refresh. New rendering rules reach those installations at the next pin
  change.

## Security review

The security review of the `agentplugins` CLI scans the package as clients
receive it: the package snapshot plus every command rendered as its skill. A signed Directory
verdict does not cover commands yet, so a package with commands is always
scanned locally or served from a cache entry keyed to its rendered commands.

## Not supported

- argument placeholders, shell injection and per-client fields (see above);
- aliases and nested command directories;
- authoring commands for one client in `plugin/targets/<client>/commands/`;
- per-client skill names: every client gets `<slug>-<name>`.
