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

> **Prepared, unreleased authoring contract.** This page is a review draft on a
> non-deploying preparation branch. Future authoring examples require the accepted
> release; they do not claim that `plugin-kit-ai@2` is available. Existing installer
> examples are identified separately. Public activation remains pending.

These are **existing installer commands**, sourced from the frozen installer
README and contract. They are separate from the future Build examples.
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
`plugin.json` is the installation authority; legacy `plugin.yaml` cannot
replace it or override its identity.

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

Continue with [maintaining installed plugins](./manage).
