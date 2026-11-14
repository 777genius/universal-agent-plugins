---
title: "Examples And Recipes"
description: "A guided map of the public example repos, starter repos, local runtime references, and skill examples in plugin-kit-ai."
canonicalId: "page:guide:examples-and-recipes"
section: "guide"
locale: "en"
generated: false
translationRequired: true
---

# Examples And Recipes

Use this page when you want to see what `plugin-kit-ai` looks like in real repositories instead of only reading abstract guidance.

## 1. Production Plugin Examples

These are the clearest examples of finished public shapes:

- [`codex-basic-prod`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/codex-basic-prod): Go plus `codex-runtime` production repo
- [`claude-basic-prod`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/claude-basic-prod): Go plus `claude` production repo
- [`codex-package-prod`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/codex-package-prod): `codex-package` target
- [`gemini-extension-package`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/gemini-extension-package): `gemini` packaging target
- [`cursor-basic`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/cursor-basic): `cursor` workspace-config target
- [`opencode-basic`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/opencode-basic): `opencode` workspace-config target

Read these when you want:

- a concrete repo layout
- real generated outputs
- a truthful public example of what “healthy” looks like

Important: these examples show distinct public product shapes. They do not imply that a real system must be split into a separate repo for every target.

## 2. Starter Repos

Use starter repos when you want to begin from a known-good baseline instead of from an empty directory.

They are best for:

- first-time setup
- team onboarding
- choosing between Go, Python, Node, Claude, and Codex starting points

The most direct code-first starter links are:

- [`plugin-kit-ai-starter-codex-go`](https://github.com/777genius/plugin-kit-ai-starter-codex-go)
- [`plugin-kit-ai-starter-codex-python`](https://github.com/777genius/plugin-kit-ai-starter-codex-python)
- [`plugin-kit-ai-starter-codex-node-typescript`](https://github.com/777genius/plugin-kit-ai-starter-codex-node-typescript)
- [`plugin-kit-ai-starter-claude-go`](https://github.com/777genius/plugin-kit-ai-starter-claude-go)
- [`plugin-kit-ai-starter-claude-python`](https://github.com/777genius/plugin-kit-ai-starter-claude-python)
- [`plugin-kit-ai-starter-claude-node-typescript`](https://github.com/777genius/plugin-kit-ai-starter-claude-node-typescript)

If you are still choosing, pair this with [Choose A Starter Repo](/en/guide/choose-a-starter).

## 3. Local Runtime References

The `examples/local` area shows Python and Node runtime references for repos that stay local-first.

These are useful when:

- you want to understand the interpreted runtime story more deeply
- you want to compare JavaScript, TypeScript, and Python local-runtime setups
- you need a concrete reference beyond the starter repos

Start with:

- [`codex-node-typescript-local`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/local/codex-node-typescript-local)
- [`codex-python-local`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/local/codex-python-local)

## 4. Skill Examples

The `examples/skills` area shows supporting skill examples and helper integrations.

These are not the main entrypoint for most plugin authors, but they are valuable when:

- you want to wire docs, review, or formatting helpers into the broader workflow
- you want to understand how adjacent skills can fit around plugin repos

## Suggested Reading By Goal

- Want the strongest runtime example: start with the Codex or Claude production example, then read [Build A Team-Ready Plugin](/en/guide/team-ready-plugin).
- Want a code-first example by language and target: start with the linked Go, Python, or Node starter repo above, then read [Build Custom Plugin Logic](/en/guide/build-custom-plugin-logic).
- Want packaging or workspace-config examples: start with Codex package, Gemini, Cursor, or OpenCode examples, then read [Package And Workspace Targets](/en/guide/package-and-workspace-targets).
- Want a clean starting point, not a finished example: go to [Starter Templates](/en/guide/starter-templates).
- Want to choose the target before looking at repos: read [Choose A Target](/en/guide/choose-a-target).
- Want the full one-repo expansion story first: read [What You Can Build](/en/guide/what-you-can-build).

## Final Rule

Examples should clarify the public contract, not replace it.

Use example repos to see shape and healthy outputs. For the one-repo multi-target mental model, read [One Project, Multiple Targets](/en/guide/one-project-multiple-targets).
