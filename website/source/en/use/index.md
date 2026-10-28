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

Universal Agent Plugins installs standard packages for the AI agents you select.
A package can contain Skills, MCP server configuration, or both. You can install
for your account or keep a project's dependencies separate.

## Get the CLI

With Node.js 22 or newer, run the npm wrapper directly:

```bash
npx universal-agent-plugins search docs
```

For repeated use, install it globally:

```bash
npm install -g universal-agent-plugins
agentplugins version
```

Homebrew is another option:

```bash
brew install 777genius/agentplugins/agentplugins
```

The [native installation guide](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md)
provides macOS, Linux and Windows instructions. Native installation needs no
Node.js for the manager, although individual plugins may need their own runtime.
Examples in this journey use `agentplugins`; `npx universal-agent-plugins`
can run the same installer commands.

## Follow the installation journey

1. [Find and inspect a package](./install). Check its source and prerequisites.
2. Preview your intended targets and scope with `--dry-run`.
3. Apply the reviewed installation and follow its activation instructions.
4. [Maintain the installed state](./manage) with list, update, repair and removal.
5. Restore a shared dependency set from `uap.toml` and `uap.lock` when needed.

[CLI 0.1.93](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.93)
includes explicit project scope. The released default is user scope, even when
your current folder contains `uap.toml`. Select `--scope project` on every
project operation. Project support and client activation differ by target.

## Read the result at the right level

Installation can still leave a client reload, trust prompt, manual activation or
OAuth step. A Prepared result means more client work remains. Read each target's
result rather than treating the whole operation as proof of runtime success.
See [client compatibility](../reference/client-compatibility) for the evidence
behind versioned native observations.

## If you are writing a package

Start with [Build plugins](../build/). The shared handoff is a directory with
root `plugin.json` and its components. It does not require the installer user to
run source-build authoring tools.

| Command | Purpose |
| --- | --- |
| `agentplugins validate ./my-plugin` | Read and validate a local package for installation. |
| `agentplugins doctor` | Diagnose installer-managed state. |
| `agentplugins author validate ./my-plugin` | Check authoring conformance and readiness in an authoring-enabled source build. |
| `agentplugins author doctor ./my-plugin` | Inspect project and toolchain evidence in that source build. |

The current npm and native installer release does not include `agentplugins author`.
