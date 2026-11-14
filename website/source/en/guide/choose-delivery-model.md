---
title: "Choose Delivery Model"
description: "How to choose between vendored helpers and the shared runtime package for Python and Node plugins."
canonicalId: "page:guide:choose-delivery-model"
section: "guide"
locale: "en"
generated: false
translationRequired: true
---

# Choose Delivery Model

Python and Node plugins have two supported ways to ship helper logic. They solve different practical problems.

<MermaidDiagram
  :chart="`
flowchart TD
  Start[Python or Node plugin] --> Shared{Need one reusable dependency across repos}
  Shared -->|Yes| Package[shared runtime package]
  Shared -->|No| Smooth{Need the smoothest self contained start}
  Smooth -->|Yes| Vendored[vendored helper]
  Smooth -->|No| Package
`"
/>

## Fast Practical Rule

If you just want the simplest working Python or Node repo today, use the default scaffold first.

If you already know that several repos should share one helper dependency, start with `--runtime-package`.

## The Two Modes

- `vendored helper`: the default scaffold writes helper files into the repo itself
- `shared runtime package`: `--runtime-package` imports `plugin-kit-ai-runtime` as a dependency instead of writing the helper into `plugin/`

## The Same Project In Both Modes

Default local-helper path:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
```

Shared-package path:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python --runtime-package
```

## Choose Vendored Helper When

- you want the smoothest first-run path
- you want the repo to stay self-contained
- you want the helper implementation visible in the repo
- your team is not yet standardizing on one shared PyPI or npm helper version

This is the default because it is the easiest starting point for Python and Node projects.

## Choose Shared Runtime Package When

- you want one reusable helper dependency across multiple plugin repos
- you prefer upgrading helper behavior through normal package version bumps
- your team is comfortable pinning versions in `requirements.txt` or `package.json`
- you already know the repo should follow the shared dependency path from day one

## What People Usually Mean In Practice

- choose vendored helper when the main goal is "get one repo working quickly"
- choose the shared runtime package when the main goal is "reuse the same helper package across repos"
- do not choose the shared package just because it sounds more production-like; it does not remove the Python or Node runtime requirement from the execution machine

## What Does Not Change

- Go is still the recommended default when you want the strongest production path
- Python still requires Python `3.10+` on the execution machine
- Node still requires Node.js `20+` on the execution machine
- `validate --strict` remains the main readiness check
- CLI install packages still do not become runtime APIs

## Recommended Team Policy

- choose Go when you want the strongest long-term supported path
- choose vendored helpers when you want the smoothest Python or Node start
- choose the shared runtime package when you already know you want a reusable dependency strategy across repos

Pair this page with [Build A Python Runtime Plugin](/en/guide/python-runtime), [Choose A Starter Repo](/en/guide/choose-a-starter), [Starter Templates](/en/guide/starter-templates), and [Production Readiness](/en/guide/production-readiness).
