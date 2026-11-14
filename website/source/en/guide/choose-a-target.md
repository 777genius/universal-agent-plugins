---
title: "Choose A Target"
description: "A practical public guide for choosing the target that matches how you want to ship the plugin."
canonicalId: "page:guide:choose-a-target"
section: "guide"
locale: "en"
generated: false
translationRequired: true
---

# Choose A Target

This is an advanced page.
If you are still deciding what kind of repo to create, start with [Choose What You Are Building](/en/guide/choose-what-you-are-building) first.

Use this page when you already know you want `plugin-kit-ai`, but you still need to match the repo to how you want to ship the plugin.

Choosing a target means choosing the main path the product needs today, not locking the repo forever.

<MermaidDiagram
  :chart="`
flowchart TD
  Need[What does the product need right now] --> Exec{Executable behavior}
  Need --> Artifact{Package or extension}
  Need --> Config{Repo managed integration}
  Exec --> Codex[codex-runtime]
  Exec --> Claude[claude]
  Artifact --> CodexPackage[codex-package]
  Artifact --> Gemini[gemini]
  Config --> OpenCode[opencode]
  Config --> Cursor[cursor]
`"
/>

## Short Rule

- choose `codex-runtime` when you want the strongest default runtime path
- choose `claude` when Claude hooks are the real product requirement
- choose `codex-package` when the product is an official Codex package
- choose `gemini` when the product is a Gemini extension package
- choose `opencode` or `cursor` when the repo should own integration/config setup

## Target Directory

| Target | Choose it when | Lane |
| --- | --- | --- |
| `codex-runtime` | You want the default executable plugin path | Recommended runtime path |
| `claude` | You need Claude hooks specifically | Recommended Claude path |
| `codex-package` | You need Codex packaging output | Recommended package path |
| `gemini` | You are shipping a Gemini extension package | Recommended extension path |
| `opencode` | You want repo-owned OpenCode integration setup | Repo-owned integration setup |
| `cursor` | You want repo-owned Cursor integration setup | Repo-owned integration setup |

## Safe Default

If you are unsure, start with `codex-runtime` and the default Go path.

That gives you the cleanest production starting point before you choose a narrower or more specialized path.

When you later move to `codex-package`, the official package lane follows the official `.codex-plugin/plugin.json` bundle layout.

If you intentionally start on supported Node/TypeScript or Python, that changes the language choice, not the need to decide every packaging or integration detail on day one.

## What To Do When You Need More Than One Target

- choose the main path that defines the product today
- keep the repo unified
- add more targets only when a real delivery or integration requirement appears

Read [One Project, Multiple Targets](/en/guide/one-project-multiple-targets) when you want the broader multi-target mental model.
