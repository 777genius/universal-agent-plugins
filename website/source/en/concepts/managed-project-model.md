---
title: "How plugin-kit-ai Works"
description: "How one repo stays the source of truth while you generate outputs, validate strictly, and hand off a clean result."
canonicalId: "page:concepts:managed-project-model"
section: "concepts"
locale: "en"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

# How plugin-kit-ai Works

plugin-kit-ai keeps one repo as the source of truth for your plugin. You edit the files you own, generate the outputs you need, validate the result strictly, and hand off a repo that stays predictable over time.

## The Short Version

The core loop is simple:

```text
source -> generate -> validate --strict -> handoff
```

That loop matters because the project is not just a starter template. The generated output can change as the target evolves, while your authored source stays clear and maintainable.

## One Repo As The Source Of Truth

The repo is where the plugin lives for real.

- authored files stay under your control
- generated outputs are rebuilt from that source
- validation checks the output you plan to ship
- handoff happens only after the generated result is clean

This lets one project grow carefully instead of scattering the same plugin logic across several repos.

## What You Actually Edit

You keep editing the project source and the plugin code you own. You do not treat generated output as the place where the project really lives.

That boundary is what keeps upgrades, target changes, and maintenance work manageable.

## Why This Is More Than Starter Templates

A starter template gives you an initial shape. plugin-kit-ai keeps managing the loop after day one:

- it regenerates target-specific output from the same source
- it validates what you are about to ship
- it keeps authored files and generated files clearly separated
- it lets one repo expand to more outputs later without rewriting the whole project model

## Where To Go Next

- Read [Project Source And Outputs](/en/concepts/authoring-architecture) for the authored-vs-generated boundary.
- Read [Target Model](/en/concepts/target-model) for the different output types.
- Read [One Project, Multiple Targets](/en/guide/one-project-multiple-targets) when you want to grow one repo further.
