---
title: "CI Integration"
description: "Turn the public authored flow into a stable CI gate for plugin-kit-ai projects."
canonicalId: "page:guide:ci-integration"
section: "guide"
locale: "en"
generated: false
translationRequired: true
---

# CI Integration

The safest CI story is not complicated. It is just strict about the public contract.

<MermaidDiagram
  :chart="`
flowchart LR
  Doctor[doctor] --> Bootstrap[bootstrap when needed]
  Bootstrap --> Generate[generate]
  Generate --> Validate[validate --strict]
  Validate --> Smoke[smoke or bundle checks]
`"
/>

## The Minimal CI Gate

For most authored projects, this is the baseline:

```bash
plugin-kit-ai doctor .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform <target> --strict
```

If your lane has stable smoke tests or bundle checks, add them after the validation gate instead of replacing it.

## Why This Works

- `doctor` catches missing runtime prerequisites early
- `generate` proves that generated outputs can be reproduced from authored state
- `validate --strict` proves that the repo is internally consistent for the chosen target
- for a multi-target repo, the same logic should hold for each target in the support scope

## Runtime-Specific Notes

### Go

Go is the cleanest CI path because the execution machine does not need Python or Node just to satisfy the runtime lane.

For launcher-based Go repos, build the checked-in launcher before `doctor`:

```bash
go build -o bin/my-plugin ./cmd/my-plugin
plugin-kit-ai doctor .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

### Node/TypeScript

Add bootstrap explicitly:

```bash
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

### Python

Use the same pattern as Node and make the Python version explicit in CI.

## Common CI Mistakes

- running `validate --strict` without `generate`
- treating generated artifacts as manually maintained files
- forgetting runtime prerequisites for Node or Python lanes
- promising compatibility for a target that is outside the stable support boundary

## Recommended Rule

If CI cannot reproduce the authored outputs and pass `validate --strict`, the repo is not ready for stable handoff. For a multi-target repo, that means an explicit green run for each target inside the support scope.

Pair this page with [Production Readiness](/en/guide/production-readiness), [Support Boundary](/en/reference/support-boundary), and [Troubleshooting](/en/reference/troubleshooting).
