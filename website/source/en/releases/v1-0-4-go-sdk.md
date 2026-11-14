---
title: "v1.0.4 Go SDK"
description: "Patch release notes for the Go SDK module path correction."
canonicalId: "page:releases:v1-0-4-go-sdk"
section: "releases"
locale: "en"
generated: false
translationRequired: true
---

# v1.0.4 Go SDK

Release date: `2026-03-29`

## Why This Patch Matters

This patch made the public Go SDK module path truthful for normal Go consumption.

## What Changed

- the Go SDK module root moved from `sdk/plugin-kit-ai/` to `sdk/`
- the public module path `github.com/777genius/plugin-kit-ai/sdk` now matches the real repo layout
- starter repos, examples, and templates were updated to stop teaching `replace`-based newcomer workarounds

## Practical Guidance

- use `github.com/777genius/plugin-kit-ai/sdk@v1.0.4` or newer for normal Go module consumption
- treat `v1.0.3` as known-bad for the Go SDK module path

## Why Users Should Care

This patch reduced friction for normal Go consumers and made the recommended SDK path look like a normal public module instead of a special-case workaround.
