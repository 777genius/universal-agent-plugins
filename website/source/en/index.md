---
title: "Agent Plugins Documentation"
description: "Public documentation for Agent Plugins."
canonicalId: "page:home"
section: "home"
locale: "en"
generated: false
translationRequired: true
---

<div class="docs-hero docs-hero--feature">
  <p class="docs-kicker">PUBLIC DOCUMENTATION</p>
  <h1>Agent Plugins</h1>
  <p class="docs-lead">Install Skills and MCP tools for the agents you use, or build a portable package of your own.</p>
</div>

## Choose your journey

<div class="docs-grid">
  <a class="docs-card" href="./use/">
    <h2>Use plugins</h2>
    <p>Inspect a package, preview its changes, and install it for your account or one project.</p>
  </a>
  <a class="docs-card" href="./build/">
    <h2>Build plugins</h2>
    <p>Write a standard package with plugin.json, Skills and MCP configuration. Explore source-build authoring tools.</p>
  </a>
</div>

## Install your first plugin

With Node.js 22 or newer:

```bash
npx universal-agent-plugins add context7
```

The CLI asks which compatible agents to use. Review the plan and complete the
activation or sign-in steps it reports. Prefer a native executable? Follow the
[native installation guide](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md)
for macOS, Linux or Windows; the native manager itself does not require Node.js.

## What is available

[CLI 0.1.95](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.95)
is the current native installer release. It adds a local management API and
user-scope native Pi extensions. See [local management](./use/manage#local-management-api)
and [Pi compatibility](./reference/client-compatibility#pi-native-extensions).
It retains explicit project scope and locked restoration. Most commands default
to your user environment. An `install` without
a source argument selects the current project when that exact folder contains a
regular `uap.toml`; explicit scope wins. Select project scope on maintenance
commands. Check `agentplugins version`: automatic selection requires 0.1.94.
Package-manager channels may lag; use the native installer for the verified
release, or retain explicit project flags supported since 0.1.93. See
[installation](./use/install) and [maintenance](./use/manage).

The current installer does not include `agentplugins author`. The
[Build guide](./build/) explains manual packages and the authoring commands
implemented in current source. Older authoring release evidence retains its
original version and does not describe the current installer download.

## Understand the result

Package validation, installation, client discovery, MCP tool execution and
service authentication are separate checks. A successful file installation may
still need client trust, a reload or a fresh session. Native tests cover specific
client versions and platforms; they do not certify every plugin or every agent.

- [Quickstart](./guide/quickstart)
- [Package layout](./build/layout)
- [Checks and evidence](./build/checks)
- [Client compatibility and test boundaries](./reference/client-compatibility)
- [Agent Plugins 1.0 specification](https://agent-plugins.org/specification)
