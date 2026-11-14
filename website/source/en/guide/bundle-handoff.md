---
title: "Bundle Handoff"
description: "How to export, install, fetch, and publish portable Python and Node bundles for supported handoff flows."
canonicalId: "page:guide:bundle-handoff"
section: "guide"
locale: "en"
generated: false
translationRequired: true
---

# Bundle Handoff

Use this guide when a Python or Node plugin should travel as a portable artifact instead of as a live repo checkout.

This is a real public capability, but it is intentionally narrower than the main Go path.

## What It Covers

The stable bundle handoff subset is for:

- exported `python` bundles on `codex-runtime` and `claude`
- exported `node` bundles on `codex-runtime` and `claude`
- local bundle install
- remote bundle fetch
- GitHub Releases bundle publish

This is the right fit when:

- another team should receive a ready artifact instead of your full repo
- your release flow already uses GitHub Releases
- you want a cleaner handoff story for Python or Node runtimes

## The Practical Flow

The producer side is:

```bash
plugin-kit-ai export . --platform <codex-runtime|claude>
plugin-kit-ai bundle publish . --platform <codex-runtime|claude> --repo <owner/repo> --tag <tag>
```

The consumer side is either:

```bash
plugin-kit-ai bundle install <bundle.tar.gz> --dest <path>
```

or:

```bash
plugin-kit-ai bundle fetch <owner/repo> --tag <tag> --platform <codex-runtime|claude> --runtime <python|node> --dest <path>
```

After install or fetch, the resulting repo still needs its normal runtime bootstrap and readiness checks.

## What Does Not Happen Automatically

`bundle install` and `bundle fetch` do not silently turn the bundle into a fully validated plugin.

Treat the installed bundle as the start of downstream setup:

1. install runtime prerequisites
2. run `plugin-kit-ai doctor .`
3. run any required bootstrap step
4. run `plugin-kit-ai validate . --platform <target> --strict`

## When Bundle Handoff Is Better Than A Live Repo

Choose bundle handoff when:

- release artifacts are the real delivery contract
- downstream consumers should not clone the source repo
- you want repeatable GitHub Releases distribution for Python or Node lanes

Stay on the live repo path when:

- the team still edits the project source directly
- the main need is collaboration inside one repo
- Go already gives you the clean compiled-binary handoff you need

## Important Boundary

Bundle handoff is not “universal packaging for every target”.

It is a supported portable handoff flow for the exported Python and Node subset on `codex-runtime` and `claude`.

Do not assume the same contract applies to:

- Go SDK repos
- workspace-configuration targets such as Cursor or OpenCode
- packaging-only targets such as Gemini
- CLI install packages

## Recommended Reading Order

Pair this page with [Choose Delivery Model](/en/guide/choose-delivery-model), [Production Readiness](/en/guide/production-readiness), and [Support Boundary](/en/reference/support-boundary).
