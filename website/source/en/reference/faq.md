---
title: "FAQ"
description: "Short answers to the questions teams ask most often when starting and scaling plugin-kit-ai repos."
canonicalId: "page:reference:faq"
section: "reference"
locale: "en"
generated: false
translationRequired: true
---

# FAQ

## Should I Start With Go, Python, Or Node?

Start with Go unless you have a real reason not to.

Choose Node/TypeScript as the main supported non-Go path. Choose Python when the plugin stays local to the repo and your team is already Python-first.

## What Is The Simplest Python Setup?

Use the default Python scaffold first:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

Then edit the plugin, regenerate, and validate again.

See [Build A Python Runtime Plugin](/en/guide/python-runtime).

## When Should I Use `--runtime-package`?

Use `--runtime-package` only when you intentionally want one shared helper dependency across multiple repos.

Most teams should start with the default local helper first.

## Are npm And PyPI `plugin-kit-ai` Packages Runtime APIs?

No. They install the CLI. They are not runtime APIs and they are not SDKs.

## When Should I Use Bundle Commands?

Use bundle commands when another machine needs portable Python or Node artifacts to fetch or install.

Do not confuse bundle delivery with the main CLI install path.

## Can I Keep Native Target Files As My Source Of Truth?

No. The intended long-term model is to keep the source of truth in the package-standard layout and treat target files as generated output.

## Is `generate` Optional?

No, not if you want the managed project flow. `generate` is part of the workflow.

## Is `validate --strict` Optional?

Treat it as the main readiness check, especially for local Python and Node runtime repos.

## Can One Repo Own Multiple Targets?

Yes.

The practical rule is:

- keep the authored state in one managed repo
- start with the primary target you need today
- add more targets only when a real product, delivery, or integration need appears

See [One Project, Multiple Targets](/en/guide/one-project-multiple-targets) and [Target Model](/en/concepts/target-model).

## Are All Targets Equally Stable?

No.

Different paths carry different support promises. Use [Support Boundary](/en/reference/support-boundary) for the short answer and [Target Support](/en/reference/target-support) for the exact matrix.
