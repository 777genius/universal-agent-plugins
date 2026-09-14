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

Milestone A is available in `universal-agent-plugins@0.1.65`; GitHub release: `agentplugins-v0.1.65`.

Compatibility is package-specific. Schema validation does not prove runtime, OAuth or activation. Codex does not support declared MCP SSE; stdio and Streamable HTTP keep their existing adapter support.

[Client compatibility (English)](https://github.com/777genius/universal-agent-plugins#supported-clients)

[Tested client versions and platform limitations](/en/reference/client-compatibility)

## Build plugins {#build-plugins}

<a id="recommended-default"></a>
<a id="if-you-only-read-one-thing"></a>

Build a portable Agent Plugins 1.0 package around plugin.json, with optional skills/ and mcp.json. Client support depends on the package and its components.

**Milestone A is available**

Milestone A static authoring is available through `agentplugins author` in `universal-agent-plugins@0.1.65`; GitHub release: `agentplugins-v0.1.65`. npm, Homebrew, native archives and public-channel E2E are verified. Static authoring only; runtime/dev/bootstrap, export and publication remain deferred. Legacy YAML migration is cancelled; preserved source is not a supported product.

```bash
npm install --global universal-agent-plugins@0.1.65
agentplugins author init ./my-plugin --template skill --name my-plugin \
  --description 'Instructions for a repeatable agent task'
agentplugins author validate ./my-plugin
agentplugins author inspect ./my-plugin
agentplugins author test ./my-plugin
```

[Open the complete Build guide](/en/build/)

[Read the Agent Plugins 1.0 specification](https://agent-plugins.org/specification)
