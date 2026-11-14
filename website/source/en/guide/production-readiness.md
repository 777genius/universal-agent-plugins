---
title: "Production Readiness"
description: "A public checklist for deciding whether a plugin-kit-ai project is ready for CI, handoff, and broad sharing."
canonicalId: "page:guide:production-readiness"
section: "guide"
locale: "en"
generated: false
translationRequired: true
---

# Production Readiness

Use this checklist before you call a project production-ready, handoff-ready, or ready to show broadly.

<MermaidDiagram
  :chart="`
flowchart LR
  Path[Lane chosen on purpose] --> Source[One authored repo]
  Source --> Checks[generate and validate gates]
  Checks --> Boundary[Support boundary confirmed]
  Boundary --> Handoff[Docs and handoff are explicit]
  Handoff --> Ready[Production ready]
`"
/>

## 1. Pick The Right Path On Purpose

- default to Go when you want the strongest runtime lane
- choose Node/TypeScript or Python when the non-Go local-runtime tradeoff is real
- choose package, extension, or integration lanes only when those are the real outputs you need

## 2. Keep One Repo Honest

- keep the project source in the package-standard layout
- treat generated target files as outputs, not as the main place you edit
- do not patch generated files by hand and expect `generate` to preserve those edits

## 3. Run The Contract Gates

At minimum, the repo should survive this flow cleanly:

```bash
plugin-kit-ai doctor .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform <target> --strict
```

For Go launcher lanes, build `bin/<name>` before `doctor` so the launcher entrypoint exists on disk:

```bash
go build -o bin/my-plugin ./cmd/my-plugin
plugin-kit-ai doctor .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

For Python and Node runtime lanes, `doctor` and `bootstrap` are part of readiness.

## 4. Verify The Exact Support Boundary

- confirm that the primary lane, and each additional lane in scope, is inside the public support boundary
- use the reference pages when you need exact `public-stable`, `public-beta`, or `public-experimental` terms
- check the generated target support matrix before you promise compatibility to downstream users

## 5. Keep Install Story And API Story Separate

- Homebrew, npm, and PyPI packages are install channels for the CLI
- they are not runtime APIs or SDK surfaces
- public API lives in the generated API section and in the documented workflows

## 6. Document The Handoff

A public-facing repo should make these things obvious:

- which lane is primary
- which additional lanes are genuinely supported
- which runtime it uses and whether that changes by target
- which command set is the canonical validation gate
- whether it depends on a shared runtime package or a Go SDK path

## Final Rule

If a teammate cannot clone the repo, run the documented flow, pass `validate --strict`, and understand the chosen lane without tribal knowledge, the project is not yet production-ready.
