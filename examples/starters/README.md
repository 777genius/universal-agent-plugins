# Canonical Starter Repos

These starter repos are the fastest way to get one working plugin repo that can later expand to more supported outputs.

Use them when you want to pick a stack, copy a template, get to the first green run quickly, and keep the repo open for later expansion.
For deeper contract examples, see [../local/README.md](../local/README.md) and [../plugins/README.md](../plugins/README.md).

## Install `plugin-kit-ai`

Use the supported CLI install order:

1. Homebrew: `brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai`
2. npm: `npm i -g plugin-kit-ai` or `npx plugin-kit-ai@latest ...`
3. pipx (when that release was published to PyPI): `pipx install plugin-kit-ai` or `pipx run plugin-kit-ai version`
4. Verified fallback: `curl -fsSL https://raw.githubusercontent.com/777genius/plugin-kit-ai/main/scripts/install.sh | sh`
5. CI: `777genius/universal-agent-plugins/setup-plugin-kit-ai@v1`

## Choose A Starter

- [codex-go-starter](./codex-go-starter): best default when you want the strongest production path
- [codex-python-starter](./codex-python-starter): best when the repo is intentionally Python-first and stays repo-local
- [codex-node-typescript-starter](./codex-node-typescript-starter): best mainstream non-Go path for TypeScript teams
- [claude-go-starter](./claude-go-starter): best when Claude hooks are the requirement and you still want the strongest production path
- [claude-python-starter](./claude-python-starter): Claude hooks path for Python-first teams
- [claude-node-typescript-starter](./claude-node-typescript-starter): Claude hooks path for TypeScript-first teams

Fast rule:

- choose Go for the strongest production path
- choose Node/TypeScript for the main supported non-Go path
- choose Python when the repo is intentionally Python-first
- choose Claude starters only when Claude hooks are the real requirement

What stays true after that choice:

- the starter is the first path, not the final boundary
- the repo can later grow to more supported outputs
- support depth depends on the target you add

## Shared-Package Variants

Ignore this section unless you already know you want the shared dependency path instead of vendored helper files:

- [codex-python-runtime-package-starter](./codex-python-runtime-package-starter): stable `codex-runtime` Notify starter for Python teams using `requirements.txt`, a repo-local `.venv`, and a pinned `plugin-kit-ai-runtime==1.1.0` dependency
- [claude-node-typescript-runtime-package-starter](./claude-node-typescript-runtime-package-starter): stable Claude hook starter for Node/TypeScript teams using `npm`, built output under `dist/main.js`, and a pinned `plugin-kit-ai-runtime@1.1.0` dependency

If you are unsure, start with the normal starter first and move to the shared-package variant only when the reusable-dependency requirement is real.

## Official Starter Templates

Use these when you want the real GitHub "Use this template" flow:

- [plugin-kit-ai-starter-codex-go](https://github.com/777genius/plugin-kit-ai-starter-codex-go)
- [plugin-kit-ai-starter-codex-python](https://github.com/777genius/plugin-kit-ai-starter-codex-python)
- [plugin-kit-ai-starter-codex-node-typescript](https://github.com/777genius/plugin-kit-ai-starter-codex-node-typescript)
- [plugin-kit-ai-starter-codex-python-runtime-package](https://github.com/777genius/plugin-kit-ai-starter-codex-python-runtime-package)
- [plugin-kit-ai-starter-claude-go](https://github.com/777genius/plugin-kit-ai-starter-claude-go)
- [plugin-kit-ai-starter-claude-python](https://github.com/777genius/plugin-kit-ai-starter-claude-python)
- [plugin-kit-ai-starter-claude-node-typescript](https://github.com/777genius/plugin-kit-ai-starter-claude-node-typescript)
- [plugin-kit-ai-starter-claude-node-typescript-runtime-package](https://github.com/777genius/plugin-kit-ai-starter-claude-node-typescript-runtime-package)

Click "Use this template" on one of those repos, then follow the starter README inside the generated repo.
The shared-package variants now use the same official template flow as the default starters.

## Quickstart

1. Copy one starter into a new repo.
2. Get to the first green run:

```bash
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
plugin-kit-ai validate . --platform <codex-runtime|claude> --strict
```

For Go starters, use the SDK-first first run instead:

```bash
go test ./...
go build -o bin/<starter-name> ./cmd/<starter-name>
plugin-kit-ai validate . --platform <codex-runtime|claude> --strict
```

3. Run the local smoke command from that starter README.
4. Expand or ship only after the first repo works:

- Python/Node starters already include `.github/workflows/bundle-release.yml` and the stable GitHub Releases handoff path:

```bash
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
plugin-kit-ai validate . --platform <codex-runtime|claude> --strict
plugin-kit-ai bundle publish . --platform <codex-runtime|claude> --repo owner/repo --tag v1.0.0
plugin-kit-ai bundle fetch owner/repo --tag v1.0.0 --platform <codex-runtime|claude> --runtime <python|node> --dest ./handoff-plugin
```

- Go starters stay on the SDK-first production path and consume `github.com/777genius/plugin-kit-ai/sdk@v1.1.0` as a normal module. Use `v1.1.0` or newer; `v1.0.3` was not a valid normal-module Go SDK release. Use the production guidance in [../plugins/README.md](../plugins/README.md) and [../../docs/PRODUCTION.md](../../docs/PRODUCTION.md) when you need the clearest long-term release story.

## Opinionated Defaults

- Go starters keep one canonical SDK-first story: `go test ./...` plus `go build -o bin/<starter-name> ./cmd/<starter-name>`
- Python starters keep one canonical env story: `requirements.txt` plus a repo-local `.venv`
- Node starters keep one canonical package-manager story: `npm`
- Python and Node starters include a helper layer so authors write handlers instead of hand-parsing launcher argv/stdin
- That helper layer also exists as the shared `plugin-kit-ai-runtime` package on PyPI and npm when teams want a reusable dependency instead of per-repo helper files
- Shared-package variants pin `plugin-kit-ai-runtime` to `1.1.0` so the reusable dependency path stays deterministic
- TypeScript starters keep built output under `dist/main.js`

Operational tradeoff:

- Go is still the recommended path when you want the most self-contained delivery model and the least downstream runtime friction
- Python starters require Python `3.10+` on the machine running the plugin
- Node starters require Node.js `20+` on the machine running the plugin

Supported alternatives still exist, but they are not encoded into the starter repos:

- Python alternatives: `uv`, `poetry`, `pipenv`
- Node alternatives: `pnpm`, `yarn`, `bun`

## Advanced Notes

- Shared-package variants are for teams that already know they want `plugin-kit-ai-runtime` as a reusable dependency instead of vendored helper files.
- Starter choice is about the first correct path, not the final limit of the product.
- If the repo later needs a wider scope, see [One Project, Multiple Targets](https://777genius.github.io/plugin-kit-ai/docs/en/guide/one-project-multiple-targets.html).
