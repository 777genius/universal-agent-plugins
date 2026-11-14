---
title: "Target Model"
description: "How runtime, package, extension, and repo-owned integration outputs differ, and how to choose the right path."
canonicalId: "page:concepts:target-model"
section: "concepts"
locale: "en"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

# Target Model

A target is the kind of output you want the repo to produce.

The important choice is not abstract taxonomy. The important choice is what you are trying to ship.

## Quick Rule

- Choose a runtime path when you want an executable plugin.
- Choose a package path when another system will load your packaged output.
- Choose an extension path when the host expects an extension artifact.
- Choose a repo-owned integration setup when the repo mainly needs checked-in configuration for another tool.

## Runtime Paths

Runtime targets produce something executable. This is the default starting point for most teams because it is the clearest way to own behavior, validate output, and grow the repo later.

## Package Paths

Package targets produce packaged output instead of the main executable runtime shape. Use them when packaging is the real delivery requirement, not just an extra export you might need later.

## Extension Paths

Extension targets fit hosts that expect a specific extension artifact or installable package shape.

## Repo-Owned Integration Setup

Some outputs are mostly checked-in configuration that helps another tool or workspace use the plugin. These are still useful supported paths, but they answer a different delivery question than an executable runtime.

## The Safe Mental Model

Start with the output you need first. If the repo grows later, you can add another supported output without changing the fact that one project stays authoritative.
