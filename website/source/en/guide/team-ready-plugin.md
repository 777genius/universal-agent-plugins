---
title: "Build A Team-Ready Plugin"
description: "A flagship public tutorial for taking a plugin from scaffold to CI-ready, handoff-ready, and team-readable shape."
canonicalId: "page:guide:team-ready-plugin"
section: "guide"
locale: "en"
generated: false
translationRequired: true
---

# Build A Team-Ready Plugin

This tutorial picks up where the first successful plugin stops. The goal is not just “it works on my machine,” but a repo another teammate can clone, validate, and ship without hidden knowledge.

<MermaidDiagram
  :chart="`
flowchart LR
  Scaffold[Scaffolded repo] --> Explicit[Document path and target scope]
  Explicit --> Honest[Keep generated files honest]
  Honest --> CI[Add repeatable CI gate]
  CI --> Handoff[Visible handoff for teammates]
  Handoff --> TeamReady[Team ready repo]
`"
/>

## Outcome

By the end, you should have:

- a package-standard authored repo
- generated files reproduced from the project source
- a strict validation check that passes cleanly
- a clear primary target or targets in scope documented for teammates
- a clear runtime choice or runtime policy by target
- a CI-friendly path that can be repeated on another machine

## 1. Start From The Narrowest Stable Path

Start from the narrowest path that actually matches the job:

```bash
plugin-kit-ai init my-plugin --template custom-logic
cd my-plugin
plugin-kit-ai inspect . --authoring
go mod tidy
plugin-kit-ai validate . --platform codex-runtime --strict
```

This gives you the cleanest base for later handoff.

## 2. Make The Choice Explicit

A team-ready repo should say, at minimum:

- which target is primary and which additional targets are genuinely supported
- which runtime it uses and whether that changes by target
- what the main validation command is, or what validation commands are required for a multi-target repo
- whether it depends on a Go SDK path or a shared runtime package

If that information is only in one maintainer's head, the repo is not ready.

## 3. Keep The Repository Honest

Before you expand the project, enforce three rules:

- the project source lives in the package-standard layout
- generated target files are outputs
- `generate` and `validate --strict` remain part of the normal workflow

Do not patch generated files by hand and then hope the team never reruns generation.

## 4. Add A Repeatable CI Gate

For the default Go launcher path, the minimum gate should look like this:

```bash
go build -o bin/my-plugin ./cmd/my-plugin
plugin-kit-ai doctor .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

If the chosen path is Node or Python, include `bootstrap` and pin the runtime version in CI:

```bash
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

If the repo supports multiple targets, the CI gate should check each supported target explicitly rather than assuming indirect coverage.

## 5. Check Whether You Actually Need A Different Path

Only move away from the starting path when the tradeoff is real:

- use `claude` when Claude hooks are the product requirement
- use `node --typescript` when the team is TypeScript-first and the local runtime tradeoff is acceptable
- use `python` when the project is intentionally local to the repo and Python-first

Changing lanes should solve a product or team problem, not just mirror language preference. If the product is truly multi-target, say that directly: the repo has a primary path and additional targets inside the supported scope.

## 6. Make Handoff Visible

A new teammate should be able to answer these questions from the repo and docs:

- how do I install prerequisites?
- what command proves the repo is healthy?
- what target am I validating for?
- which files are authored state and which are generated?

If the answer to any of those is “ask the original author,” the repo is still not ready.

## 7. Link The Repo Back To The Public Contract

A team-ready plugin repo should point people to:

- [Production Readiness](/en/guide/production-readiness)
- [CI Integration](/en/guide/ci-integration)
- [Repository Standard](/en/reference/repository-standard)
- the current public release note, now [v1.1.2](/en/releases/v1-1-2)

## Final Rule

The repo is ready when another teammate can clone it, understand the path and target scope, reproduce the generated outputs, and pass the strict validation gate without improvisation.

Pair this tutorial with [Build Your First Plugin](/en/guide/first-plugin), [Authoring Architecture](/en/concepts/authoring-architecture), and [Support Boundary](/en/reference/support-boundary).
