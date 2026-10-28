---
title: "Quickstart"
description: "Install available plugins or prepare a portable Agent Plugins 1.0 package."
canonicalId: "page:guide:quickstart"
section: "guide"
locale: "en"
generated: false
translationRequired: true
---

<a id="quickstart"></a>

# Use plugins / Build plugins

Install a package for the agents you use, or create a portable Agent Plugins 1.0
package with instructions and tool configuration.

## Use plugins {#use-plugins}

<a id="optional-first-proof"></a>

With Node.js 22 or newer:

```bash
npx universal-agent-plugins add context7
```

The CLI asks which compatible agents to use and reports any remaining activation
steps. To inspect and preview first:

```bash
npx universal-agent-plugins info context7
npx universal-agent-plugins add context7 --target codex,cursor --dry-run
```

The [native installation guide](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md)
also provides macOS, Linux and Windows executables. The native manager needs no
Node.js, although individual plugins may require their own runtimes. Check
`npx universal-agent-plugins version` when using npm: a package-manager channel
can still provide an older CLI. The automatic selection below requires 0.1.94;
use the native installer or Homebrew if needed. The explicit project commands
in this guide also work with 0.1.93.

[CLI 0.1.94](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.94)
includes explicit project scope and automatic project selection for `install`.
To preview adding a package to a project:

```bash
npx universal-agent-plugins add context7 --target cursor --scope project \
  --project-root /path/to/project --dry-run
```

Choose an existing project directory. Add and maintenance commands default to
user scope, so select project scope explicitly. In 0.1.94, `install` without a
source argument selects the project only if the exact current folder contains
a regular `uap.toml`; explicit scope wins and parents are not searched. Review
the plan before omitting `--dry-run`. Native discovery, trust, authentication
and successful MCP calls remain separate from writing the package's files.

[Complete installation journey](../use/install) ·
[Client compatibility and test boundaries](../reference/client-compatibility)

### Restore a saved dependency set

Check what your selected user environment records:

```bash
npx universal-agent-plugins deps status
```

For a saved `uap.toml`/`uap.lock` pair, export from an exchange folder:

```bash
npx universal-agent-plugins deps export
```

Copy the folder to another machine, install its target clients, and run there:

```bash
npx universal-agent-plugins deps import
npx universal-agent-plugins install --locked --scope user
```

Only the validated agreement and lock are transferred. Profiles, settings,
credentials, receipts and plugin data stay separate. Installation still needs
the exact sources or verified cached packages. For a project, commit the pair
and restore with explicit `install --locked --scope project`.
See [maintenance and restoration](../use/manage) for recovery and transfer details.

## Build plugins {#build-plugins}

<a id="recommended-default"></a>
<a id="if-you-only-read-one-thing"></a>

A standard package starts with root `plugin.json`, and can include
`skills/<name>/SKILL.md`, `mcp.json`, or both. You can write these files manually
and validate the package with the released installer:

```bash
npx universal-agent-plugins validate ./my-plugin
npx universal-agent-plugins add ./my-plugin --target codex --dry-run
```

These examples assume `./my-plugin` already contains a complete package.
The [Build guide](../build/) walks through a small manual Skill package and the
source-build scaffolding tools.

**Authoring availability:** the current npm and native installer release does
not include `agentplugins author`. Current source implements scaffolding,
static checks, explicit MCP runtime tests, development cycles, dependency
bootstrap and JSON maintenance. Use the [source-build setup](../build/#build-from-source)
for those commands. Older authoring release evidence remains tied to its own
version; it is not a promise about the current installer download.

Static validation does not establish runtime, OAuth or activation. MCP transport
support also varies by client; for example, Codex does not support declared MCP
SSE. Keep the package's requirements and remaining checks visible in its README.

[Agent Plugins 1.0 specification](https://agent-plugins.org/specification)
