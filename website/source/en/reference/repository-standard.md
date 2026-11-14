---
title: "Repository Standard"
description: "What a healthy plugin-kit-ai repo should look like, and how to separate the project source from generated outputs."
canonicalId: "page:reference:repository-standard"
section: "reference"
locale: "en"
generated: false
translationRequired: true
---

# Repository Standard

This page defines the public shape of a healthy `plugin-kit-ai` repository.

## The Main Rule

The repository should make its intended setup obvious and its generated outputs reproducible.

In practice, that means:

- the project source is easy to locate
- generated target files are clearly outputs
- the primary target or targets in scope are visible
- the runtime choice or runtime policy is visible
- the validation command is documented

## What Should Be Easy To Find

A healthy repo should make these things discoverable without digging:

- the primary target or targets in scope
- the chosen runtime or runtime policy by target
- the canonical `validate --strict` command, or the validation commands if there are several targets
- runtime prerequisites such as Go, Python, or Node
- whether the repo uses a Go SDK path or a shared runtime package

## What Should Not Be The Source Of Truth

These should not act as the main source of truth:

- hand-edited generated target files
- wrapper install packages treated as runtime APIs
- tribal knowledge about “the command you actually need to run”

## Healthy Repository Signals

- `generate` can reproduce the target outputs
- `validate --strict` passes cleanly for the intended target, or for each target the repo publicly claims to support
- the repo explains its chosen path in public-facing docs or README material
- CI uses the same public readiness flow as local development

## Weak Repository Signals

- target files are patched by hand after generation
- the runtime or target choice is implicit or inconsistent across machines
- downstream users need maintainer guidance to reproduce the basic flow
- the repo promises support for areas outside the declared support boundary

## Relationship To This Docs Site

This public docs site treats the repository standard as the place where:

- authoring guidance becomes operational
- support boundaries become enforceable
- handoff becomes credible

Pair this page with [Authoring Workflow](/en/reference/authoring-workflow), [Production Readiness](/en/guide/production-readiness), and [Glossary](/en/reference/glossary).
