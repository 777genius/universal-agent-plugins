# plugin-kit-ai-runtime (PyPI authoring helper)

> **Supported legacy runtime helper.** The existing helper API remains supported
> for the earlier `plugin-kit-ai` runtime workflow. It is separate from the
> retired YAML CLI and from the current public installer,
> `universal-agent-plugins` (`agentplugins`). Start new portable packages with
> the [current documentation](https://universal-agent-plugins.com/docs/en/).
> The examples below describe this helper API; they do not add legacy authoring
> commands to the current installer release.

Official Python helper package for launcher-based `plugin-kit-ai` plugins.

Use it when you want the supported handler-oriented API as a shared dependency instead of generating a local `plugin/plugin_runtime.py` helper in each repo.

Most teams should start with the default local-helper path first and switch to this package only when they want one reusable helper dependency across multiple repos.

## Start Here

Use this package when:

- you want the same helper dependency across multiple plugin repos
- you want to import `plugin_kit_ai_runtime` from `requirements.txt` instead of keeping a generated helper file in `plugin/`
- you already know that the shared-package path is the right long-term fit

Do not use this package just because it sounds more production-like.

It does not remove the Python runtime requirement from the machine that runs the plugin.
If you want the simplest first repo, use the default local-helper path instead.

## Fastest Working Setup

Scaffold a Python project directly on the shared-package path:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python --runtime-package
cd my-plugin
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
plugin-kit-ai test . --platform codex-runtime --event notify
```

If Claude hooks are your real first requirement, scaffold with `--platform claude` instead and use:

```bash
plugin-kit-ai test . --platform claude --all
```

If you are testing from a local development build of the CLI instead of a released version, pass `--runtime-package-version` explicitly during `init`.
Released stable CLIs pin the matching helper version automatically.

Install:

```bash
pip install plugin-kit-ai-runtime
```

## Minimal `plugin/main.py`

Typical entrypoint when you want this mode from day one:

```python
from plugin_kit_ai_runtime import CodexApp, continue_

app = CodexApp()


@app.on_notify
def on_notify(event):
    _ = event
    return continue_()


raise SystemExit(app.run())
```

Keep stdout reserved for tool responses and write diagnostics to stderr only.

## Quick Decision Rule

- choose the default local-helper path when you want the smoothest first repo
- choose `plugin-kit-ai-runtime` when you want one reusable helper dependency across repos
- choose Go instead when you want the cleanest packaging and distribution story

## Notes

- Go is still the recommended path when you want the most self-contained delivery model.
- Python authoring remains a stable supported lane, but the machine running the plugin still needs Python `3.10+`.
- The helper API mirrors the generated `plugin/plugin_runtime.py` scaffold surface.

## Docs

- [setup guide](https://777genius.github.io/plugin-kit-ai/docs/en/guide/python-runtime/)
- [delivery model](https://777genius.github.io/plugin-kit-ai/docs/en/guide/choose-delivery-model/)
- [API reference](https://777genius.github.io/plugin-kit-ai/docs/en/api/runtime-python/)
