# Changelog

All notable changes to this repository are documented here. **CLI releases** (`plugin-kit-ai` binary built from `cli`) are versioned together with the monorepo for now; SDK history remains in [sdk/CHANGELOG.md](sdk/CHANGELOG.md).

## [Unreleased]

Changes after `v1.2.4` land here.

### Added

- Unreleased source commands `agentplugins deps export` and `deps import`
  transfer the exact validated `uap.toml` / `uap.lock` pair through the current
  folder (or an optional directory), with a preview and confirmation for every
  copy. `deps import --install` restores through the strict locked installer;
  installation failures after saving retain the imported desired state.

- Dependency environments now accept stable exact/caret registry versions and
  transitive `plugin.json` dependencies, choosing one compatible minimum version
  per source in the existing `uap.lock`. Compatible pins replay unchanged;
  shared packages remain while required. Offline `deps tree` shows the graph,
  and `deps update <alias>` refreshes registry ranges without changing intent.
  Locked replay verifies pinned declarations and rejects incomplete graphs
  before installation or repair. This remains an unreleased source workflow.

- Locked install/sync fetches each missing immutable pin once per command,
  stages verified bytes through preflight and publishes the store after
  confirmation. Exact copied-lock restore permits superseded, non-revoked
  registry releases without changing versions or claiming installed history;
  publication identity, revocation, suspension and compatibility checks remain
  enforced. Ordinary fresh registry selection still requires an active release.

- `agentplugins install` is the everyday environment converge verb, matching
  how package managers are used: a first run installs everything declared in
  `uap.toml` and `uap.lock`, a repeat run is a successful nothing-to-do, and
  missing or drifted environment-owned projections are repaired from their
  recorded lock pin before converging to the locked state. It accepts
  `--locked`, `--yes` and `--external-uninstalled` with the same fail-closed
  meaning as `deps sync`. `install <name-or-source>` keeps the historical
  `add` alias contract, so the bare `install` command replaces the old alias;
  `agentplugins add` remains the canonical single-package verb.
- The `agentplugins` lifecycle commands now accept ordered, comma-separated
  targets such as `--target codex,cursor`, with versioned batch JSON output and
  explicit partial-failure reporting that preserves successful client changes.
- `agentplugins info <name> --target copilot --format json` now reports
  receipt reconciliation, exact native discovery, the observed client version,
  and path-free command evidence without changing its additive schema-version 1
  envelope or mutating installation state.
- Portable commands for Agent Plugins packages. A Markdown file in
  `io.github.777genius.agentplugins/commands/` becomes the skill
  `<slug>-<name>` in every client that takes skills. A command marked
  `disable-model-invocation: true` is explicit-only: it reaches a client only
  when that client is confirmed to keep it out of automatic invocation, and is
  otherwise withheld with the `explicit_invocation_not_enforceable` warning. No
  client is confirmed yet, so explicit-only commands are withheld everywhere in
  this release. GitHub Copilot CLI and Visual Studio Code receive no commands
  at all while Visual Studio Code is frozen: they share one installed package,
  so each command is withheld from both with the
  `command_not_supported_by_client` warning. See [docs/UAP_COMMANDS.md](docs/UAP_COMMANDS.md) and
  [ADR 0008](docs/adr/0008-portable-commands.md).
- `agentplugins validate` counts `commands` and `invalid_commands`, plans of
  `add`, `update` and `info` mark rendered skills with `source` and
  `source_name`, and command problems are diagnostics with `boundary: command`.

### Changed

The following `agentplugins` changes are breaking. They ship in the next minor
0.x release without a compatibility layer; each item says how to move.

- **`io.github.777genius.agentplugins/commands/` is now read.** Every entry in
  it except dotfiles must be a valid command; an invalid one is reported and
  not delivered. Move unrelated files elsewhere, and add the CI check from
  [docs/UAP_COMMANDS.md](docs/UAP_COMMANDS.md#checking-commands-in-ci), since
  `agentplugins validate` still exits with success on invalid commands.
- **Update refreshes command skills first.** For a package with commands,
  `update` of an unchanged revision now re-renders the command skills with the
  current rules before the ordinary update when the installed command skills
  differ from the current rendering, and changes nothing otherwise; other
  host projection changes are left alone, as for a package without commands. Its preview, single or grouped, is never unchanged:
  `update --all` and the batch update plan such a package instead of skipping
  it as current, and an embedded installer's preview reports
  `RequiresConfirmation` rather than `NoChange`. Hosts that skip unchanged
  previews need no change. After upgrading the installer, run
  `agentplugins update <name>` for packages with commands; `update --all`
  still skips Directory installations reported current by Discovery. Saved
  dependency environments pick up new rules at their next pin change.
- **Outdated command projections are refused.** Operations that keep the
  installed tree return `usecase.ErrProjectionRefreshRequired` (the same
  value as the new `domain.ErrProjectionRefreshRequired`) when the command
  skills of an intact package would render differently today: repair, both
  single-target and the group repair behind `agentplugins repair`, and, on
  clients that track native objects, a repeated add, a resumed activation and
  an unchanged group target. The embedded installer wraps it in
  `ErrUpdateRequired` with the `update_required` reason. Check for either with
  `errors.Is` and run `OpUpdate` instead. A saved dependency gets
  `usecase.ErrProjectionPinChangeRequired` instead, reported by the embedded
  installer with the `pin_change_required` reason and no next action, because
  it applies new command rules only at its next pin change.
- **Go API.** `installer.ComponentDecision` gains `Source` and `SourceName`, so
  unkeyed literals must name their fields. `clients.OpenCodeNativeRequirements`
  takes the client definition, because commands alone can deliver skills.
  `domain.SecurityEvaluationInput` gains `Commands` and `PluginName`; hosts
  that call the evaluator directly pass them so the scan covers the rendered
  commands. An adapter that sets `ClientTraits.EnforcesExplicitSkillInvocation`
  must implement `clients.ExplicitSkillDecorator`, and an adapter whose
  projection moves the package's `skills/` elsewhere must implement
  `clients.SkillsLocator`, so installed command skills can be read back.
- **Security review.** A package with commands is always scanned locally, or
  served from a cache entry for the same rendered commands; a signed Directory
  verdict is not accepted for it.
- **Authoring reports.** Invalid commands fail Readiness as `installer_policy`
  findings without changing Conformance or HostSafety. Compatibility reports a
  withheld explicit-only command as the warning `target_command_withheld`, and
  as an error only when the client gets nothing else.
- **Workspace lock renamed to `.uap-integrations.lock`.** The workspace desired
  state file written by `plugin-kit-ai integrations sync` is now
  `<repo>/.uap-integrations.lock`, so its name no longer collides with the
  legacy `plugin-kit-ai` branding or with the dependency environment lock
  `uap.lock`. `plugin-kit-ai integrations` commands read and write only the new
  name: a repository whose lock still carries the legacy
  `.plugin-kit-ai.lock` name fails with rename guidance instead of silently
  adopting or rewriting it, so migrate with `mv .plugin-kit-ai.lock
  .uap-integrations.lock`. The legacy name remains an accepted package-tree
  exclusion everywhere digests are computed, so existing workspaces see no
  digest drift. Authoring reports list the tree exclusion as
  `root non-directory .uap-integrations.lock`.

### Fixed

- A pending ChatGPT registration no longer cancels a multi-client Context7
  installation. Other selected clients are installed and ChatGPT is reported
  as a separate setup step with a ChatGPT-only resume command.
- `agentplugins add` now enters the guided Context7 setup for ChatGPT and Kiro
  automatically. The former `--prepare` flag has been removed, and ChatGPT
  registration guidance is formatted as four short, executable steps.
- Context7 preparation for ChatGPT now directs users to the working public
  `/mcp` endpoint with no authentication, accepts the `asdk_app_` identifier
  shown by ChatGPT, and reports authentication as not required. The previous
  `/mcp/oauth` guidance failed before Context7's OAuth flow could start.
- NPM registry publish workflows now require an explicit `NPM_PUBLISH_READY=true` repository variable before auto-running from release asset publication, while keeping manual dispatch available for credential repair.
- npm publication workflows now support npm Trusted Publishing through GitHub OIDC by default, while keeping token-based publishing as an explicit `NPM_PUBLISH_AUTH=token` fallback.
- Runtime package registry smoke now falls back to the registry's published latest version when a manual publish workflow cannot expose its dispatch tag through `workflow_run`.

## [1.2.4] - 2026-05-24

### Fixed

- Reissued the Cursor package installation release from current `main` so downstream Homebrew, npm, and PyPI publication workflows resolve the stable tag from the expected branch state.
- Hardened release asset publication so future stable tags must point at current `origin/main` before artifacts are built.

## [1.2.3] - 2026-05-24

### Added

- Cursor package installs now support `.cursor-plugin/plugin.json` bundles with generated `skills`, while preserving the older `.cursor/mcp.json` projection path for MCP-only Cursor plugins.

### Fixed

- Cursor integration state no longer reports the Cursor CLI as missing for local plugin package installs, because local Cursor plugin materialization does not require `cursor-agent` in `PATH`.

## [1.1.2] - 2026-04-19

### Fixed

- routed Gemini installs for GitHub repo-path sources through the managed local projection flow so first-party aliases such as `gitlab` and `notion` no longer fail during full multi-target `plugin-kit-ai add <source>` runs

## [1.1.1] - 2026-04-19

### Fixed

- restored integration installer compatibility with both legacy `src/plugin.yaml` trees and current `plugin/plugin.yaml` trees, which fixes `plugin-kit-ai add <source>` for first-party aliases such as `gitlab` and `notion` when the published CLI resolves current `universal-plugins-for-ai-agents` sources

## [1.1.0] - 2026-04-18

### Added

- repo-level lifecycle e2e coverage for managed integrations across `cursor`, `codex`, `opencode`, and `claude`, including add, update, remove, dry-run, persisted-state, and journal-phase verification
- public release pages for `v1.1.0` across the docs locales, plus generated sidebar updates so the current release appears in the published navigation

### Changed

- pinned Go SDK and shared `plugin-kit-ai-runtime` starter references now align on the `v1.1.0` version contract
- runtime authoring package upgrade smoke now exercises upgrades into `1.1.0` instead of stopping on the older patch line
- docs and website baseline messaging now point to `v1.1.0` as the current public release instead of leaving `v1.0.6` as the visible top-line baseline

Post-`v1.0.0` hardening on `main` continues here. The initial stable release was tagged as `v1.0.0` at commit `6e9379868a666e79d7530a02e171a160c2cb1689`.

### Added

- dedicated `polyglot-smoke` CI workflow for Ubuntu and Windows covering generated launcher smoke for `go`, `python`, `node`, and `shell`
- executable-ABI passthrough e2e coverage for stdin/stdout/stderr/exit-code preservation across generated runtime paths
- dedicated `release-assets` workflow to publish root GitHub Release archives and `checksums.txt` from a selected stable tag before downstream Homebrew/npm/PyPI channels consume the release

### Changed

- Windows launcher validation now accepts extensionless configured entrypoints such as `./bin/x` when the generated launcher file is `./bin/x.cmd`
- documentation now reflects post-`v1.0.0` contract status, the executable-ABI beta boundary, Windows runtime resolution rules, and the TypeScript-over-Node supported path
- Go SDK module root moved from `sdk/plugin-kit-ai/` to `sdk/`, making `github.com/777genius/plugin-kit-ai/sdk@v1.0.4` the first truthful normal-module release; `v1.0.3` remains published but is known-bad for Go SDK consumption
- maintainer-facing docs now distinguish monorepo Go `1.25.10` requirements from generated Go plugin projects that remain on Go `1.22+`, and the repository now ships root `LICENSE` and `SECURITY.md`

## [1.0.0] - 2026-03-26

Release tag: `v1.0.0`  
Release commit: `6e9379868a666e79d7530a02e171a160c2cb1689`

### Added

- **`docs/ARCHITECTURE.md`**, **`repotests/README.md`** — composition roots, exit-code notes, env vars for optional E2E.
- **`docs/FOUNDATION_REWRITE_VNEXT.md`** — Codex-first rewrite target: descriptor-driven core, platform-first API, explicit delivery phases, and acceptance bar for the foundation rewrite.
- **`docs/adr/`** — accepted rewrite ADR set for runtime foundation, descriptor system, unified capability policy, and transport model.
- **`cli/internal/app`** — `InstallRunner` / `InitRunner` between Cobra and `plugininstall` / `scaffold`; `plugin-kit-ai install` uses **signal-aware context** (interrupt/terminate).
- **`install/plugininstall`:** module `github.com/777genius/plugin-kit-ai/plugininstall` — GitHub Releases install with SHA256 (`checksums.txt`), `.tar.gz` / raw binary; **`domain.PickInstallAsset`**; **`ports.FileSystem`** **`PathExists`** / **`RemoveBestEffort`**; GitHub adapter split **`release.go`** / **`download.go`** (`NewClient` unchanged).
- **`plugin-kit-ai install`:** `owner/repo` with **`--tag`** or **`--latest`**; GoReleaser **`.tar.gz`** or **raw** `*-<goos>-<goarch>[.exe]` + mandatory **`checksums.txt`**; `[--dir bin] [--force] [--pre] [--output-name]`; optional `GITHUB_TOKEN` / `--github-token`; hidden `--github-api-base` for tests/Enterprise.
- **`cli`:** Cobra commands `init`, `install`, `version` (`runtime/debug.ReadBuildInfo`).
- **Workspace / tests:** `go.work` uses `./cli`, `./install/plugininstall`, `./sdk`; integration/guard tests live under **`repotests/`** (mock GitHub install, module guards, optional live E2E).
- **Integration test:** `plugin-kit-ai init` in a temp dir → `go mod edit -replace` to local SDK → `go test` / `go vet` on the generated module.
- **Repository tooling:** root `Makefile` (`make test`, `make vet`, optional **`make test-e2e-live`** — live GitHub install checks), `.goreleaser.yml`, `.github/workflows/ci.yml`, `scripts/install.sh` (bootstrap plugin-kit-ai; see comments for `plugin-kit-ai install`).

### Changed

- **Release candidate contract:** root docs, support policy, SDK stability notes, rehearsal artifacts, and audit ledger now describe the approved `v1.0` stable set plus the remaining beta leftovers.
- **Phase 1 foundation prep:** primary docs now describe only current shipped behavior; rewrite-target claims moved to `docs/FOUNDATION_REWRITE_VNEXT.md` and `docs/adr/`.
- **`plugin-kit-ai install`:** **`--latest`** (GitHub `releases/latest`); raw binary assets matching `claude-notifications-<goos>-<goarch>`; `--pre` for prerelease; **`--output-name`**; GitHub API client rejects release JSON above 32 MiB with a clear network error.
- **`install/plugininstall`:** atomic install writes `fsync` the temp file and best-effort `fsync` the install directory after `rename` (skipped on Windows).
- **Downloads:** HTTP client follows **HTTP→HTTP** redirects (e.g. `httptest`) while still blocking insecure downgrades from HTTPS; integration tests cover **302** asset URLs and **429** then success on `checksums.txt`.
- **Raw binary releases:** when several assets match `-GOOS-GOARCH`, companion tools named `sound-preview-*`, `list-devices-*`, `list-sounds-*` are ignored so repos like claude-notifications-go resolve to the main binary. Optional **`make test-e2e-live`** (`PLUGIN_KIT_AI_E2E_LIVE=1`, package **`./repotests`**) runs real GitHub install + `--version` checks.
