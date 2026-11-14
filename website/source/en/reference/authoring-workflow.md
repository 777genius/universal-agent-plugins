---
title: "Authoring Workflow"
description: "The main workflow from init to generate, validate, test, and handoff."
canonicalId: "page:reference:authoring-workflow"
section: "reference"
locale: "en"
generated: false
translationRequired: true
---

# Authoring Workflow

The recommended workflow is intentionally simple:

```text
init -> generate -> validate --strict -> test -> handoff
```

<MermaidDiagram
  :chart="`
flowchart LR
  Init[init] --> Generate[generate]
  Generate --> Validate[validate --strict]
  Validate --> Test[test or smoke checks]
  Test --> Handoff[handoff]
  Bootstrap[doctor or bootstrap when needed] -. supports .-> Generate
  Bootstrap -. supports .-> Validate
`"
/>

## What Each Step Means

| Step | Purpose |
| --- | --- |
| `init` | Create a package-standard project layout |
| `generate` | Generate target artifacts from the project source |
| `validate --strict` | Run the main readiness check |
| `test` | Run stable smoke tests where applicable |
| `export` / bundle flow | Produce handoff artifacts for supported Python and Node cases |

## Rules That Keep The Repo Healthy

- the project source lives in the package-standard project layout
- generated target files are outputs, not the long-term source of truth
- strict validation is a required check, not an optional extra

This workflow matters equally for single-target and multi-target repos.

The only difference is that in a multi-target project, the `generate` and `validate` loop is repeated for each target the repo actually promises to support.

## When The Workflow Changes

The workflow can widen for special cases:

- `doctor` and `bootstrap` matter for Python and Node runtime paths
- `import` and `normalize` matter when consolidating hand-managed target files into the managed project model
- bundle commands matter for portable Python and Node handoff flows

Start with [Quickstart](/en/guide/quickstart) when you need the shortest path.
