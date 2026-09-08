# Executable Plugin ABI

`plugin-kit-ai` supports an executable plugin ABI for runtimes beyond Go. This ABI is the polyglot contract for repository-local executable plugins. The Go SDK remains the typed and recommended path when you want the most self-contained production story.

Current status: `public-stable` for repo-local `python` and `node` authoring plus exported bundle handoff on `codex-runtime` and `claude`; launcher-based `shell` remains `public-beta`.

For the `plugin-kit-ai` CLI itself, the recommended package-manager install path is Homebrew: `brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai`. The official JavaScript ecosystem path is `npm i -g plugin-kit-ai` or `npx plugin-kit-ai@latest ...`; this wrapper stays `public-beta`, downloads the matching published GitHub Releases binary, and verifies `checksums.txt`. When a release is published to PyPI, the Python ecosystem path is `pipx install plugin-kit-ai` or `pipx run plugin-kit-ai version`; this wrapper stays `public-beta`, downloads the matching published GitHub Releases binary, and verifies `checksums.txt`. The shared Python/Node authoring helper path is `plugin-kit-ai-runtime` on PyPI and npm; those packages mirror the scaffold helper API and stay separate from CLI installation. The verified fallback bootstrap path is `scripts/install.sh`, and the official CI setup path is `777genius/universal-agent-plugins/setup-plugin-kit-ai@v1`. `scripts/install.sh` also accepts one-shot pass-through commands such as `sh -s -- add notion --dry-run`. These install the CLI itself; they do not widen `plugin-kit-ai install`, which remains the stable binary-only installer for third-party plugin binaries.

Runtime matrix:

| Runtime | Status | Scope | Bootstrap |
|---------|--------|-------|-----------|
| `go` | stable | default SDK authoring path | Go `1.22+` to build; downstream plugin users run a compiled binary without a separately installed language runtime |
| `python` | stable local-runtime subset | repo-local executable ABI on `codex-runtime` and `claude` | Python `3.10+`; lockfile-first manager detection; `venv`/`requirements`/`uv` expect repo-local `.venv`, `poetry`/`pipenv` can use manager-owned envs |
| `node` | stable local-runtime subset | repo-local executable ABI on `codex-runtime` and `claude` | system Node.js `20+`; JavaScript by default, TypeScript via `--runtime node --typescript` |
| `shell` | public-beta | repo-local executable ABI | POSIX shell on Unix, `bash` on Windows |

## Invocation

Claude hooks:

- command shape: `<entrypoint> <HookName>`
- payload carrier: `stdin_json`
- example: `./bin/my-plugin Stop`

Codex `Notify`:

- command shape: `<entrypoint> notify <json-payload>`
- payload carrier: `argv_json`
- example: `./bin/my-plugin notify '{"client":"codex-tui"}'`

## Response Rules

- stdout is reserved for the upstream hook response payload
- stderr is reserved for diagnostics and human-readable errors
- exit code must be passed through unchanged by any launcher layer
- launcher scripts must not parse or transform hook payloads
- launcher scripts must not rewrite stdout

For Claude hooks, stdout must match Claude's upstream hook output format for the invoked event.
For Codex `notify`, successful completion is represented by exit code `0`; stdout is typically empty.

## Execution Model

- Go projects may use direct executable mode
- interpreted runtimes (`python`, `node`, `shell`) use a stable entrypoint plus a launcher wrapper
- the stable entrypoint path is recorded in `plugin.yaml` for new projects
- executable plugins are authored through the package standard layout: root `plugin.yaml` plus `targets/<platform>/...`
- current Claude/Codex/Gemini native config files stay as generated managed artifacts; they are not the authored source of truth
- repo-local authoring, validation, and launcher execution are supported for interpreted runtimes
- `plugin-kit-ai doctor` is the stable read-only readiness surface for the `python`/`node` local-runtime subset on `codex-runtime` and `claude`; `shell` remains beta
- `plugin-kit-ai export` is the stable portable handoff surface for the `python`/`node` local-runtime subset on `codex-runtime` and `claude`; `shell` remains beta
- `plugin-kit-ai bundle install` is the stable local bundle installer for exported `python`/`node` handoff bundles; it accepts local `.tar.gz` archives only, unpacks into `--dest`, and does not run `bootstrap` or `validate`
- `plugin-kit-ai bundle fetch` is the stable remote handoff companion for exported `python`/`node` bundles; URL mode verifies `--sha256` or `<url>.sha256`, GitHub Releases mode prefers `checksums.txt` and falls back to `<asset>.sha256`, then installs through the same local bundle contract
- `plugin-kit-ai bundle publish` is the stable GitHub Releases producer-side companion for exported `python`/`node` bundles; it reuses the same export contract, creates a published release by default, supports `--draft` as an opt-in safety mode, and uploads the bundle plus `<asset>.sha256`
- universal package management and packaged distribution through `plugin-kit-ai install` are out of scope for interpreted runtimes in this cycle
- Windows launcher resolution is platform-aware:
  - `python`: launcher resolution still prefers `.venv\Scripts\python.exe`, but `validate --strict` now treats `poetry` and `pipenv` manager-owned envs as ready without requiring repo-local `.venv`
  - `shell`: requires `bash` in `PATH`
  - generated launcher files use `.cmd`, while config entrypoints remain extensionless such as `./bin/my-plugin`
- `plugin-kit-ai validate --strict` is the canonical CI-grade readiness gate for interpreted runtimes and uses the same runtime lookup order as the generated launcher contract
- TypeScript is not a first-class runtime; the stable authoring path is Node runtime plus `--typescript`, which compiles to JavaScript and runs through the `node` runtime entrypoint
- generated Python and Node scaffolds include an official helper layer for handler-oriented authoring on top of the low-level executable ABI

Operational recommendation:

- choose Go when you want the least downstream setup friction and a compiled binary handoff
- choose Python or Node when your team already works in that runtime and repo-local iteration matters more than zero-runtime-dependency delivery
- make the external runtime requirement explicit to users up front: Python plugins need Python installed, Node plugins need Node installed

The launcher is intentionally minimal:

- discover the runtime
- locate the project root from the launcher path
- `exec` the runtime target with original argv
- preserve stdin/stdout/stderr/exit code

Current hardening coverage:

- generated launcher smoke exists for `go`, `python`, `node`, and `shell`
- ABI passthrough e2e verifies stdin/stdout/stderr/exit-code preservation
- generated-project canaries verify Claude stable hook routing, Codex `notify` argv wiring, and `generate --check` drift detection for generated runtime artifacts
- CI includes a dedicated `polyglot-smoke` lane for Ubuntu and Windows

## Non-Goals In This Iteration

- packaged distribution for Python or Node ecosystems through `plugin-kit-ai install`
- release/install packaging for Python or Node ecosystems through `plugin-kit-ai install`
- TypeScript-specific runtime support
- ABI changes to Claude or Codex wire formats
