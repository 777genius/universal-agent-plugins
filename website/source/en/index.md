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
  <p class="docs-lead">
    Build one portable plugin package and validate it for multiple AI agents with <code>agentplugins author</code>.
  </p>
</div>


## Choose your journey

<div class="docs-grid">
  <a class="docs-card" href="./use/">
    <h2>Use plugins</h2>
    <p>Find a reviewed package, preview the installation plan, and install it for selected AI agents.</p>
  </a>
  <a class="docs-card" href="./build/">
    <h2>Build plugins</h2>
    <p>Create a standard package with plugin.json, Skills, MCP configuration, or a supported combination.</p>
  </a>
</div>

## Build with Agent Plugins

Install the released CLI, then use the `agentplugins author` command group:

```bash
npm install --global universal-agent-plugins@0.1.65
agentplugins author init ./my-plugin --template skill --name my-plugin \
  --description 'Instructions for a repeatable agent task'
agentplugins author validate ./my-plugin
agentplugins author inspect ./my-plugin
agentplugins author test ./my-plugin
```

[Open the complete Build guide](/en/build/)

## Current scope

Milestone A creates and statically checks portable Agent Plugins 1.0 packages.
Runtime execution, dev loops, dependency bootstrap, export, and publication are
planned separately. YAML migration is cancelled. Static validation does not prove client
activation, service authentication, or runtime behavior.

The retired YAML implementation remains in source control as historical
reference. It is not a supported product or migration path.

## Reference

- [Quickstart](/en/guide/quickstart)
- [Package layout](/en/build/layout)
- [Static checks and evidence](/en/build/checks)
- [Current support boundary](/en/build/)
- [Agent Plugins 1.0 specification](https://agent-plugins.org/specification)
