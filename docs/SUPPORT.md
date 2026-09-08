# Support And Compatibility Policy

## Universal Agent Plugins installer

For `agentplugins`, start with [native CLI installation](./NATIVE_INSTALL.md)
and the [client compatibility evidence](https://777genius.github.io/universal-agent-plugins/docs/en/reference/client-compatibility.html).
The matrix identifies tested installer/client versions and platforms;
package validity, installation, activation, runtime and OAuth are separate layers.

For help, [open a question or bug report](https://github.com/777genius/universal-agent-plugins/issues/new/choose)
with your OS/architecture, installer and client versions, the command used, and
the diagnostic output. Remove secrets and private paths before sharing output.
Use the [security reporting process](../SECURITY.md) for vulnerabilities.

## Historical plugin-kit-ai v1 policy

The remaining sections define the approved public contract for `plugin-kit-ai` after the `v1.0.0` release.

## Recommended Production Lanes

Treat these as the main production lanes the project is prepared to recommend publicly today.

- `Codex runtime Go`
  - promise: stable production-ready runtime lane inside the `Notify` contract
  - boundary: repo-owned generated runtime wiring and the stable `Notify` path only, not parity with every Codex CLI behavior
  - primary evidence: the runtime support matrix, generated config contract coverage, and deterministic Codex runtime tests
- `Codex package`
  - promise: stable production-ready official package lane
  - boundary: the documented `codex-package` manifest and bundle layout contract, not every possible future sidecar or vendor CLI behavior
  - primary evidence: target support matrix, package contract tests, and `generate|import|validate` coverage
- `Gemini packaging`
  - promise: stable production-ready official Gemini CLI extension packaging lane
  - boundary: package-generation and documented extension packaging workflows, not unrelated runtime parity claims
  - primary evidence: target support matrix plus deterministic `generate|import|validate` coverage
- `Gemini Go runtime`
  - promise: stable production-ready 9-hook Go runtime lane
  - boundary: the promoted 9-hook Go runtime contract only
  - primary evidence: runtime support matrix, deterministic Gemini runtime smoke, and dedicated opt-in live runtime smoke
- `Claude default stable lane`
  - promise: stable production-ready Claude lane for `Stop`, `PreToolUse`, and `UserPromptSubmit`
  - boundary: the stable default hook subset and documented package authoring support, not the wider beta hook set
  - primary evidence: runtime support matrix, strict generated hook-routing validation, and deterministic generated-project coverage
- `Python` and `Node` local runtime lanes
  - promise: recommended non-Go authoring lanes for supported local-runtime repos on `codex-runtime` and `claude`
  - boundary: stable local interpreted subset only, with an explicit downstream runtime requirement on the target machine
  - primary evidence: `doctor`, `bootstrap`, `validate --strict`, export and bundle coverage, plus the deterministic `test-polyglot-smoke` lane

## Supported Advanced And Integration Lanes

- `OpenCode` and `Cursor` are repo-managed integration lanes. They are supported delivery and workspace-management surfaces, not weak targets and not the default runtime starting point.
- narrower or more specialized runtime expansions can be useful, but they should be adopted deliberately and described with their exact formal tier
- install wrappers, convenience flows, and other specialized surfaces should not be treated as if they carry the same promise as the main production lanes

## Public Language And Formal Terms

Use this mapping consistently across public docs and team policy:

- `Recommended` -> usually maps to promoted `public-stable` production lanes
- `Advanced` -> supported surface with a narrower, more specialized, or more careful contract
- `Experimental` -> opt-in surface without a normal compatibility promise

When a team needs exact compatibility policy, the formal terms win:

- `public-stable`: backward-compatible within a major release. Removal requires deprecation first.
- `public-beta`: supported and documented, but may change before promotion. Breaking changes require changelog notes.
- `public-experimental`: opt-in surface with no compatibility promise.
- `internal`: not part of the public contract.

## Canonical Contract References

The declared `v1` candidate set was reviewed through [V0_9_AUDIT.md](./V0_9_AUDIT.md). Post-`v1.0.0` community-first interpreted promotion is reviewed through [INTERPRETED_STABLE_SUBSET_AUDIT.md](./INTERPRETED_STABLE_SUBSET_AUDIT.md). Gemini runtime promotion is reviewed through [GEMINI_RUNTIME_AUDIT.md](./GEMINI_RUNTIME_AUDIT.md). Anything not listed remains `public-beta` or `internal`.

Canonical event-level support claims live in [generated/support_matrix.md](./generated/support_matrix.md). That table is the source of truth for:

- platform and event names
- runtime support status
- maturity
- `v1` target flag
- invocation and carrier shape
- scaffold and validate support
- transport modes
- capability tags
- live-test profile labels

The generated support matrix is runtime-event-only. Gemini runtime hooks, plus the Claude/Codex runtime lanes, appear there. Packaging-only or workspace-config-only targets are defined in this policy and in CLI docs.

The target/package contract matrix lives in [generated/target_support_matrix.md](./generated/target_support_matrix.md). That table is the source of truth for target class, production class, import/generate/validate support, portable component kinds, target-native component kinds, and managed artifact sets.

## Contract Levels

- `public-stable`: backward-compatible within a major release. Removal requires deprecation first.
- `public-beta`: supported and documented, but may change before promotion. Breaking changes require changelog notes.
- `public-experimental`: opt-in surface with no compatibility promise.
- `internal`: not part of the public contract.

## Current Contract

The current source tree is split between `public-stable`, `public-beta`, and `public-experimental`.
This section is the exact formal ledger. It should stay stricter and more exhaustive than the front-door product phrasing.

## Exact Contract Vocabulary

Use these terms consistently in public docs, generated artifacts, and CLI output:

- `production-ready`: runtime path covered by the current stable promise
- `public-stable`: compatibility tier for promoted public surfaces
- `public-beta`: supported but not yet covered by the stable promise
- `public-experimental`: opt-in surface with no compatibility promise
- `runtime-supported but not stable`: implemented runtime path that still remains `public-beta`
- `packaging-only target`: target with manifest/generate/import support but without a production-ready runtime contract

## Current Public-Stable

SDK packages and stable root API:

- `github.com/777genius/plugin-kit-ai/sdk`
- `github.com/777genius/plugin-kit-ai/sdk/claude`
- `github.com/777genius/plugin-kit-ai/sdk/codex`
- `github.com/777genius/plugin-kit-ai/sdk/gemini`
- `plugin-kit-ai.New`, `plugin-kit-ai.Config`, `plugin-kit-ai.App`
- `(*plugin-kit-ai.App).Use`
- `(*plugin-kit-ai.App).Claude`
- `(*plugin-kit-ai.App).Codex`
- `(*plugin-kit-ai.App).Gemini`
- `(*plugin-kit-ai.App).Run`
- `(*plugin-kit-ai.App).RunContext`
- `plugin-kit-ai.Supported`

Stable event surfaces:

- Claude:
  - `Stop`
  - `PreToolUse`
  - `UserPromptSubmit`
- Codex:
  - `Notify`
- Gemini:
  - `SessionStart`
  - `SessionEnd`
  - `BeforeModel`
  - `AfterModel`
  - `BeforeToolSelection`
  - `BeforeAgent`
  - `AfterAgent`
  - `BeforeTool`
  - `AfterTool`

Current production-ready target boundary:

- Claude: production-ready within the stable `Stop`, `PreToolUse`, and `UserPromptSubmit` event set
- Claude package authoring also supports first-class `plugin/targets/claude/settings.json`, `plugin/targets/claude/lsp.json`, `plugin/targets/claude/user-config.json`, and `plugin/targets/claude/manifest.extra.json`
- Codex runtime: production-ready within the stable `Notify` path
- Codex package: production-ready official plugin package lane
- Codex package bundle contract: `.codex-plugin/` contains only `plugin.json`, while optional `.app.json` and `.mcp.json` stay at plugin root and must match manifest refs
- Gemini packaging: production-ready official Gemini CLI extension packaging lane through `plugin-kit-ai generate|import|validate` and local `extensions link|config|disable|enable`
- Gemini runtime: optional production-ready 9-hook Go runtime lane for `SessionStart`, `SessionEnd`, `BeforeModel`, `AfterModel`, `BeforeToolSelection`, `BeforeAgent`, `AfterAgent`, `BeforeTool`, and `AfterTool`, with dedicated deterministic runtime smoke and dedicated opt-in real CLI runtime smoke
- Cursor plugin packaging: packaging-only Cursor marketplace plugin lane through `plugin-kit-ai generate|import|validate`, `.cursor-plugin/plugin.json`, root `skills/**`, and optional shared `.mcp.json`; not a production-ready runtime target
- OpenCode: workspace-config lane through `plugin-kit-ai generate|import|validate`, `opencode.json.plugin`, inline `mcp`, validated mirrored `.opencode/skills/`, first-class `.opencode/{commands,agents,themes,tools}/`, first-class `default_agent`, `instructions`, `permission`, sanctioned `config.extra.json`, stable `.opencode/plugins/` plus `.opencode/package.json`, and JSON/JSONC plus explicit user-scope import; not a production-ready runtime target

Stable CLI commands:

- `plugin-kit-ai init`
- `plugin-kit-ai bootstrap` for `python` and `node` launcher-based projects on `codex-runtime` and `claude`
- `plugin-kit-ai doctor` for `python` and `node` launcher-based projects on `codex-runtime` and `claude`
- `plugin-kit-ai export` for `python` and `node` launcher-based projects on `codex-runtime` and `claude`
- `plugin-kit-ai bundle install` for local exported Python/Node bundles on `codex-runtime` and `claude`
- `plugin-kit-ai bundle fetch` for remote exported Python/Node bundles on `codex-runtime` and `claude`
- `plugin-kit-ai bundle publish` for GitHub Releases handoff of exported Python/Node bundles on `codex-runtime` and `claude`
- `plugin-kit-ai validate`
- `plugin-kit-ai capabilities`
- `plugin-kit-ai inspect`
- `plugin-kit-ai install`
- `plugin-kit-ai version`

Stable CLI bootstrap/setup path for `plugin-kit-ai` itself:

- `brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai` is the recommended package-manager install path for the CLI itself
- `npm i -g plugin-kit-ai` or `npx plugin-kit-ai@latest ...` is the official JavaScript ecosystem path for the CLI itself; this wrapper stays `public-beta`, downloads the matching published GitHub Releases binary, and verifies `checksums.txt`
- when the PyPI wrapper is published for a release, `pipx install plugin-kit-ai` or `pipx run plugin-kit-ai version` is the Python ecosystem path for the CLI itself; this wrapper stays `public-beta`, downloads the matching published GitHub Releases binary, and verifies `checksums.txt`
- for Python/Node plugin authoring helpers, the shared package path is `plugin-kit-ai-runtime` on PyPI and npm; this mirrors the scaffold helper API and stays separate from the CLI wrappers above
- helper delivery modes are documented in [CHOOSING_HELPER_DELIVERY_MODE.md](./CHOOSING_HELPER_DELIVERY_MODE.md); the default scaffold vendors helper files, while `init ... --runtime-package` switches to the shared dependency path; released CLIs pin the helper version automatically and development builds accept `--runtime-package-version`
- `scripts/install.sh` resolves the latest published stable release by default, verifies `checksums.txt`, auto-detects OS/arch, installs the matching GitHub Releases tarball into `BIN_DIR`, and can pass trailing script arguments to the installed CLI for one-shot commands
- `777genius/universal-agent-plugins/setup-plugin-kit-ai@v1` is the official CI setup action and reuses the same verified release contract instead of rebuilding from source in downstream repos

Current beta CLI commands:

- `plugin-kit-ai bootstrap` for launcher-based `shell` projects
- `plugin-kit-ai doctor` for launcher-based `shell` projects
- `plugin-kit-ai export` for launcher-based `shell` projects

Stable `plugin-kit-ai install` contract:

- installs third-party plugin binaries from GitHub Releases only
- requires `checksums.txt` in the selected release for verified installation
- supports `--tag` or `--latest` selection, but not both together
- writes a deterministic success summary with installed path, release ref/source, selected asset, and target GOOS/GOARCH
- permits overwrite only for existing files when `--force` is set
- does not include self-update or auto-update behavior for the `plugin-kit-ai` CLI itself

The release-layout compatibility boundary for `plugin-kit-ai install` is documented separately in [INSTALL_COMPATIBILITY.md](./INSTALL_COMPATIBILITY.md).

Stable generated scaffold contract:

- Codex runtime required authored files: `go.mod`, `plugin/README.md`, `plugin/plugin.yaml`, `plugin/launcher.yaml`, generated `cmd/<project>/main.go`, plus root boundary docs `CLAUDE.md` and `AGENTS.md`
- Codex package required authored files: `plugin/README.md`, `plugin/plugin.yaml`, plus root boundary docs `CLAUDE.md` and `AGENTS.md`
- Codex runtime optional authored docs: `plugin/targets/codex-runtime/config.extra.toml` for supported repo-local config passthrough beyond managed `model` and `notify`
- Codex package optional authored docs: `plugin/targets/codex-package/package.yaml` for overrides, `plugin/targets/codex-package/interface.json`, `plugin/targets/codex-package/app.json`, and `plugin/targets/codex-package/manifest.extra.json`; shared package metadata now defaults from `plugin/plugin.yaml`
- Claude required authored files: `go.mod`, `plugin/README.md`, `plugin/plugin.yaml`, generated `cmd/<project>/main.go`, plus root boundary docs `CLAUDE.md` and `AGENTS.md`
- stable launcher-based local-runtime scaffold subset on `codex-runtime` and `claude`:
  - `python`: `plugin/plugin.yaml`, `plugin/launcher.yaml`, `plugin/README.md`, launcher under `bin/`, plus supported manager manifests; default helper delivery vendors `plugin/plugin_runtime.py`, while `init ... --runtime-package` imports `plugin_kit_ai_runtime`; official shared helper package: `plugin-kit-ai-runtime`
  - `node`: `plugin/plugin.yaml`, `plugin/launcher.yaml`, `plugin/README.md`, launcher under `bin/`, plus supported manager manifests; default helper delivery vendors `plugin/plugin-runtime.{mjs,ts}`, while `init ... --runtime-package` imports `plugin-kit-ai-runtime`; TypeScript is the stable authoring mode via `--runtime node --typescript`; official shared helper package: `plugin-kit-ai-runtime`
  - `init --extras` for the stable interpreted `python`/`node` subset also emits `.github/workflows/bundle-release.yml`, an opt-in GitHub Actions workflow that uses `setup-plugin-kit-ai@v1` and runs `doctor -> bootstrap -> validate --strict -> bundle publish`
- native vendor files generated from `plugin/plugin.yaml` remain part of the scaffolded project contract

Runtime recommendation contract:

- Go is the recommended default when users want typed handlers, the strongest supported authoring path, and the least downstream runtime friction
- Go plugins normally ship as compiled binaries, so plugin users do not need a separately installed Python or Node runtime just to execute them
- Python and Node are stable supported authoring lanes for the repo-local interpreted subset on `codex-runtime` and `claude`
- Python and Node projects must make their external runtime requirement explicit to users up front:
  - Python plugins require Python `3.10+` on the machine running the plugin
  - Node plugins require Node.js `20+` on the machine running the plugin
- vendored helper files and shared runtime packages are both supported delivery modes for the same Python/Node helper API

## Current Public-Beta Surfaces

Current beta surfaces that remain intentionally outside the stable set:
- OpenCode workspace-config lane through `plugin-kit-ai generate|import|validate`, covering official-style `opencode.json` and `opencode.jsonc`, package refs including tuple-form plugin options, inline `mcp`, validated portable skills mirrored into `.opencode/skills/`, first-class workspace commands/agents/themes, first-class `default_agent`, `instructions`, `permission`, first-class beta standalone tools mirrored into `.opencode/tools/`, stable official-style local JS/TS plugin code mirrored into `.opencode/plugins/`, stable shared dependency metadata mirrored into `.opencode/package.json` for tools and plugins, explicit `--include-user-scope` import from `~/.config/opencode`, and sanctioned `config.extra.json` for broader product config; `custom_tools` remain beta across standalone tools and plugin code
- Cursor workspace secondary lane through `plugin-kit-ai generate|import|validate`, covering repo-local `.cursor/mcp.json`, project-root `.cursor/rules/**`, authored `plugin/targets/cursor-workspace/AGENTS.md` merged into root `AGENTS.md`, and `--include-user-scope` import from `~/.cursor/mcp.json`; nested non-root `.cursor/rules/**`, JSONC, GUI-only/global rule authoring, and marketplace plugin packaging remain outside scope; not a production-ready runtime target
- optional extras generated by `plugin-kit-ai init --extras`
- `plugin-kit-ai init --platform claude --claude-extended-hooks` for the wider runtime-supported Claude hook scaffold beyond the stable default subset
- `plugin-kit-ai generate`, `plugin-kit-ai import`, and `plugin-kit-ai normalize`
- launcher-based `shell` runtime authoring on `codex-runtime` and `claude`, including `init --runtime shell`, `bootstrap`, `doctor`, `validate --strict`, and `export`
- experimental `plugin-kit-ai skills` authoring/generate subsystem and generated skill artifacts
- Claude official runtime-supported hooks not yet promoted to `public-stable`:
  - `SessionStart`
  - `SessionEnd`
  - `Notification`
  - `PostToolUse`
  - `PostToolUseFailure`
  - `PermissionRequest`
  - `SubagentStart`
  - `SubagentStop`
  - `PreCompact`
  - `Setup`
  - `TeammateIdle`
  - `TaskCompleted`
  - `ConfigChange`
  - `WorktreeCreate`
  - `WorktreeRemove`
- any newly added surfaces after the first stable set, until separately reviewed and promoted
- experimental local typed Claude hook registration helpers in `sdk/claude`
- experimental local typed Codex hook registration helper in `sdk/codex`

Config contract:

- canonical projects author under `plugin/`, with `plugin/plugin.yaml` as the authoring manifest and root `CLAUDE.md` / `AGENTS.md` as boundary docs
- the package-standard `plugin.yaml` schema is intentionally limited to package/build intent; unknown keys warn in `plugin-kit-ai validate`
- `plugin-kit-ai normalize` is the canonical cleanup path for rewriting unknown manifest content into the package-standard shape
- `plugin-kit-ai import` is the supported bridge from current native Claude/Codex/Gemini/OpenCode layouts back into the authored package-standard layout
- Codex runtime project-local config generated by `plugin-kit-ai generate` or `plugin-kit-ai init --platform codex-runtime`
- Codex runtime passthrough config lives in `plugin/targets/codex-runtime/config.extra.toml`; managed `model` and `notify` stay owned by `plugin/launcher.yaml` plus `plugin/targets/codex-runtime/package.yaml`
- live Codex CLI evidence currently splits into two buckets:
  - confirmed working paths: real `codex exec` with explicit `-c notify=...` override still reaches the repository-owned notify hook harness, the checked-in `examples/plugins/codex-basic-prod` runtime also passes that real `codex exec` path after rebuild, the same checked-in runtime example now also passes real `codex mcp get --json` and `codex mcp list --json` when its generated runtime MCP config is projected back through documented `-c mcp_servers...` overrides, real `codex mcp get --json` and `codex mcp list --json` with explicit `-c mcp_servers...` overrides still expose the projected MCP server contract, real `codex mcp add|get|list|remove` also work through an isolated temporary home for both stdio and streamable HTTP servers, those same mutable CLI config-management commands also pass when seeded from the checked-in runtime and package production examples, their auth-seeded variants also pass for those checked-in runtime and package production examples, a generated `codex-package` `.mcp.json` sidecar can be projected into those documented overrides for real `mcp get`, `mcp list`, plus `exec` MCP smoke, that same generated stdio sidecar now also passes real `codex mcp add|get|list|remove` through both isolated and auth-seeded config homes, a synthetic generated HTTP `codex-package` sidecar now also passes that same real `codex mcp add|get|list|remove` path in both isolated and auth-seeded config homes, auth-seeded live `codex mcp login|logout` also now prove the current CLI rejects stdio servers with stable OAuth-only diagnostics while preserving subsequent `get|list|remove`, and auth-seeded live `codex mcp get` plus `codex mcp remove` now also prove the current missing-server behavior after removal: `get` fails, while `remove` stays idempotent
  - evidence-only project-config probes: on the current live Codex CLI build (`v0.117.0` in repo evidence), `codex exec`, `codex mcp get`, and `codex mcp list` do not reliably honor project-local `.codex/config.toml`; this now holds both for a synthetic generated runtime workspace and for the checked-in `examples/plugins/codex-basic-prod` runtime example, across `exec`, `mcp get`, and `mcp list`. Separately, an isolated `CODEX_HOME` can successfully drive `codex mcp add|get|list|remove`, but a follow-up `codex exec` on that same isolated config currently loses live auth and therefore remains evidence-only. A stronger auth-seeded variant now also proves that copied live auth keeps `codex login status` plus documented stdio and HTTP `mcp add|get|list|remove` flows working inside a temporary `CODEX_HOME`, while the subsequent `codex exec` probe can still skip because the persisted MCP tool is not exposed in that session. The corresponding opt-in live tests record `skip` with captured output instead of claiming support that the vendor CLI did not prove
- Codex package manifest generated by `plugin-kit-ai generate` or `plugin-kit-ai init --platform codex-package`; first-class package metadata and `interface` live under `plugin/targets/codex-package/`, while `manifest.extra.json` remains passthrough-only for unsupported future fields
- Claude plugin metadata and hook routing files generated by `plugin-kit-ai generate` or `plugin-kit-ai init --platform claude`
- Gemini CLI extension manifest generated by `plugin-kit-ai generate --target gemini`, with optional Go launcher-based runtime support in the current production-ready 9-hook runtime contract
- OpenCode workspace config generated by `plugin-kit-ai generate --target opencode`, with workspace-config-only status in the current contract
- Cursor packaged plugin generated by `plugin-kit-ai generate --target cursor`, with `.cursor-plugin/plugin.json`, root `skills/**`, and optional shared `.mcp.json` in the current contract
- Cursor workspace config generated by `plugin-kit-ai generate --target cursor-workspace`, with workspace-config-only status in the current contract
- OpenCode local plugin loading stable subset is guarded by `generate --check`, strict validation, the production example canary, the documented `test-opencode-live` loader smoke path, and the documented `test-opencode-cli-live` real-model smoke path
- OpenCode standalone tools beta subset is guarded by `generate --check`, strict validation, the production example canary, and the documented `test-opencode-tools-live` smoke path
- OpenCode shared portable MCP initialization evidence is guarded by the documented `test-opencode-mcp-live` smoke path
- package-standard authored projects are defined by `plugin/plugin.yaml`, optional `plugin/mcp/servers.yaml`, optional `plugin/launcher.yaml`, optional `plugin/skills/**`, optional `plugin/publish/**`, and `plugin/targets/<platform>/...`
- generated native target files remain managed artifacts, not authored source-of-truth files
- generated Claude/Codex config wiring is a repo-owned contract surface guarded by `generate --check`, deterministic generated-project canaries, and the `polyglot-smoke` lane
- Claude authored hook routing must stay aligned with `plugin/launcher.yaml.entrypoint`; `validate --strict` is the enforcing gate for that consistency
- executable-runtime hardening currently includes generated launcher smoke for `go`, `python`, `node`, and `shell`, plus Windows `.cmd` validation coverage and ABI passthrough e2e
- stable local-runtime interpreted subset:
  - targets: `codex-runtime`, `claude`
  - runtimes: `python`, `node`
  - stable scope is scaffold, validate, launcher execution, repo-local bootstrap, read-only doctor checks, bounded portable export bundles, local exported bundle install, remote bundle fetch, and GitHub Releases bundle publish
  - `python`: Python `3.10+`; lockfile-first manager detection; `venv`, `requirements.txt`, and `uv` use repo-local `.venv`, while `poetry` and `pipenv` can validate against manager-owned envs
  - `node`: system Node.js `20+`; lockfile-first manager detection for `bun`, `pnpm`, `yarn`, or `npm`; JavaScript by default, TypeScript via `--runtime node --typescript`
  - operational tradeoff: interpreted runtimes are supported, but they are not zero-runtime-dependency delivery modes; the target machine still needs the appropriate external runtime installed
- beta local-runtime remainder:
  - `shell`: POSIX shell on Unix, `bash` required on Windows
  - supported scope is scaffold, validate, launcher execution, repo-local bootstrap, read-only doctor checks, and bounded portable export bundles
  - unsupported scope is universal package-management policy and packaged distribution through `plugin-kit-ai install`
- stable local bundle-install subset:
  - `bundle install` accepts only local `.tar.gz` bundles created by `plugin-kit-ai export`
  - supported subset: exported `python` and `node` bundles for `codex-runtime` and `claude`
  - unsupported scope: `shell`, remote URLs, registries, GitHub Releases, and implicit `bootstrap` or `validate`
- stable remote bundle-fetch subset:
  - `bundle fetch` supports direct HTTPS bundle URLs and GitHub Releases bundle discovery
  - URL mode verifies `--sha256` or `<url>.sha256`
  - GitHub Releases mode prefers `checksums.txt` and falls back to `<asset>.sha256`
  - supported subset: exported `python` and `node` bundles for `codex-runtime` and `claude`
  - unsupported scope: `shell`, registries, generic authenticated HTTPS distribution, and implicit `bootstrap` or `validate`
- stable GitHub bundle-publish subset:
  - `bundle publish` exports the same `python`/`node` bundle contract and uploads it to GitHub Releases
  - creates a published release by default; `--draft` keeps the target release as draft
  - uploaded assets are `<asset>.tar.gz` plus `<asset>.sha256`
  - supported subset: exported `python` and `node` bundles for `codex-runtime` and `claude`
  - unsupported scope: `shell`, registries, package-manager publishing, and generic HTTPS publishing
- community-first downstream setup path:
  - local recommended install path uses Homebrew
  - local JS ecosystem install path uses `npm i -g plugin-kit-ai` as `public-beta`
  - local Python ecosystem install path uses `pipx install plugin-kit-ai` as `public-beta` when that release was published to PyPI
  - local CLI bootstrap uses `scripts/install.sh`
  - CI bootstrap uses `777genius/universal-agent-plugins/setup-plugin-kit-ai@v1`
  - root GitHub Release assets come from `.github/workflows/release-assets.yml` and are the source of truth for downstream CLI channels
  - Homebrew tap updates follow successful `Release Assets` completion or a manual tag-scoped rerun and remain separate from `plugin-kit-ai install`
  - npm publishes follow successful `Release Assets` completion or a manual tag-scoped rerun and remain separate from `plugin-kit-ai install`
  - PyPI publishes follow successful `Release Assets` completion when trusted publishing is enabled, or a manual tag-scoped rerun when maintainers need that channel, and remain separate from `plugin-kit-ai install`
  - this setup path is separate from binary-only `plugin-kit-ai install`

Declared release review:

- production plugin authoring guide: [PRODUCTION.md](./PRODUCTION.md)
- stable-candidate ledger: [V0_9_AUDIT.md](./V0_9_AUDIT.md)
- post-`v1` interpreted stable-subset ledger: [INTERPRETED_STABLE_SUBSET_AUDIT.md](./INTERPRETED_STABLE_SUBSET_AUDIT.md)
- release playbook: [RELEASE.md](./RELEASE.md)
- release notes template: [RELEASE_NOTES_TEMPLATE.md](./RELEASE_NOTES_TEMPLATE.md)
- rehearsal worksheet: [REHEARSAL_TEMPLATE.md](./REHEARSAL_TEMPLATE.md)

## Internal Surfaces

These areas are not supported as public dependencies:

- `sdk/internal/...`
- `cli/plugin-kit-ai/internal/...`
- `install/plugininstall/internal/...`
- `install/plugininstall/adapters/...`
- `install/plugininstall/domain/...`
- `install/plugininstall/ports/...`
- generator implementation details and generated package internals

## Current Public-Experimental Surfaces

- `plugin-kit-ai skills init`
- `plugin-kit-ai skills validate`
- `plugin-kit-ai skills generate`
- canonical authored skill format under `skills/<name>/SKILL.md`
- generated Claude/Codex skill artifacts under `generated/skills/...`
- local typed Claude hook registration helpers:
  - `claude.RegisterCustomCommonJSON`
  - `claude.RegisterCustomContextJSON`
  - `claude.RegisterCustomPostToolUseJSON`
  - `claude.RegisterCustomPermissionRequestJSON`
- local typed Codex hook registration helper:
  - `codex.RegisterCustomJSON`

The skills subsystem is a compatibility-first authoring layer: `SKILL.md` remains the source of truth, execution remains language-neutral, and generated artifacts are derived outputs. None of this surface is covered by the stable compatibility promise yet.
Handwritten `SKILL.md` is supported; `plugin-kit-ai skills init` is convenience scaffold only, not a required authoring path.
See [SKILLS.md](./SKILLS.md) for usage guidance, examples, and when not to use it.

The custom hook helpers are intended as an escape hatch when Claude or Codex add hooks before `plugin-kit-ai` ships first-class support. They preserve typed handlers, but are not covered by the stable compatibility promise.

## Compatibility Rules

- `public-beta` changes must be called out in changelogs or release notes when user code, scaffold output, readiness semantics, or bundle contents change.
- `public-beta` surfaces are not covered by a backward-compatibility promise; before promotion, older beta-only paths may be removed directly as long as the current contract and resulting breakage are documented.
- The declared `v1` candidate set must be reviewed through [V0_9_AUDIT.md](./V0_9_AUDIT.md) before any surface is promoted.
- post-`v1` stable-promotion candidates must be reviewed through a dedicated promotion ledger such as [INTERPRETED_STABLE_SUBSET_AUDIT.md](./INTERPRETED_STABLE_SUBSET_AUDIT.md)
- OpenCode local plugin loading is promoted through [OPENCODE_STABLE_PROMOTION_AUDIT.md](./OPENCODE_STABLE_PROMOTION_AUDIT.md); helper-based custom tools remain `public-beta`
- OpenCode standalone tools beta evidence is tracked through [OPENCODE_TOOLS_BETA_AUDIT.md](./OPENCODE_TOOLS_BETA_AUDIT.md); `custom_tools` remain `public-beta`
- `public-stable` defines the post-`v1.0` compatibility promise for the approved set.
- No surface is promoted to `public-stable` until it has descriptor-backed docs, scaffold/validate alignment, and test coverage across unit, integration, contract, and smoke layers.
- Unified cross-platform abstractions are out of scope for the `v1` public contract unless they are explicitly declared later.

## Target Stable Boundary For The `v1` Candidate Set

This section defines what promotion means once a candidate surface moves from `public-beta` to `public-stable`.

SDK and CLI stable promotion means:

- no breaking changes outside a future major release
- removal only through deprecation first
- documented replacement path required for future replacements
- support docs and generated support metadata must match shipped behavior

Codex stable promotion means:

- stable registration API for `OnNotify`
- stable invocation mapping to `Notify`
- stable decode semantics for valid notify payload input
- stable response behavior
- stable scaffold and validate support for Codex plugin layout

Codex stable promotion does **not** mean:

- availability or health of local Codex installation
- success of Codex transport/auth/network/session startup
- absence of Codex runtime panics before hook firing
- stability of Codex internal logs or retry wording

Claude stable promotion means:

- stable registration APIs for `Stop`, `PreToolUse`, and `UserPromptSubmit`
- stable decode and response semantics for those events
- stable scaffold and validate support for Claude plugin layout

Claude runtime-supported beta expansion currently includes:

- `SessionStart`
- `SessionEnd`
- `Notification`
- `PostToolUse`
- `PostToolUseFailure`
- `PermissionRequest`
- `SubagentStart`
- `SubagentStop`
- `PreCompact`
- `Setup`
- `TeammateIdle`
- `TaskCompleted`
- `ConfigChange`
- `WorktreeCreate`
- `WorktreeRemove`

OpenCode stable promotion means:

- stable repo-local authored/generate/import/validate contract for `targets/opencode/plugins/**`
- stable repo-local authored/generate/import/validate contract for `targets/opencode/package.json`
- stable dependency-free official-style named async plugin scaffold/example shape
- stable explicit user-scope import normalization for project-local and `--include-user-scope` OpenCode plugin tree/package metadata
- stable deterministic loader smoke evidence through the documented `TestOpenCodeLoaderSmoke` path

OpenCode stable promotion does **not** mean:

- guaranteed availability of a local `opencode` binary
- guaranteed success of external OpenCode startup/auth/provider/network health before plugin load
- stable support for every possible helper-based custom tool implementation
- stable support for every possible standalone `.opencode/tools/**` behavior or helper pattern

Stable diagnostics boundary is limited to:

- runtime failure families
- validate failure kinds
- install exit-code families
- declared high-signal phrasing in `DIAGNOSTICS.md`

Release evidence note:

- Codex real smoke passed in the latest release-evidence refresh.
- Claude real smoke passed in the latest release-evidence refresh.
- Live install checks passed in the latest release-evidence refresh.
- Final release execution records the candidate SHA and tag in git history.

## Deprecation Rules

- Deprecated public surfaces must be marked in docs and changelogs before removal.
- Removal requires a documented replacement path.
- Deprecated `public-beta` surface may still change before `v1`, but removal should not be silent.
