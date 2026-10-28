---
title: "Find and install a plugin"
description: "Inspect an existing package, preview target changes, and understand activation."
canonicalId: "page:use:install"
section: "use"
locale: "en"
generated: false
translationRequired: true
---

# Find and install a plugin

Choose a package whose source and purpose you understand. The commands below
use the released installer; see [CLI installation](./) if `agentplugins` is not
available yet. Use `--dry-run` to review a plan before applying changes.

## Discover the package

```bash
agentplugins search docs
agentplugins info context7
```

A short name such as `context7` comes from the signed Universal Agent Plugins
Registry. Search and package information help you choose a candidate; they do
not execute it or prove that its service is available. Review the exposed tools,
Skills, runtime requirements and account access before installation.

## Preview one explicit selection

```bash
agentplugins add context7 --target codex,cursor --dry-run
```

Review the source, selected targets, destinations, security findings and
remaining activation work. This command prints a plan without applying the
installation. It does not test an MCP server.

## Install after reviewing the plan

```bash
agentplugins add context7 --target codex,cursor
```

For interactive selection, omit `--target`. The CLI can select one detected
agent automatically or offer a picker when several are available. Use explicit
targets for a reproducible selection and in automation.

The installer preflights the selected clients and protects files it does not
own. If an operation reports a failed rollback or pending recovery, keep that
report and follow its recovery guidance before retrying.

## Finish activation

Complete the client-specific steps in the result: a reload, fresh session,
workspace trust, sign-in or explicit activation may remain. Prepared files and
successful native registration do not establish OAuth or successful tool calls.
Use [managed-state diagnostics](./manage) if installation is missing or drifted;
check client-side authentication when the package is present but unavailable.

## Use a local package

Select the directory containing root `plugin.json`:

```bash
agentplugins validate ./my-plugin
agentplugins add ./my-plugin --target codex --dry-run
```

Installer validation and planning still apply security policy. They can fetch
security metadata or a pinned scanner even for a local package; `--dry-run`
avoids managed installation changes, and does not promise an offline operation.

After reviewing the plan, omit `--dry-run` to install. A local directory stays a
local installation; it is not a portable dependency that another machine can
fetch. Ask the author for the [package handoff evidence](../build/handoff).

## Select an exact GitHub source

The established immutable GitHub syntax requires a full 40-character commit
SHA. Replace this example with a real repository, reviewed revision and package
path:

```text
agentplugins add owner/repository@0123456789abcdef0123456789abcdef01234567//path/to/plugin --target codex --dry-run
```

This `owner/repository@...` form rejects branches, tags and abbreviated SHAs.
Without a path, the installer selects a valid root package or the only valid
nested candidate. Multiple candidates require an explicit path; more than 16
possible packages require it before candidate fetching. Repair replays the
recorded source. Moving an immutable installation to another source is a
separate source-selection operation.

CLI 0.1.93 also accepts direct HTTPS Git, SSH, supported provider browser URLs,
and complete HTTPS/local archives. For example, replace these placeholders with
reviewed package sources:

```text
agentplugins add 'https://git.example.com/team/tools.git#refs/heads/main//plugins/demo' --target codex --dry-run
agentplugins add 'ssh://git@git.example.com/team/tools.git#main//plugins/demo' --target codex --dry-run
agentplugins add 'https://downloads.example.com/plugin.tar.gz' --target codex --dry-run
```

Git uses existing native credentials and SSH host trust. HTTPS archives are
credential-free and must contain a complete standard package. The lock pins
exact Git revisions or original archive bytes, plus package digests. A moving
source can advance through an explicit update; locked restore retains the pin.
These complete-package sources differ from standalone Skill acquisition.
Read [direct complete package sources](https://github.com/777genius/universal-agent-plugins/blob/main/docs/DIRECT_PACKAGE_SOURCES.md)
for provider syntax, package selection, checksums and extraction limits.

## Install Agent Skills

Use the `skills` command group for standalone Skills from a public GitHub
repository, a folder or `SKILL.md` URL, or an HTTPS publisher with a complete
`.well-known` Skills index:

```bash
agentplugins skills add owner/repository --ui
```

Replace `owner/repository` with a real source. `--ui` opens the bundled browser
picker and requires an interactive terminal. Use `--no-ui` for terminal selection.
For an explicit selection:

```bash
agentplugins skills add owner/repository --skill review --skill pdf \
  --target codex,claude --no-ui --yes
```

- Repeat `--skill` for exact names. Use `--path` for an exact repository-relative
  Skill folder when the name is ambiguous.
- `--ref` selects an exact Git ref when a URL boundary is ambiguous.
- `--dry-run` previews the selected installation.
- JSON and non-interactive use require explicit Skill selection, targets and
  `--yes`, except when previewing with `--dry-run`.
- `--ui` cannot be combined with `--skill`, `--target` or `--dry-run`.
- HTTPS publishers require an exact `--skill` name and an index listing every
  file in that Skill.

Automated security findings can block installation. `--security-details` shows
the findings in human output; review them before deciding whether an explicit
`--accept-security-risk` is appropriate. Installed Skills appear in
`agentplugins list` and use the ordinary maintenance commands.

## Install for one project

Explicit project scope is available in
[CLI 0.1.93 and later](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.93).
Choose an existing directory and select scope on each command:

```bash
agentplugins add context7 --target cursor --scope project \
  --project-root /path/to/project --dry-run
agentplugins skills add owner/repository --ui --scope project \
  --project-root /path/to/project
```

The first command previews; omit `--dry-run` when ready to apply it. The second
opens a scoped picker and can apply the user's selection. Without
`--project-root`, explicit project scope uses the invocation directory. It does
not search parent folders. The released CLI otherwise defaults to user scope.

The target must support the requested project components. ChatGPT project
installation is explicitly unsupported. VS Code prepares `.github/skills` and
owned `servers` members in `.vscode/mcp.json` for one local folder. Its result
remains Prepared: review workspace/server trust and inspect the client settings.
That metadata observation does not prove MCP startup, tool calls or model use.

Read the [project installation guide](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md)
and [per-client project boundaries](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md)
for paths and activation limits. Continue with [maintenance and restoration](./manage).
