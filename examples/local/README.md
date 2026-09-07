# Repo-Local Plugin Examples

> **Historical plugin-kit-ai v1 managed examples; baseline 1.2.4.**
> Commands, stability labels, and support claims below describe the preserved v1
> workflow. These are not root `plugin.json` starters for the standard MVP.
> Project migration is not available in v2 yet. Maintain legacy projects using
> the v1 1.2.4 command set. See [historical context](../../website/source/en/legacy/v1/index.md)
> and the [prepared, unreleased Build guide](../../website/source/en/build/index.md).
> Public activation remains gated; no v2 npm availability is implied.

All three examples have `plugin/plugin.yaml` targeting `codex-runtime`.
Their Python, Node, and TypeScript helpers demonstrate historical launcher
behavior, not the future offline MCP/Skill authoring loop. Preserve those
helpers and dependencies when consulting these examples.

These examples are reference implementations for the fast local plugin entrance layer.
For copy-first starter repos, see [../starters/README.md](../starters/README.md).

- [codex-python-local](./codex-python-local): repo-local `codex-runtime` example for Python teams using `plugin-kit-ai bootstrap .`, `.venv`, `validate --strict`, launcher-based `notify`, and the helper API in `plugin/plugin_runtime.py` that mirrors the shared `plugin-kit-ai-runtime` package
- [codex-node-local](./codex-node-local): repo-local `codex-runtime` example for Node teams using `plugin-kit-ai bootstrap .`, `validate --strict`, launcher-based `notify`, and the helper API in `plugin/plugin-runtime.mjs` that mirrors the shared `plugin-kit-ai-runtime` package
- [codex-node-typescript-local](./codex-node-typescript-local): repo-local `codex-runtime` example for TypeScript teams using `plugin-kit-ai doctor .`, `plugin-kit-ai bootstrap .`, built output under `dist/`, and the helper API in `plugin/plugin-runtime.ts` that mirrors the shared `plugin-kit-ai-runtime` package

These Node/TypeScript and Python examples are the `public-stable` repo-local local-runtime subset.
They are supported paths for teams that prefer those runtimes, but they still require Python or Node to be installed on the machine running the plugin.
Launcher-based `shell` authoring remains `public-beta` and is covered through runtime docs plus `polyglot-smoke`, not through a checked-in local example repo.
They complement, not replace, the production reference repos in [../plugins/README.md](../plugins/README.md).
Go now also has copy-first starters in [../starters/README.md](../starters/README.md), and the production examples remain the clearest long-term support and release story when you want the most self-contained delivery model.
