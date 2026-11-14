---
title: "Starter Templates"
description: "Official starter repositories for common first paths in plugin-kit-ai."
canonicalId: "page:guide:starter-templates"
section: "guide"
locale: "en"
generated: false
translationRequired: true
---

# Starter Templates

If you want a known-good starting point instead of scaffolding from a blank directory, use the official starter repositories.

## Important: Starters Are Entry Points

The starter names are intentionally split by primary path such as Codex or Claude.

That does **not** mean the repo is permanently locked to one agent family.

A starter helps you choose the best first shape for:

- your primary runtime requirement
- your team language choice
- your first supported target

After that, keep the repo unified and grow it as needed.

Read [One Project, Multiple Targets](/en/guide/one-project-multiple-targets) if you want the broader system view.

## Codex Runtime

- [plugin-kit-ai-starter-codex-go](https://github.com/777genius/plugin-kit-ai-starter-codex-go)
- [plugin-kit-ai-starter-codex-python](https://github.com/777genius/plugin-kit-ai-starter-codex-python)
- [plugin-kit-ai-starter-codex-node-typescript](https://github.com/777genius/plugin-kit-ai-starter-codex-node-typescript)
- [plugin-kit-ai-starter-codex-python-runtime-package](https://github.com/777genius/plugin-kit-ai-starter-codex-python-runtime-package)

## Claude

- [plugin-kit-ai-starter-claude-go](https://github.com/777genius/plugin-kit-ai-starter-claude-go)
- [plugin-kit-ai-starter-claude-python](https://github.com/777genius/plugin-kit-ai-starter-claude-python)
- [plugin-kit-ai-starter-claude-node-typescript](https://github.com/777genius/plugin-kit-ai-starter-claude-node-typescript)
- [plugin-kit-ai-starter-claude-node-typescript-runtime-package](https://github.com/777genius/plugin-kit-ai-starter-claude-node-typescript-runtime-package)

## When To Prefer A Starter

Use a starter when:

- you want a known-good repo layout immediately
- you want to compare your project against a minimal supported example
- you want to onboard teammates with less scaffolding guesswork

Use `plugin-kit-ai init` directly when:

- you want a fresh repo from first principles
- you need to choose flags explicitly
- you are building around an existing repository structure

## Safe Mental Model

- choose a starter for the **first** correct path
- do not treat the starter family as the final boundary of the repo
- keep one repo and expand it only when the product really needs more outputs
