---
title: "Choose What You Are Building"
description: "Pick the right plugin-kit-ai starting path before you think about target taxonomy."
canonicalId: "page:guide:choose-what-you-are-building"
section: "guide"
locale: "en"
generated: false
translationRequired: true
---

# Choose What You Are Building

Start with the job first. You do not need to understand target IDs, runtime lanes, or local MCP transport details before you create the repo.

## Connect An Online Service

Use this when the plugin should connect to a hosted service like Notion, Stripe, Cloudflare, or Vercel.

```bash
plugin-kit-ai init my-plugin --template online-service
```

This creates:

- one editable source under `plugin/`
- shared hosted-service wiring under `plugin/mcp/servers.yaml`
- generated app-specific output files for the supported package and workspace targets
- no runtime code or launcher contract by default

## Connect A Local Tool

Use this when the plugin should call into a repo-owned CLI, container, or local executable tool like Docker Hub, Chrome DevTools, or HubSpot Developer.

```bash
plugin-kit-ai init my-plugin --template local-tool
```

This creates:

- one editable source under `plugin/`
- local command, container, or tool wiring under `plugin/mcp/servers.yaml`
- generated app-specific output files for the supported package and workspace targets
- no runtime code or launcher contract by default

## Build Custom Plugin Logic - Advanced

Use this when the plugin's value lives in your code, hooks, runtime behavior, or orchestration logic.

```bash
plugin-kit-ai init my-plugin --template custom-logic
```

This path gives you more control and more responsibility than the first two starters:

- you edit runtime-facing files under `plugin/`
- you keep one repo even as generated target outputs appear at the root
- you own the runtime entrypoint, test flow, and behavior that define the plugin

Open [Build Custom Plugin Logic](/en/guide/build-custom-plugin-logic) when you want the dedicated advanced guide for this path.

## What To Do Next

After any of those starts:

```bash
cd my-plugin
plugin-kit-ai inspect . --authoring
plugin-kit-ai generate .
plugin-kit-ai generate --check .
```

Then validate the supported output you actually plan to ship first.

## When To Open The Advanced Pages

- Open [Quickstart](/en/guide/quickstart) when you want the shortest first-run flow.
- Open [Build Custom Plugin Logic](/en/guide/build-custom-plugin-logic) when you are intentionally choosing the advanced runtime path.
- Open [Choose A Target](/en/guide/choose-a-target) when you need target-specific shipping decisions.
- Open [What You Can Build](/en/guide/what-you-can-build) when you want the full product map.
