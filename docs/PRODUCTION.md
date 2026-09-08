# Production Plugin Workflow

This document is the canonical production authoring path for plugin authors using `plugin-kit-ai`.

If you are starting a new repo, use the job-first entry layer first:

- `plugin-kit-ai init my-plugin --template online-service`
- `plugin-kit-ai init my-plugin --template local-tool`
- `plugin-kit-ai init my-plugin --template custom-logic`

Those are onboarding categories, not new target IDs.
Under the hood they still map into the same target system, generation contracts, and support boundary described in the rest of this document.

## Current Target Boundary

- Claude: production-ready within the stable `Stop`, `PreToolUse`, and `UserPromptSubmit` event set when launcher-backed hooks are used, and also supports a production-ready package/config lane for package-only plugins under the same `claude` target
- Codex runtime: production-ready within the stable `Notify` path
- Codex package: production-ready official plugin package lane
- Gemini packaging: production-ready official Gemini CLI extension packaging lane through `generate|import|validate` and local `extensions link|config|disable|enable`
- Gemini runtime: optional production-ready 9-hook Go runtime lane for `SessionStart`, `SessionEnd`, `BeforeModel`, `AfterModel`, `BeforeToolSelection`, `BeforeAgent`, `AfterAgent`, `BeforeTool`, and `AfterTool`, with dedicated deterministic runtime smoke and dedicated opt-in real CLI runtime smoke
- Cursor: packaged plugin target with `.cursor-plugin/plugin.json`, root `skills/**`, and optional shared `.mcp.json`; `cursor-workspace` remains the secondary repo-local `.cursor/` subset when you explicitly need workspace files instead of a package bundle
- OpenCode: workspace-config-only target with a stable repo-local local-plugin-loading subset for official-style plugin subtree ownership and shared package metadata, plus first-class beta standalone tools and permission-first passthrough config semantics; `custom_tools` remain beta

Repo-local executable runtime boundary:

| Runtime | Current tier | Production guidance |
|---------|--------------|---------------------|
| `go` | stable | default production path |
| `python` | stable local-runtime subset | repo-local on `codex-runtime` and `claude`; lockfile-first manager detection with repo-local `.venv` readiness for `requirements`/`venv`/`uv`, manager-owned env readiness for `poetry`/`pipenv` |
| `node` | stable local-runtime subset | repo-local on `codex-runtime` and `claude`; system Node.js `20+`; lockfile-first manager detection plus TypeScript via `--runtime node --typescript` |
| `shell` | public-beta | repo-local only, POSIX shell on Unix, `bash` required on Windows |

Node/TypeScript and Python are the stable interpreted subset for scaffold, validate, launcher execution, repo-local bootstrap, read-only doctor checks, bounded portable export bundles, and local/remote bundle handoff on `codex-runtime` and `claude`.
Shell remains beta-hardening-only in this workflow.
This workflow does not imply a universal package-management contract or packaged distribution through `plugin-kit-ai install`.
Use Homebrew to install the `plugin-kit-ai` CLI locally when possible, use `npm i -g plugin-kit-ai` as the official `public-beta` JavaScript ecosystem path, use `pipx install plugin-kit-ai` as the `public-beta` Python ecosystem path only when that release was published to PyPI, use `scripts/install.sh` as the verified fallback, and use `777genius/universal-agent-plugins/setup-plugin-kit-ai@v1` to bootstrap the same verified CLI in CI.

Canonical authored inputs live under `plugin/`: `plugin/plugin.yaml`, optional `plugin/mcp/servers.yaml`, optional `plugin/launcher.yaml`, optional `plugin/skills/**`, optional `plugin/publish/**`, and `plugin/targets/<platform>/...`.
Plugin-root `README.md`, Claude/Codex/Gemini/Cursor/OpenCode native config files, and other root manifests are generated managed artifacts and should be treated as generated outputs. Root `CLAUDE.md` and `AGENTS.md` are committed boundary docs that tell humans and agents to edit only `plugin/`.

## Canonical Production Lane

Run this exact sequence before shipping a plugin repo:

```bash
plugin-kit-ai normalize .
plugin-kit-ai generate .
plugin-kit-ai generate --check .
plugin-kit-ai validate . --platform <claude|codex-runtime|codex-package|cursor|opencode> --strict
```

Then run the fixture-driven smoke:

- Claude: run `plugin-kit-ai test . --platform claude --all`; generated launcher-based Claude projects already scaffold `fixtures/claude/{Stop,PreToolUse,UserPromptSubmit}.json` and matching `goldens/claude/*`, and `--update-golden` is only needed when you intentionally want to refresh that checked-in output contract
- Codex: run `plugin-kit-ai test . --platform codex-runtime --event Notify`; generated launcher-based Codex runtime projects already scaffold `fixtures/codex-runtime/Notify.json` and matching `goldens/codex-runtime/*`, and `--update-golden` is only needed when you intentionally want to refresh that checked-in output contract
- Cursor: treat `generate --check` plus `validate --strict` as the packaged plugin readiness gate for `.cursor-plugin/plugin.json`, root `skills/**`, and optional shared `.mcp.json`
- OpenCode: run `make test-opencode-e2e-live` for the full stable live lane, or run `make test-opencode-live` for the loader smoke, `make test-opencode-cli-live` for stable real-model local-plugin-loading evidence, `make test-opencode-tools-live` for standalone-tools beta evidence, and `make test-opencode-mcp-live` for the shared portable MCP init proof; all remain opt-in and require `opencode` in `PATH`

For interpreted runtimes, add the bootstrap step before `validate --strict`:

- `doctor`: run `plugin-kit-ai doctor .` first when you want a read-only readiness verdict; it now reports the runtime/build-tool binaries visible to the current shell so PATH problems are obvious before bootstrap
- `python`: run `plugin-kit-ai bootstrap .`; `venv`, `requirements.txt`, and `uv` end in repo-local `.venv`, while `poetry` and `pipenv` can end in manager-owned envs
- `node`: run `plugin-kit-ai bootstrap .`; it chooses the detected install manager, and TypeScript-shaped Node projects also run `build`
- `test`: run `plugin-kit-ai test . --platform <claude|codex-runtime> --event <event>` for a single stable event, or `--all` for the full stable event set on the selected platform; fixtures default to `fixtures/<platform>/<event>.json`, goldens default to `goldens/<platform>/<event>.*`, generated launcher-based Claude and Codex runtime projects now pre-seed that layout during `init`, and `--update-golden` rewrites the current stdout/stderr/exitcode contract
- `shared helper packages`: `plugin-kit-ai-runtime` on PyPI and npm mirrors the supported Python/Node helper API when teams want a shared dependency instead of per-repo helper files; the default scaffold remains self-contained for hermetic first run, and `plugin-kit-ai init ... --runtime-package` is the official opt-in path when you want new projects to start on the shared dependency mode; released CLIs auto-pin the helper version, while development builds should pass `--runtime-package-version`
- helper delivery tradeoff: see [CHOOSING_HELPER_DELIVERY_MODE.md](./CHOOSING_HELPER_DELIVERY_MODE.md)
- `shell`: ensure the launcher target remains executable on Unix and `bash` is available on Windows; this path remains `public-beta`
- `export`: run `plugin-kit-ai export . --platform <codex-runtime|claude>` when you need a portable handoff bundle after readiness is already green; this is stable for `python` and `node`, beta for `shell`
- `bundle publish`: run `plugin-kit-ai bundle publish . --platform <codex-runtime|claude> --repo <owner/repo> --tag <tag>` when you want a producer-side GitHub Releases handoff for exported Python/Node bundles; it creates a published release by default, supports `--draft` when you want to keep the release as draft, and uploads the bundle plus `<asset>.sha256`; this path is stable and stays separate from `plugin-kit-ai install`
- `bundle install`: run `plugin-kit-ai bundle install <bundle.tar.gz> --dest <path>` when you need a local unpack/install handoff for exported Python/Node bundles; this path is stable for the Python/Node local-bundle subset and stays separate from `plugin-kit-ai install`
- `bundle fetch`: run `plugin-kit-ai bundle fetch --url <https://...tar.gz> --dest <path>` or `plugin-kit-ai bundle fetch <owner/repo> --tag <tag> --dest <path>` when you need a remote handoff path for exported Python/Node bundles; URL mode verifies `--sha256` or `<url>.sha256`, GitHub Releases mode prefers `checksums.txt` and falls back to `<asset>.sha256`, and this path is stable and separate from `plugin-kit-ai install`
- `init --extras`: for the stable interpreted `python`/`node` subset on `codex-runtime` and `claude`, this now emits `.github/workflows/bundle-release.yml`, which uses `setup-plugin-kit-ai@v1` and runs `doctor -> bootstrap -> validate --strict -> bundle publish`

After bootstrap, treat `validate --strict` as the CI-grade readiness gate for interpreted runtimes.
When CI or another tool needs structured output, use `plugin-kit-ai validate --format json`; it emits the versioned `plugin-kit-ai/validate-report` contract with `schema_version: 1` and `outcome` values `passed`, `failed`, or `failed_strict_warnings`.
Use [VALIDATE_JSON_CONTRACT.md](./VALIDATE_JSON_CONTRACT.md) for the ABI details and [CODEX_TARGET_BOUNDARY.md](./CODEX_TARGET_BOUNDARY.md) when deciding between `codex-runtime` and `codex-package`.

## Claude Release-Ready Path

- Start from `plugin-kit-ai init --platform claude` or `plugin-kit-ai import --from claude`
- Keep `plugin/plugin.yaml`, optional `plugin/mcp/servers.yaml`, and `plugin/targets/claude/...` as the authored source of truth
- Claude now supports two valid authoring modes under the same `claude` target:
  - runtime/hooks mode: `plugin/launcher.yaml` plus optional `plugin/targets/claude/hooks/hooks.json`
  - package-only mode: no `plugin/launcher.yaml`, with package/config surfaces such as `plugin/mcp/servers.yaml`, `plugin/skills/`, `plugin/targets/claude/settings.json`, `plugin/targets/claude/lsp.json`, `plugin/targets/claude/user-config.json`, `plugin/targets/claude/manifest.extra.json`, `plugin/targets/claude/commands/**`, or `plugin/targets/claude/agents/**`
- First-class Claude package docs include `plugin/targets/claude/settings.json`, `plugin/targets/claude/lsp.json`, `plugin/targets/claude/user-config.json`, and `plugin/targets/claude/manifest.extra.json`
- Commit generated `.claude-plugin/plugin.json`
- Commit generated `.mcp.json` when portable MCP is authored
- Commit generated `hooks/hooks.json` only when hooks are authored or the launcher-backed stable default hook projection is active
- `validate --strict` enforces that authored `plugin/targets/claude/hooks/hooks.json` command entries still match `plugin/launcher.yaml.entrypoint`
- Treat the stable promise as applying only to `Stop`, `PreToolUse`, and `UserPromptSubmit`
- The default Claude scaffold already matches that stable subset; use `--claude-extended-hooks` only as an explicit expansion step
- Treat additional runtime-supported Claude hooks as `public-beta` unless separately promoted

Reference implementation:

- [examples/plugins/claude-basic-prod](../examples/plugins/claude-basic-prod)
- [`context7` in universal-plugins-for-ai-agents](https://github.com/777genius/universal-plugins-for-ai-agents/tree/main/plugins/context7)

## Codex Runtime Release-Ready Path

- Start from `plugin-kit-ai init --platform codex-runtime` or `plugin-kit-ai import --from codex-runtime`
- Keep `plugin/plugin.yaml`, `plugin/launcher.yaml`, and `plugin/targets/codex-runtime/...` as the authored source of truth
- Commit generated `.codex/config.toml`
- Treat the stable promise as applying only to the `Notify` path

Reference implementation:

- [examples/plugins/codex-basic-prod](../examples/plugins/codex-basic-prod)

## Codex Package Release-Ready Path

- Start from `plugin-kit-ai init --platform codex-package` or `plugin-kit-ai import --from codex-package`
- Keep `plugin/plugin.yaml`, optional `plugin/mcp/servers.yaml`, plus `plugin/targets/codex-package/...` only when you need Codex-specific overrides
- Prefer shared package metadata in `plugin/plugin.yaml`; use `plugin/targets/codex-package/package.yaml` only for Codex-only overrides, `plugin/targets/codex-package/interface.json` for prompt/interface UX, and `plugin/targets/codex-package/manifest.extra.json` only for unsupported future manifest fields
- Commit generated `.codex-plugin/plugin.json` plus optional `.mcp.json` and `.app.json`
- Treat this lane as the official Codex plugin bundle, separate from local notify/runtime wiring

Reference implementation:

- [examples/plugins/codex-package-prod](../examples/plugins/codex-package-prod)

## OpenCode Release-Ready Path

- Start from `plugin-kit-ai init --platform opencode` or `plugin-kit-ai import --from opencode`
- Keep `plugin/plugin.yaml`, optional `plugin/mcp/servers.yaml`, optional `plugin/skills/**`, and `plugin/targets/opencode/...` as the authored source of truth
- Commit generated `opencode.json`, `.opencode/tools/**`, `.opencode/plugins/**`, and `.opencode/package.json`
- Treat the stable promise as applying to repo-local authored/generate/import/validate for local plugin subtree ownership and shared dependency metadata in `.opencode/package.json`
- Treat standalone `.opencode/tools/**` authoring as first-class `public-beta`
- Treat `custom_tools` as `public-beta` whether they ship through standalone tool files or plugin code
- Record `make test-opencode-live` plus `make test-opencode-cli-live` evidence whenever you are refreshing or asserting the OpenCode stable boundary
- Record `make test-opencode-tools-live` evidence whenever you are refreshing or asserting the standalone-tools beta boundary
- Record `make test-opencode-mcp-live` evidence whenever you are refreshing or asserting the OpenCode shared portable MCP init boundary

Reference implementation:

- [examples/plugins/opencode-basic](../examples/plugins/opencode-basic)

## Cursor Packaged Plugin Path

- Start from `plugin-kit-ai init --platform cursor` or `plugin-kit-ai import --from cursor`
- Keep `plugin/plugin.yaml`, optional `plugin/mcp/servers.yaml`, and optional `plugin/skills/**` as the authored source of truth
- Commit generated `.cursor-plugin/plugin.json`, generated `skills/**` when portable skills are authored, and generated `.mcp.json` when portable MCP is authored
- Treat this lane as the primary Cursor packaged-plugin path in the current contract; phase 1 intentionally keeps `agents/subagents/hooks/rules/commands` as target-native future work rather than pretending they are already part of the portable core

## Cursor Workspace Secondary Path

- Use `plugin-kit-ai init --platform cursor-workspace` or `plugin-kit-ai import --from cursor-workspace` only when you intentionally need the repo-local `.cursor/` subset
- Keep `plugin/plugin.yaml`, optional `plugin/mcp/servers.yaml`, and `plugin/targets/cursor-workspace/...` as the authored source of truth
- Commit generated `.cursor/mcp.json` and `.cursor/rules/**`
- Treat this lane as the documented Cursor workspace-config subset only. Do not assume support for global `~/.cursor/mcp.json`, nested non-root `.cursor/rules/**`, JSONC, or marketplace plugin packaging through this target

Reference implementation:

- [examples/plugins/cursor-basic](../examples/plugins/cursor-basic)

## What This Workflow Guarantees

- normalized `plugin.yaml` with no unknown fields
- generated native artifacts are in sync
- strict validation passes with no manifest drift and no Claude authored-hook entrypoint drift
- the committed example-shaped repo can build and execute a deterministic local smoke path
- OpenCode stable local plugin loading is evidenced through the deterministic marker-based `test-opencode-live` loader smoke path plus the real-model `test-opencode-cli-live` smoke path
- OpenCode standalone tools beta evidence is recorded separately through the deterministic marker-based `test-opencode-tools-live` smoke path
- OpenCode shared portable MCP initialization is evidenced through the deterministic `test-opencode-mcp-live` smoke path
- Cursor packaged-plugin readiness is bounded to deterministic generate/import/validate behavior for `.cursor-plugin/plugin.json`, root `skills/**`, and optional shared `.mcp.json`

## Gemini Packaging Boundary

- Start from `plugin-kit-ai init --platform gemini` or `plugin-kit-ai import --from gemini`
- Keep `plugin/plugin.yaml`, optional `plugin/mcp/servers.yaml`, optional `plugin/skills/**`, plus `plugin/targets/gemini/...` as the authored source of truth
- Commit generated `gemini-extension.json` plus generated `hooks/`, `commands/`, `policies/`, and selected context artifacts
- Treat Gemini packaging as the primary path: inline `mcpServers`, `contextFileName`, `settings`, `themes`, `excludeTools`, `plan.directory`, and `manifest.extra.json`
- Use `plugin-kit-ai inspect . --target gemini` to confirm the managed artifact set and whether the repo is still packaging-only or has the optional launcher-based Gemini runtime lane enabled
- Use `plugin-kit-ai init --platform gemini --runtime go` when you want the production-ready 9-hook Gemini Go runtime lane
- Use `plugin-kit-ai capabilities --mode runtime --platform gemini` to inspect the supported Gemini runtime surface, `make test-gemini-runtime` for the deterministic repo-local runtime gate, and `make test-gemini-runtime-live` for the matching opt-in real CLI runtime smoke when you need live evidence
- Use `gemini extensions link` for local development, `gemini extensions config` for install-time settings, and `gemini extensions disable|enable` to exercise scope changes; restart Gemini CLI after changes

## What It Does Not Guarantee

- external Claude CLI health before hook execution
- external Codex CLI health before `notify` execution
- automatic parity for future Gemini hooks outside the promoted 9-hook stable subset
- arbitrary OpenCode custom tool semantics beyond the documented tool/plugin/package-metadata contract
- dependency bootstrap beyond the bounded helpers, or packaged distribution through `plugin-kit-ai install`
