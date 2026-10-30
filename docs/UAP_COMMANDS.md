# Portable commands

**Released in CLI 0.1.93:** the installer reads and validates portable commands
and delivers eligible command skills according to each client's support rules.
This release availability is separate from native runtime qualification; the
withholding and client limitations below still apply.

A portable command is a Markdown prompt the user starts by hand. The package
author writes it once, and the `agentplugins` installer delivers it to every
compatible client as a skill. The package stays an Agent Plugins 1.0 package: commands
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
symlinks, `.MD`) reports `command_name_invalid`. A reserved stem such as
`con.md` does not get that far with the installer: `agentplugins validate`,
`add` and `update` cannot read a package that holds a reserved name anywhere,
and stop with the general error `snapshot package content failed` before any
command check runs.

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
| `command_description_invalid` | `description` missing, not a string, blank, longer than 1,024 characters, more than one line, or holding a control character (NUL included) or a Unicode line or paragraph separator. |
| `command_invocation_flag_invalid` | `disable-model-invocation` missing or not a YAML boolean. |
| `command_body_encoding` | Invalid UTF-8 or a NUL byte. |
| `command_body_empty` | Empty or whitespace-only body. |
| `command_plugin_variable` | A plugin root or data variable: `CLAUDE_PLUGIN_ROOT`, `CLAUDE_PLUGIN_DATA`, `CLAUDE_SKILL_DIR`, `COPILOT_PLUGIN_ROOT`, `CURSOR_PLUGIN_ROOT`, `DEVIN_PLUGIN_ROOT`, `PLUGIN_ROOT` or `PLUGIN_DATA` after `$` or `${`, or `${extensionPath}`. |
| `command_client_variable` | Any other `${CLAUDE_...}`, `${COPILOT_...}`, `${CURSOR_...}` or `${DEVIN_...}`. The braceless form, such as `$CLAUDE_NOTIFICATIONS_BIN`, is allowed. |
| `command_argument_placeholder` | `$ARGUMENTS`, `${ARGUMENTS}`, `$` directly followed by a digit (`$1`, also `$5`), or `{{args}}`. Describe the input in prose instead: "the text the user wrote after the command". |
| `command_shell_injection` | `` !` ``, `!{`, or three or more backticks or tildes followed by `!` anywhere in the text, such as ```` ```! ```` at a line start, after `- ` or `> `, or inline. |

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

**Ordinary commands** (`disable-model-invocation: false`) reach clients whose
adapters support both Skills and portable commands. Check the per-client plan;
being a supported installation target does not imply support for every component.
GitHub Copilot CLI and Visual Studio Code withhold commands as described below.
Pi supports native extensions only and receives neither portable Skills nor
command Skills; see [the Pi extension contract](PI_NATIVE_EXTENSION_CONTRACT.md).
The user runs a delivered skill the way the client runs skills by hand. Like any
ordinary skill, the model may also choose it on its own.

**GitHub Copilot CLI and Visual Studio Code receive no commands.** Visual
Studio Code is frozen by an owner decision of 2026-10-16, and it installs
through the GitHub Copilot CLI backend, sharing one package with Copilot CLI.
One package cannot hold commands for one of the two and not for the other, so
the client registry withholds commands from both:

- every command is withheld with the plan warning
  `command_not_supported_by_client`, and its compatibility row is unsupported
  with the same limitation;
- the package's skills, MCP servers and other components still install as
  before;
- a package with nothing else for these clients is rejected with
  `no_supported_components`;
- authoring reports show the warning `target_command_withheld`, and an error
  only when the client gets nothing else.

When the freeze is lifted, both clients receive commands again; run
`agentplugins update <name>` after upgrading the installer.

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

Every explicit-only skill also carries `disable-model-invocation: true` in its
header, whatever the client's own mechanism is. It is the one marker the
installer checks on the written bytes for every client, so no decorator can
deliver an explicit-only command without it. For Codex the field is only that
marker: Codex's skill loader ignores header fields it does not know, and the
policy it acts on is `agents/openai.yaml`. Codex's optional skill-creator
validator does list allowed fields and would flag it, but the loader does not
use that validator.

The table records what each client documents or ships, as reviewed on
2026-10-16. It is not a statement of what `agentplugins` delivers.

| Client | Manual skill invocation | Explicit-only mechanism | Explicit-only commands delivered |
| --- | --- | --- | --- |
| Claude Code | `/skill`, plugin skills `/plugin:skill` | documented: `disable-model-invocation: true` | no, probe pending |
| OpenAI Codex | `$skill`, plugin skills `plugin:skill` | documented: `agents/openai.yaml`, `policy.allow_implicit_invocation: false` | no, probe pending |
| ChatGPT | `@` | not confirmed | no |
| Cursor | `/`, `@` | documented; plugin skills that use it are hidden (reported bug) | no |
| GitHub Copilot CLI | `/SKILL-NAME` | documented: `disable-model-invocation` | no; no commands reach the GitHub Copilot family while VS Code is frozen |
| Visual Studio Code | `/` | documented: `disable-model-invocation` | no; frozen by owner decision (2026-10-16), no commands at all |
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
  the ordinary update. The plan cannot tell in advance whether the rules
  changed anything, so neither `update <name>` nor `update --all` reports such
  a package as current: every target that receives commands is planned and
  asks for confirmation. The update then compares the installed command skills
  with the current rendering and refreshes only when they differ; otherwise it
  leaves files and state as they are. Other projection inputs that changed
  since installation, such as the `opencode.jsonc` OpenCode creates on its
  first run, are not a reason to refresh, exactly as for a package without
  commands. An embedded installer's update preview for such a package reports
  `RequiresConfirmation` and never `NoChange`. Clients that receive no portable
  commands keep the ordinary update behavior. `update
  --all` still skips a Directory installation that the signed Discovery record
  reports as current; run `agentplugins update <name>` for it.
- **Operations that keep the installed tree** do not apply new rules. On
  clients that rebuild their native projection from the installed tree (Kiro,
  Gemini CLI, OpenCode, Cline, Windsurf / Devin), a repeated `add`, a resumed
  activation and an unchanged target of a group `add` stop with
  `usecase.ErrProjectionRefreshRequired` when the installed command skills
  differ from what the current rules render. A preview, and therefore the CLI
  and the embedded installer, stops before any change; a confirmed
  `Service.Add` called without a preview may first prepare the installed MCP
  runtime. Other clients keep the installed skills and report no change. Run
  `agentplugins update <name>`.
- **Repair** is for restoring a damaged installation, not for applying new
  rules; use `agentplugins update <name>` for that. `agentplugins repair` and a
  single-target repair through the embedded installer stop with
  `usecase.ErrProjectionRefreshRequired` when the installed command skills of
  an intact package differ from the current rendering; other projection
  changes do not stop them. If the package is missing or damaged and it can no
  longer be rebuilt as recorded, remove and add it again.
- **The embedded installer** reports `usecase.ErrProjectionRefreshRequired`
  wrapped in `installer.ErrUpdateRequired`, with a conflict outcome, the reason
  `update_required` and an update next action; `errors.Is` finds either error.
  When a group refresh stops part way, every target names its client and gets
  a phase: completed, failed or not attempted.
- **Saved dependency environments** (`agentplugins deps`) do not use the
  refresh. New rendering rules reach those installations at the next pin
  change. Until then a sync leaves a settled binding as it is, but on the
  clients listed above an update or repair of the binding at an unchanged pin,
  or a sync that still has to finish its activation, stops with
  `usecase.ErrProjectionPinChangeRequired`, which `errors.Is` does not match to
  `ErrProjectionRefreshRequired`. The embedded installer reports it as a
  conflict with the reason `pin_change_required` and no next action, because
  an update cannot help.

## Security review

The security review of the `agentplugins` CLI scans the package as clients
receive it: the package snapshot plus every command rendered as its skill. A signed Directory
verdict does not cover commands yet, so a package with commands is always
scanned locally or served from a cache entry keyed to its rendered commands.

## Known limitations

- OpenCode expands `@path` file references in skill text. The body rules do
  not reject `@path`, so such a reference in a command reads a file in
  OpenCode and stays plain text in other clients. Write file names without a
  leading `@` when the command must mean the same everywhere.

## Not supported

- argument placeholders, shell injection and per-client fields (see above);
- aliases and nested command directories;
- authoring commands for one client in `plugin/targets/<client>/commands/`;
- per-client skill names: every client gets `<slug>-<name>`.
