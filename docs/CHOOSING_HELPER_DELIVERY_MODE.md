# Choosing Helper Delivery Mode

This document explains the two supported helper-delivery modes for launcher-based `python` and `node` plugins on `codex-runtime` and `claude`.

Both modes are first-class supported paths.
They expose the same supported handler-oriented helper API through different operational tradeoffs.

## The Two Modes

- `vendored helper`: the default `plugin-kit-ai init` path writes helper files into the plugin repo itself, such as `plugin/plugin_runtime.py` or `plugin/plugin-runtime.ts`
- `shared runtime package`: `plugin-kit-ai init ... --runtime-package` keeps the helper API in `plugin-kit-ai-runtime` on PyPI or npm and imports it as a dependency instead of writing the helper file into `plugin/`

## Choose Vendored Helper When

- you want the default self-contained scaffold
- you want `init -> bootstrap` to stay as hermetic as possible
- you want the repo to keep working even if your team is not yet standardizing on a shared PyPI/npm helper version
- you want the helper implementation to stay visible in the repo for easy local inspection

This is the default because it is the smoothest first-run path for repo-local interpreted plugins.

## Choose Shared Runtime Package When

- your team wants one shared Python or Node helper dependency across multiple plugins
- you want to update helper behavior through a normal package upgrade path instead of copying scaffolded helper files between repos
- you are comfortable pinning and maintaining the shared package version in `requirements.txt` or `package.json`
- you want new scaffolds to match the eventual reusable dependency story from day one

Current opt-in scaffold commands:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python --runtime-package --runtime-package-version 1.1.0
plugin-kit-ai init my-plugin --platform codex-runtime --runtime node --typescript --runtime-package --runtime-package-version 1.1.0
plugin-kit-ai init my-plugin --platform claude --runtime python --runtime-package --runtime-package-version 1.1.0
plugin-kit-ai init my-plugin --platform claude --runtime node --typescript --runtime-package --runtime-package-version 1.1.0
```

Released stable CLIs can infer that pin automatically from their own tagged version.
Development builds should pass `--runtime-package-version` explicitly.

## What Does Not Change

- Go is still the recommended default when you want the smoothest production path and the least downstream runtime friction
- Python plugins still require Python `3.10+` on the machine running the plugin
- Node plugins still require Node.js `20+` on the machine running the plugin
- `plugin-kit-ai validate --strict` remains the canonical readiness gate
- `plugin-kit-ai install` still does not manage interpreted-runtime dependencies

## Recommended Team Policy

- choose `go` when you want the strongest supported path and a self-contained binary for downstream users
- choose `vendored helper` when you want the smoothest repo-local Python/Node start
- choose `shared runtime package` when you already know you want a reusable dependency across multiple Python/Node plugin repos

If you start with the default vendored helper and later move to `plugin-kit-ai-runtime`, that remains a supported switch in delivery model, not a fallback or deprecation path.
