---
title: "Packages And Integration Setup"
description: "When packaging or checked-in integration setup is the right answer instead of an executable runtime plugin."
canonicalId: "page:guide:package-and-workspace-targets"
section: "guide"
locale: "en"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

# Packages And Integration Setup

Not every project should ship as an executable runtime plugin.

Sometimes the real requirement is a package another system will load, an extension artifact, or checked-in integration setup that lives in the repo.

## The Short Rule

Choose packages or integration setup when the delivery shape matters more than running the plugin directly.

## Choose This Page When

This is the right path when:

- packaging is the real delivery requirement
- the host expects an extension or packaged artifact
- the repo mainly needs checked-in integration setup for another tool
- an executable runtime would add unnecessary operational work

## What Makes This Different From A Runtime Path

A runtime path is usually the clearest default when you want an executable plugin.

Packages and integration setup answer a different question: how should this plugin be delivered or wired into another system?

## The Safe Mental Model

Pick runtime when you want to run the plugin directly. Pick packages or integration setup when delivery shape is the main requirement.

## Codex Package Boundary

For the official Codex package lane, keep the bundle layout explicit and narrow:

- `.codex-plugin/` contains only `plugin.json`
- optional `.app.json` and `.mcp.json` stay at the plugin root

This package path is for the official Codex plugin bundle surface, not for mixing repo-local runtime wiring into the package layout.
