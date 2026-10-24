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


These are **existing installer commands**, sourced from the frozen installer
README and contract. They are separate from the Build examples.
Choose a package because it solves your task and its source is acceptable to you.

## Discover the package

```bash
agentplugins search docs
agentplugins info context7
```

Search finds candidates; info lets you inspect a candidate before installation.
A registry short name such as `context7` comes from the signed Universal Agent
Plugins Registry. Discovery is not execution of the package or proof that its
remote service is available.

Read what the package exposes and which agents can consume its components.
Do not substitute a similarly named package when the source is ambiguous.

## Preview one explicit selection

```bash
agentplugins add context7 --target codex,cursor --dry-run
```

This asks the installer for a plan for Codex and Cursor. `--dry-run` belongs to
installation policy; it does not add an authoring mode or test an MCP server.
Review the selected package, destinations, compatibility findings, and remaining
activation work. A plan is not an installed-state receipt.

## Install after reviewing the plan

For an intentional installation, the existing installer command is:

```bash
agentplugins add context7 --target codex,cursor
```

For interactive use, omit `--target` and choose among detected agents. One
detected agent is selected automatically; several produce a multi-select prompt.
Use explicit targets when you need a reproducible selection.

The installer preflights selected agents before changing managed files. If a
multi-agent operation cannot finish, follow its rollback or repair instruction.
Do not manually delete unrelated client configuration to clear an error.

## Finish activation

Read the per-client result. Complete any sign-in or manual activation it names,
then start the session requested by that client. Installation and activation
are distinct: writing native configuration does not prove OAuth succeeded or
that a client loaded every component.

If the plugin is missing in the client, use [managed-state diagnostics](./manage)
and retain the activation instructions. Re-running authoring validation cannot
finish a client login.

## Use a local package

For a directory supplied by an author, select the actual package root:

```bash
agentplugins validate ./my-plugin
agentplugins add ./my-plugin --target codex --dry-run
```

The first command is installer validation. The second previews installation;
it deliberately stops short of applying it. See the author's
[handoff checklist](/en/build/handoff) for the static evidence to request.
`plugin.json` is the installation authority and defines the package identity.

## Use an exact remote source

The installer accepts a full commit SHA, with an explicit package path when
needed. This is a **syntax example**: replace the owner, repository, full SHA,
and path with a real source you reviewed before invoking it.

```text
agentplugins add owner/repository@0123456789abcdef0123456789abcdef01234567//path/to/plugin --target codex --dry-run
```

Branches, tags, and abbreviated SHAs are rejected. Without a package path, the
installer uses a valid root package or the only valid nested candidate. Multiple
candidates require an explicit path; more than 16 possible packages require it
before candidate fetching. An exact source is immutable: repair replays recorded
source, while moving to another exact source is a separate switch decision.

## Install Agent Skills from GitHub

Agent Skills use the same CLI through the `skills` command group. Its `add`
subcommand selects Skills from a public GitHub repository, a folder inside it,
or a `SKILL.md` URL:

```bash
agentplugins skills add owner/repository --ui
```

`--ui` opens the bundled browser picker and needs an interactive terminal. The
terminal picker is `--no-ui`. Explicit selections skip the picker:

```bash
agentplugins skills add owner/repository --skill review --skill pdf --target codex,claude --no-ui --yes
```

- `--skill` takes an exact Skill name and can repeat. If the name is ambiguous,
  add `--path`.
- `--path` selects one exact repository-relative Skill folder. Without it, the
  repository root is used.
- `--ref` names an exact Git ref when a URL boundary is ambiguous.
- `--dry-run` shows the exact plan without changes.
- JSON and non-interactive use require explicit `--skill` or `--path`, `--target`,
  and `--yes`, except with `--dry-run`.
- `--ui` cannot be combined with `--skill`, `--target`, or `--dry-run`.
- The installation scope is `user` only in this release.

Automated security findings can block an install. `--security-details` shows
every finding in human output. `--accept-security-risk` continues past blocking
findings and should be used only after review.

Installed Skills are saved like other packages. They show up in
`agentplugins list`, and you maintain them with `agentplugins update`,
`agentplugins repair`, and `agentplugins remove`. See
[maintaining installed plugins](./manage).

Continue with [maintaining installed plugins](./manage).
