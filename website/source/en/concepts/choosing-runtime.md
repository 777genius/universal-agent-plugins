---
title: "Choosing Runtime"
description: "How to choose between Go, Python, Node, and shell authoring paths."
canonicalId: "page:concepts:choosing-runtime"
section: "concepts"
locale: "en"
generated: false
translationRequired: true
---

# Choosing Runtime

Runtime choice is not just about language preference. It changes how the plugin runs, what the execution machine must have installed, and how simple CI and handoff will be.

<MermaidDiagram
  :chart="`
flowchart TD
  Start[Need a runtime lane] --> Prod{Need the strongest runtime lane}
  Prod -->|Yes| Go[go]
  Prod -->|No| Local{Is the plugin repo local by design}
  Local -->|Yes| Team{Is the team Python first or Node first}
  Team --> Python[python]
  Team --> Node[node or node --typescript]
  Local -->|No| Escape{Need only an escape hatch}
  Escape --> Shell[shell]
`"
/>

## Choose Go When

- you want the strongest runtime lane
- you want typed handlers and the cleanest release story
- you want the least bootstrap friction in CI and on other machines

## Choose Python Or Node When

- the plugin is repo-local by design
- your team already lives in that runtime
- you accept owning runtime bootstrap yourself
- you are comfortable with Python `3.10+` or Node.js `20+` being present on the execution machine

## Choose Shell Only When

- you need a narrow escape hatch
- you explicitly accept an experimental or advanced tradeoff

## Safe Default Matrix

| Situation | Recommended choice |
| --- | --- |
| Strongest runtime lane | `go` |
| Main non-Go runtime lane | `node --typescript` |
| Local Python-first team | `python` |
| Escape hatch | `shell` |
