---
title: "Choose A Starter Repo"
description: "A practical matrix for choosing the right official starter by target, runtime, and delivery path."
canonicalId: "page:guide:choose-a-starter"
section: "guide"
locale: "en"
generated: false
translationRequired: true
---

# Choose A Starter Repo

Use this page when you want the fastest path into a repo that can later expand to more supported outputs.

<MermaidDiagram
  :chart="`
flowchart TD
  Start[Need a starter] --> Product{Primary path is Codex or Claude}
  Product --> Codex[Codex starter family]
  Product --> Claude[Claude starter family]
  Codex --> Runtime{Go, Node or Python}
  Claude --> Runtime2{Go, Node or Python}
`"
/>

Before you choose, remember one important rule:

- the starter tells you how to begin
- it is not the final limit of the product
- and it does not stop one repo from later supporting more targets

If that distinction is still fuzzy, read [One Project, Multiple Targets](/en/guide/one-project-multiple-targets) first.

## Pick Fast, Then Expand Later

- choose Go when you want the strongest production path
- choose Node/TypeScript when you want the main supported non-Go path
- choose Python when the repo is intentionally Python-first and stays local to the repo
- choose Claude starters only when Claude hooks are the actual product requirement

Pick the starter for the first correct path, not for an imagined permanent product boundary.

## What Stays True After You Pick

- You still keep one repo.
- You still keep the same core workflow.
- You can add supported targets later as the product grows.
- Support depth depends on the target you add.

## Starter Matrix

| If you want | Best starter | Why |
| --- | --- | --- |
| Strongest Codex production path | `plugin-kit-ai-starter-codex-go` | Go-first production path with the cleanest handoff story |
| Repo-local Codex plugin in Python | `plugin-kit-ai-starter-codex-python` | Stable Python subset with a known-good repo layout |
| Repo-local Codex plugin in Node/TS | `plugin-kit-ai-starter-codex-node-typescript` | Main supported non-Go path |
| Strongest Claude production path | `plugin-kit-ai-starter-claude-go` | Stable Claude subset plus the cleanest production path |
| Repo-local Claude plugin in Python | `plugin-kit-ai-starter-claude-python` | Stable Claude hook subset with Python helpers |
| Repo-local Claude plugin in Node/TS | `plugin-kit-ai-starter-claude-node-typescript` | Stable Claude hook subset for TypeScript-first teams |

## Shared-Package Variants

Ignore this section unless you already know that your team wants `plugin-kit-ai-runtime` as a reusable dependency instead of vendored helper files.

Use the shared-package variants when:

- you want a shared dependency across multiple plugin repos
- you are comfortable pinning and upgrading the runtime package explicitly
- you do not want helper files copied into every repo

Current shared-package starters:

- [`plugin-kit-ai-starter-codex-python-runtime-package`](https://github.com/777genius/plugin-kit-ai-starter-codex-python-runtime-package): Python Codex starter with `plugin-kit-ai-runtime` pinned in `requirements.txt`
- [`plugin-kit-ai-starter-claude-node-typescript-runtime-package`](https://github.com/777genius/plugin-kit-ai-starter-claude-node-typescript-runtime-package): Node/TypeScript Claude starter with `plugin-kit-ai-runtime` pinned in `package.json`

If you are choosing between the normal Python starter and the runtime-package Python starter, read [Build A Python Runtime Plugin](/en/guide/python-runtime) first and then [Choose Delivery Model](/en/guide/choose-delivery-model).

## When To Avoid Over-Optimizing The Choice

Do not spend too long searching for the perfect starter.

If you are unsure:

1. start with the Go starter for the strongest default
2. start with the Node/TypeScript starter for the main supported non-Go path
3. only choose Python or shared-package variants when the team tradeoff is already real

## Good Team Policy

A team-wide starter choice should stay consistent long enough that:

- everyone recognizes the repo layout
- CI uses the same readiness flow
- handoff does not depend on maintainer explanation

But a stable starter choice still does not stop one repo from adding other targets later if the product requires them.

Pair this page with [Starter Templates](/en/guide/starter-templates), [Choose Delivery Model](/en/guide/choose-delivery-model), and [Repository Standard](/en/reference/repository-standard).
