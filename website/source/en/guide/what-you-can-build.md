---
title: "What You Can Build"
description: "Use this page as the product map: what outputs exist, what the default start looks like, and how one repo can expand later."
canonicalId: "page:guide:what-you-can-build"
section: "guide"
locale: "en"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

# What You Can Build

Use this page as the product map. It shows what kinds of outputs exist, not when one repo should grow or split later.

plugin-kit-ai can start with one executable plugin and expand into additional supported outputs over time.

## Recommended Starting Shape

Start with one runtime path, usually Codex runtime with Go. That keeps the first repo simple and gives you the clearest validate-and-ship loop.

If your team already works in Node/TypeScript or Python, those are supported starting paths too.

## One Repo, Many Supported Outputs

From the same project, you can grow toward:

- runtime outputs for supported hosts
- packaged outputs when packaging is the real delivery requirement
- extension outputs for hosts that expect an extension artifact
- repo-owned integration setup when the repo mostly needs checked-in configuration for another tool

## What This Page Is Not For

Choosing Node or Python does not force you to decide every packaging or integration detail on day one.

This page is the overview. If your question is whether one repo should keep growing, read [One Project, Multiple Targets](/en/guide/one-project-multiple-targets).
