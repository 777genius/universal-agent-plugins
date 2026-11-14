---
title: "Build A Python Runtime Plugin"
description: "A simple end-to-end path for a repo-local Python plugin."
canonicalId: "page:guide:python-runtime"
section: "guide"
locale: "en"
generated: false
translationRequired: true
---

# Build A Python Runtime Plugin

Use this path when your team already writes Python and you want the plugin to run from this repo.

If you want one compiled binary and the easiest distribution story, choose Go instead. Python is the supported path when the repo itself stays the main place where the plugin is developed and run.

## Choose Your Python Path In 10 Seconds

Use the default Python path when you want the simplest first repo:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
```

Use the shared-package path when you want to import `plugin_kit_ai_runtime` from `requirements.txt` across multiple repos:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python --runtime-package
```

If you are unsure, start with the default path first.

## What This Path Gives You

- one plugin repo
- Python `3.10+` on the machine that runs the plugin
- a local `.venv`
- a supported Python flow for `codex-runtime` or `claude`
- one main check before commit or handoff: `validate --strict`

## If You Only Want The Shortest Path

Copy this and get to the first green run:

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
cd my-plugin
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
plugin-kit-ai test . --platform codex-runtime --event notify
```

Only switch to `--runtime-package` after the shared-dependency requirement is real.

## 1. Install The CLI

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai version
```

## 2. Scaffold A Python Project

For the normal Python-first Codex path:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
cd my-plugin
```

If Claude hooks are the real first requirement, scaffold Claude instead:

```bash
plugin-kit-ai init my-plugin --platform claude --runtime python
cd my-plugin
```

## 3. Prepare The Local Python Environment

```bash
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
```

`doctor` tells you whether the repo is ready.

`bootstrap` creates `.venv` when needed and installs `requirements.txt`.

## 4. Generate And Validate

```bash
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

`generate` updates the generated launcher and config files from your source files.

For a Claude-first repo, switch the validate target:

```bash
plugin-kit-ai validate . --platform claude --strict
```

## 5. Add Your Python Logic

The default scaffold keeps the helper local in `plugin/plugin_runtime.py`, so the first version stays self-contained.

Typical Codex starter shape:

```python
from plugin_runtime import CodexApp, continue_

app = CodexApp()


@app.on_notify
def on_notify(event):
    _ = event
    return continue_()


if __name__ == "__main__":
    raise SystemExit(app.run())
```

Edit `plugin/main.py` for your plugin logic. Keep stdout reserved for tool responses and write diagnostics to stderr only.

## 6. Run A Smoke Test

For the Codex runtime path:

```bash
plugin-kit-ai test . --platform codex-runtime --event notify
```

You can also run the generated launcher directly:

```bash
./bin/my-plugin notify '{"client":"codex-tui"}'
```

For Claude, the simplest smoke check is:

```bash
plugin-kit-ai test . --platform claude --all
```

## 7. When To Use The Shared Python Package

Stay on the default local helper when you want the simplest first repo.

Use the shared dependency path when you want the same helper package across multiple repos:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python --runtime-package
```

That path imports [`plugin_kit_ai_runtime`](/en/api/runtime-python/plugin-kit-ai-runtime) from the published [`plugin-kit-ai-runtime`](https://github.com/777genius/plugin-kit-ai/tree/main/python/plugin-kit-ai-runtime) package instead of generating `plugin/plugin_runtime.py`.

If you are using a local development build of the CLI from this source tree, pass `--runtime-package-version` explicitly during `init`.
Released stable CLIs infer the matching helper version automatically.

## The Short Rule

- choose Python when the team is already in Python and the plugin is repo-local
- choose Go when you want the cleanest packaging and distribution story
- use `doctor -> bootstrap -> generate -> validate --strict` as the normal Python flow
- switch to `--runtime-package` only when you actually want a shared dependency

## Next Steps

- Read [Choosing Runtime](/en/concepts/choosing-runtime) for the runtime tradeoffs.
- Read [Choose Delivery Model](/en/guide/choose-delivery-model) for the local-helper vs shared-package decision.
- Open [Python Runtime API](/en/api/runtime-python/) when you need the helper reference.
