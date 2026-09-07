---
title: "Quickstart"
description: "Install available plugins or prepare a portable Agent Plugins 1.0 package."
canonicalId: "page:guide:quickstart"
section: "guide"
locale: "en"
generated: false
translationRequired: true
---

# Use plugins / Build plugins

Install available plugins or prepare a portable Agent Plugins 1.0 package.

## Use plugins {#use-plugins}

Available now: install plugins with Universal Agent Plugins. With Node.js 22+, run:

```bash
npx universal-agent-plugins add context7
```

For native installation on macOS, Linux or Windows, follow the installer guide. The CLI asks which compatible agents to use; individual plugins may need their own runtimes.

[Installer guide](https://github.com/777genius/universal-agent-plugins#quick-start)

Published versions checked on 2026-09-07: universal-agent-plugins 0.1.51 (npm), plugin-kit-ai 1.2.4 (npm/PyPI), stable GitHub release agentplugins-v0.1.51.

Compatibility is package-specific. Schema validation does not prove runtime, OAuth or activation. Codex does not support declared MCP SSE; stdio and Streamable HTTP keep their existing adapter support.

[Client compatibility (English)](https://github.com/777genius/universal-agent-plugins#supported-clients)

## Build plugins {#build-plugins}

Build a portable Agent Plugins 1.0 package around plugin.json, with optional skills/ and mcp.json. Client support depends on the package and its components.

**Preparation — unreleased**

The standard-first authoring CLI is in preparation and is unreleased. Published plugin-kit-ai 1.2.4 on npm and PyPI is the historical v1 tool, not standard-first v2. Installing plugin-kit-ai@latest does not provide the future authoring workflow.

[Read the Agent Plugins 1.0 specification](https://agent-plugins.org/specification)

## Historical v1 maintenance {#historical-v1}

Maintain existing plugin.yaml projects with the v1 instructions below. These templates and generated outputs are historical v1 workflows; they do not create the new standard-first authoring flow.

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai version
plugin-kit-ai init my-plugin
cd my-plugin
go mod tidy
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

```bash
plugin-kit-ai init my-plugin --template online-service
plugin-kit-ai init my-plugin --template local-tool
plugin-kit-ai init my-plugin --template custom-logic
```

### What You Get

- one plugin repo from day one
- authored files under `plugin/`
- generated Codex runtime output from the same repo
- a clean readiness check through `validate --strict`

### Supported Node And Python Paths

If your team already lives in Node/TypeScript or Python, those paths are supported and visible from the start:

- `codex-runtime --runtime node --typescript`
- `codex-runtime --runtime python`
- both are local interpreted runtime paths, so the target machine still needs Node.js `20+` or Python `3.10+`
- Go still stays the default when you want the strongest general production story

### If You Are Intentionally Starting On Node Or Python

Use this alternate flow only when the language choice is already part of the product requirement:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime node --typescript
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

Or start with Python:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

### What To Do Next

- edit the plugin under `plugin/`
- run `plugin-kit-ai generate ./my-plugin` again after changes
- run `plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict` again
- only then add another way to ship it if the product needs that

### Expand Later

| If you want | Add this later |
| --- | --- |
| Claude hooks as the real product | `claude` |
| Official Codex package | `codex-package` |
| Gemini extension package | `gemini` |
| Repo-owned integration setup | `opencode` or `cursor` |

Choose `claude` first only when Claude hooks are already the real product requirement.

### What Expands Later

- the repo stays unified as you add more lanes
- package and extension lanes come from the same authored source
- OpenCode and Cursor fit when the repo should own integration setup
- the exact support boundary stays in the reference docs, not in your first-start flow

### After Quickstart

- Continue with [Choose What You Are Building](/en/guide/choose-what-you-are-building) for historical v1 template selection.
- Continue with [Build Custom Plugin Logic](/en/guide/build-custom-plugin-logic) if you are intentionally taking the advanced runtime path.
- Continue with [Build Your First Plugin](/en/guide/first-plugin) if you specifically want the narrow legacy-compatible Codex runtime tutorial.
- Continue with [What You Can Build](/en/guide/what-you-can-build) for the historical v1 product map.
- Continue with [Choose A Target](/en/guide/choose-a-target) when you are ready to match the repo to how you want to ship it.
- Continue with [One Project, Multiple Targets](/en/guide/one-project-multiple-targets) when you are ready to expand beyond the first path.
