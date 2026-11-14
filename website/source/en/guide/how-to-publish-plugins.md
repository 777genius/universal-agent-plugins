---
title: "How To Publish Plugins"
description: "A practical guide to publishing plugin-kit-ai projects to Codex, Claude, and Gemini without confusing local apply with publication planning."
canonicalId: "page:guide:how-to-publish-plugins"
section: "guide"
locale: "en"
generated: false
translationRequired: true
---

# How To Publish Plugins

Use this guide when your repo is already authored in `plugin-kit-ai` and you want the clearest next step for Codex, Claude, or Gemini publication.

Start from a repo where `plugin-kit-ai generate .` and `plugin-kit-ai validate . --strict` already pass, so the publication commands read current managed artifacts instead of stale ones.

## What This Guide Covers

- which platforms support real local apply today
- which platform uses plan-and-readiness instead
- which command to run first
- what result to expect after the command finishes

## Quick Comparison

| Platform | Publication model | Real apply in `plugin-kit-ai` | Main command | What you get |
|---|---|---:|---|---|
| Codex | local marketplace root | yes | `publish --channel codex-marketplace` | `.agents/plugins/marketplace.json` plus `plugins/<name>/...` |
| Claude | local marketplace root | yes | `publish --channel claude-marketplace` | `.claude-plugin/marketplace.json` plus `plugins/<name>/...` |
| Gemini | repository/release readiness | no | `publish --channel gemini-gallery --dry-run` | a bounded publication plan and readiness diagnostics |

## The Short Rule

- use `publish` when you want a publication workflow
- use `publication` when you want an inspect or doctor view first
- Codex and Claude support real local apply today
- Gemini uses plan-and-readiness publication in v1, not local apply

The repo shape stays the same:

- `plugin.yaml` is the core plugin manifest
- `targets/...` holds target-specific authored inputs
- `publish/...` holds publication intent
- `publication` is the inspect and doctor surface
- `publish` is the publication workflow surface

## Publish To Codex

For Codex, publication means materializing a local marketplace root.

Run this first:

```bash
plugin-kit-ai publish ./my-plugin --channel codex-marketplace --dest ./local-codex-marketplace --dry-run
```

Apply it when the plan looks right:

```bash
plugin-kit-ai publish ./my-plugin --channel codex-marketplace --dest ./local-codex-marketplace
```

Expected result:

- `.agents/plugins/marketplace.json`
- `plugins/<name>/...`

A local root like that can already act as a Codex plugin source.

## Publish To Claude

For Claude, publication also means materializing a local marketplace root.

Run this first:

```bash
plugin-kit-ai publish ./my-plugin --channel claude-marketplace --dest ./local-claude-marketplace --dry-run
```

Apply it when the plan looks right:

```bash
plugin-kit-ai publish ./my-plugin --channel claude-marketplace --dest ./local-claude-marketplace
```

Expected result:

- `.claude-plugin/marketplace.json`
- `plugins/<name>/...`

## Publish To Gemini

For Gemini, publication does **not** mean building a local marketplace root.

In v1, `plugin-kit-ai` does three bounded things:

- validates publication intent
- checks repository readiness
- builds a publication plan

Start with readiness:

```bash
plugin-kit-ai publication doctor ./my-plugin --target gemini
```

Then inspect the publication plan:

```bash
plugin-kit-ai publish ./my-plugin --channel gemini-gallery --dry-run
```

Expected prerequisites:

- a public GitHub repository
- a valid `origin` remote pointing to GitHub
- the GitHub topic `gemini-cli-extension`
- `gemini-extension.json` in the correct root

Gemini uses plan-and-readiness publication in v1, not local apply.

## Plan Across All Authored Channels

Use this when one repo authors more than one publication channel:

```bash
plugin-kit-ai publish ./my-plugin --all --dry-run --dest ./local-marketplaces --format json
```

Important rules:

- it uses only authored `publish/...` channels
- it does not infer channels from `targets`
- it is planning-only in v1
- `--dest` is required only when authored channels include Codex or Claude local marketplace flows
- Gemini-only orchestration does not require `--dest`

If the repo authors only `gemini-gallery`, this also works:

```bash
plugin-kit-ai publish ./my-plugin --all --dry-run --format json
```

## Which Command Should I Run?

- I want a local Codex marketplace root: `plugin-kit-ai publish --channel codex-marketplace --dest <marketplace-root>`
- I want a local Claude marketplace root: `plugin-kit-ai publish --channel claude-marketplace --dest <marketplace-root>`
- I want Gemini publication readiness: `plugin-kit-ai publication doctor --target gemini`
- I want a Gemini publication plan: `plugin-kit-ai publish --channel gemini-gallery --dry-run`
- I want one combined publication plan: `plugin-kit-ai publish --all --dry-run` and add `--dest <marketplace-root>` when Codex or Claude authored channels are included

## Further Reading

- [CLI README publication section](https://github.com/777genius/plugin-kit-ai/tree/main/cli)
- [`plugin-kit-ai publish`](/en/api/cli/plugin-kit-ai-publish)
- [`plugin-kit-ai publication`](/en/api/cli/plugin-kit-ai-publication)
- [`plugin-kit-ai publication doctor`](/en/api/cli/plugin-kit-ai-publication-doctor)
