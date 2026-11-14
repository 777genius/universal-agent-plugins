---
title: "Troubleshooting"
description: "Fast recovery steps for the most common install, generate, validate, and bootstrap problems."
canonicalId: "page:reference:troubleshooting"
section: "reference"
locale: "en"
generated: false
translationRequired: true
---

# Troubleshooting

Use this page when the workflow stops moving. Start with the simplest check first.

## The CLI Installs But Does Not Run

Check that the binary is really on your shell `PATH`.

If you installed through npm or PyPI, make sure the package actually downloaded the published binary. Do not treat the wrapper package itself as the runtime.

## Python Or Node Runtime Projects Fail Early

Check the real runtime first:

- Python runtime repos require Python `3.10+`
- Node runtime repos require Node.js `20+`

Use `plugin-kit-ai doctor <path>` before assuming the repo itself is broken.

Typical recovery flow:

```bash
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

## `validate --strict` Fails

Treat this as signal, not noise.

Common causes:

- generated artifacts are stale because `generate` was skipped
- the selected platform does not match the project source
- the runtime path still needs bootstrap or environment fixes

## `generate` Output Looks Different Than Expected

That usually means the project source and your mental model drifted apart.

Re-check the package-standard layout instead of hand-editing generated target files to force the output you expected.

## I Am Unsure Which Path I Should Use

Start with the default Go path if you want the strongest contract.

Move to Node/TypeScript or Python only when the local-runtime tradeoff is real and intentional.

See [Build A Python Runtime Plugin](/en/guide/python-runtime), [Authoring Workflow](/en/reference/authoring-workflow), and [FAQ](/en/reference/faq).
