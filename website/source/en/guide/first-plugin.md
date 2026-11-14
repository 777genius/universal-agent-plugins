---
title: "Build Your First Plugin"
description: "A minimal end-to-end tutorial from init to strict validation."
canonicalId: "page:guide:first-plugin"
section: "guide"
locale: "en"
generated: false
translationRequired: true
---

# Build Your First Plugin

This tutorial now covers the narrow legacy-compatible Codex runtime Go path.

It keeps the scope intentionally narrow:

- first target: `codex-runtime`
- first language: `go`
- first readiness gate: `validate --strict`

If you are still choosing the path for a new repo, start with [Choose What You Are Building](/en/guide/choose-what-you-are-building) or [Build Custom Plugin Logic](/en/guide/build-custom-plugin-logic) first.

## 1. Install The CLI

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai version
```

## 2. Scaffold A Project

```bash
plugin-kit-ai init my-plugin
cd my-plugin
go mod tidy
```

This path still exists for backward compatibility, but it is no longer the recommended first start for new repos.

Run `go mod tidy` once after scaffold so the starter writes `go.sum` before the first strict validation pass.

## 3. Generate The Target Files

```bash
plugin-kit-ai generate .
```

Treat generated target files as outputs. Keep editing the repo through `plugin-kit-ai` instead of hand-maintaining generated files.

## 4. Run The Readiness Gate

```bash
plugin-kit-ai validate . --platform codex-runtime --strict
```

Use this as the main CI-grade gate for a local plugin project.

## What You Have Now

- one plugin repo
- authored files under `plugin/` for new repos
- generated Codex runtime output
- a clear readiness gate through `validate --strict`

## 5. When To Switch Paths

Switch to another path only when you actually need it:

- choose `claude` for Claude plugins
- choose `--runtime node --typescript` for the main supported non-Go path
- choose `--runtime python` when the project stays local to the repo and your team is Python-first
- choose `codex-package`, `gemini`, `opencode`, or `cursor` only when you really need a different way to ship the plugin

That does not mean the repo must stay single-target forever: start with the most important target today and add the others only when the product genuinely expands.

## Next Steps

- Read [Build Custom Plugin Logic](/en/guide/build-custom-plugin-logic) if what you really want is the advanced runtime path rather than the narrow legacy-compatible tutorial.
- Read [One Project, Multiple Targets](/en/guide/one-project-multiple-targets) if the one-repo, many-outputs idea is a core reason you care about the product.
- Use [Starter Templates](/en/guide/starter-templates) when you want a known-good example repo.
- Browse [CLI Reference](/en/api/cli/) when you need exact command behavior.
