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

Install available plugins or prepare a portable Agent Plugins 1.0 package.

## Use plugins {#use-plugins}

<a id="optional-first-proof"></a>

Available now: install plugins with Universal Agent Plugins. With Node.js 22+, run:

```bash
npx universal-agent-plugins add context7
```

For native installation on macOS, Linux or Windows, follow the installer guide. The CLI asks which compatible agents to use; individual plugins may need their own runtimes.

[Installer guide](https://github.com/777genius/universal-agent-plugins#quick-start)

Agent Plugins authoring is available in `universal-agent-plugins`; GitHub release: `agentplugins-v*`.

Compatibility is package-specific. Schema validation does not prove runtime, OAuth or activation. Codex does not support declared MCP SSE; stdio and Streamable HTTP keep their existing adapter support.

[Client compatibility (English)](https://github.com/777genius/universal-agent-plugins#supported-clients)

[Tested client versions and platform limitations](/en/reference/client-compatibility)

### Save and restore your plugins

Eligible remote installs automatically save `uap.toml` and `uap.lock` for your
user account, outside the current project. Find their exact paths and check your
saved plugins with:

```bash
npx universal-agent-plugins deps status
```

By default, the files live in `~/.config/agentplugins` on Linux
(`$XDG_CONFIG_HOME/agentplugins` when set), `~/Library/Application Support/agentplugins`
on macOS, or `%APPDATA%\agentplugins` on Windows. `AGENTPLUGINS_HOME` can select
another directory.

The transfer commands below ship in the published CLI. From a folder you want to
carry to another machine, run:

```bash
npx -y universal-agent-plugins@latest deps export
```

Copy that folder to the destination machine, open it there and install your
target agents. Then run:

```bash
npx -y universal-agent-plugins@latest deps import
npx -y universal-agent-plugins@latest install --locked
```

Or combine import and locked installation with `npx -y universal-agent-plugins@latest deps import --install`.
The folder argument is optional and defaults to the current folder. Export reads
your global user environment; import writes the destination's global user
environment. Only the strictly validated, unchanged bytes of `uap.toml` and
`uap.lock` are copied; packages, settings and secrets are not included.

Each actual copy needs a default-no terminal confirmation or `--yes`;
`--dry-run` previews it, and identical files need no copy or confirmation.
File transfer works offline, but installation still needs the exact sources or
cached packages and must pass current security checks. If copying is interrupted,
rerun the original transfer from its intact source. If installation fails after
import saves the files, address the error and retry `npx -y universal-agent-plugins@latest install --locked`
(use `npx -y universal-agent-plugins@latest deps recover` first if pending recovery is reported).

The lockfile restores the recorded package revisions. Agent sign-in and any
manual activation remain separate steps. Local paths, literal `discovery:`
selectors and ChatGPT preparation are not saved as portable dependencies.

[Saved dependency guide](https://github.com/777genius/universal-agent-plugins/blob/main/docs/UAP_DEPENDENCIES.md)

## Build plugins {#build-plugins}

<a id="recommended-default"></a>
<a id="if-you-only-read-one-thing"></a>

Build a portable Agent Plugins 1.0 package around plugin.json, with optional skills/ and mcp.json. Client support depends on the package and its components.

**Agent Plugins authoring is available**

Static authoring is available through `agentplugins author` in `universal-agent-plugins`; GitHub release: `agentplugins-v*`. npm, Homebrew, native archives and public-channel E2E are verified. The release does not run package code, start development servers, bootstrap dependencies, export bundles, or publish packages.

```bash
npm install --global universal-agent-plugins
agentplugins author init ./my-plugin --template skill --name my-plugin \
  --description 'Instructions for a repeatable agent task'
agentplugins author validate ./my-plugin
agentplugins author inspect ./my-plugin
agentplugins author test ./my-plugin
```

[Open the complete Build guide](/en/build/)

[Read the Agent Plugins 1.0 specification](https://agent-plugins.org/specification)
