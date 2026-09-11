---
title: "Use plugins"
description: "Choose installation or authoring, then follow the appropriate journey."
canonicalId: "page:use:index"
section: "use"
locale: "en"
generated: false
translationRequired: true
---

# Use plugins

> **Prepared, unreleased authoring contract.** This page is a review draft on a
> non-deploying preparation branch. Future authoring examples require the accepted
> release; they do not claim that `plugin-kit-ai@2` is available. Existing installer
> examples are identified separately. Public activation remains pending.

Choose by the result you need:

| Your job | Start here | Tool boundary |
| --- | --- | --- |
| Use an existing plugin in an agent | [Find and install](./install) | `agentplugins` manages installation and activation instructions. |
| Write instructions or connect tools in a new package | [Build plugins](/en/build/) | Future `agentplugins author` and `plugin-kit-ai` share the authoring engine. |
| Maintain a YAML or launcher project | [Historical v1](/en/legacy/v1/) | Use the historical 1.2.4 context; migration is unavailable in v2. |

You do not need to create a project to use someone else's plugin. The installer
accepts standard packages from the registry, local directories, or exact GitHub
commits. Compatibility depends on both the package and the selected agent.

## The installer journey

1. [Find and inspect a package](./install) before selecting agents.
2. Review its source, tools, permissions, and any sign-in requirements.
3. Preview an explicit target selection with the installer's dry run.
4. Install only after reviewing the plan, then follow the printed activation steps.
5. [Check and maintain installed state](./manage) when updating or repairing.

A successful installation can still require a client restart, a fresh session,
manual activation, or OAuth. Read the result for your selected client instead of
assuming that every agent activates the same way.

## Existing installer entrypoints

The current installer documentation uses the native `agentplugins` executable
or the npm package `universal-agent-plugins`. The npm wrapper requires Node.js 22
or newer. These two examples express the same discovery job; choose one:

```bash
agentplugins search docs
```

```bash
npx universal-agent-plugins search docs
```

For installer acquisition, use the repository's
[native installation guide](https://github.com/777genius/universal-agent-plugins/blob/ec0883bf0bfb7f4044321bba9dfd4fd8d6467721/docs/NATIVE_INSTALL.md).
The examples here assume the chosen installer is already available. No future
Build package installation command is implied by this installer guidance.

## Similar command names, different jobs

| Command | Question it answers |
| --- | --- |
| `agentplugins validate ./my-plugin` | Can the installer read this local standard package? |
| `agentplugins doctor` | What needs attention in installer-managed state? |
| Future `agentplugins author validate ./my-plugin` | Does this authored package conform, and is its static readiness complete? |
| Future `agentplugins author doctor ./my-plugin` | What project and toolchain evidence can be inspected without execution? |

Installer doctor does not author a package. Authoring doctor does not repair an
installation. Keep reports from these jobs separate when asking for help.

## What travels between the journeys

The handoff is a directory with root `plugin.json` and the package's components.
It is not a generated YAML project or a promise that a server has been tested.
Authors can prepare [a local planner handoff](/en/build/handoff); installers then
make their own target, security, and activation decisions.

Portable components are Skills and MCP configuration. Client-specific hooks or
extensions are separate compatibility concerns, not a third portable component.
