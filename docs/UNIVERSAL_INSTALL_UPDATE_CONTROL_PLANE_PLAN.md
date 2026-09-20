# Universal Install And Update Control Plane Plan

Plan date: 2026-04-08

This document defines the recommended architecture for a universal install and update control plane in `plugin-kit-ai`.

It is the lifecycle-management companion to:

- [Architecture Notes](./ARCHITECTURE.md)
- [Implementation RFC V1](./UNIVERSAL_INSTALL_UPDATE_CONTROL_PLANE_IMPLEMENTATION_RFC_V1.md)
- [Plugin Standard And Publish Plan](./PLUGIN_STANDARD_AND_PUBLISH_PLAN.md)
- [Publish Layer Spec](./PUBLISH_LAYER_SPEC.md)
- [plugin.yaml V1 Spec](./PLUGIN_YAML_V1_SPEC.md)
- [Support And Compatibility Policy](./SUPPORT.md)
- [Codex, Claude, And Gemini Publication Research](./research/plugin-marketplaces/README.md)

## Evidence policy

This document intentionally separates three classes of statements:

- `Confirmed vendor fact`: directly supported by current official vendor documentation
- `Architectural inference`: derived from confirmed facts, but not itself a vendor promise
- `Project policy`: an internal `plugin-kit-ai` design choice

This separation is mandatory.

If we blur vendor facts and our own abstractions:

- we will over-promise unsupported behavior
- we will encode the wrong lifecycle assumptions into adapters
- we will create support debt immediately

## Evidence promotion rules

Rules:

- a behavior starts as `architectural_inference` unless the current official docs support it directly
- live testing can raise confidence in an implementation, but it cannot upgrade an undocumented behavior into `confirmed vendor fact`
- a claim may be promoted from `architectural_inference` to `confirmed vendor fact` only when the source appendix and evidence registry are updated with an official documentation citation
- if vendor docs and observed behavior diverge, adapters must fall back to the stricter interpretation until the discrepancy is resolved
- every adapter capability that affects mutation safety, activation, or ownership must map to an explicit `evidence_key`

## Purpose

Build one user-facing install and update experience that can manage plugin-like integrations across multiple AI agents without pretending those agents share one native package format.

The control plane should let users do simple things:

- install one integration once
- apply it to all compatible local agents
- update it safely
- remove it safely
- repair broken installs
- automatically adopt newly added target support when policy allows

It should do that while preserving the real native contracts of:

- Claude plugins and marketplaces
- Codex plugins and marketplaces
- Gemini extensions
- Cursor MCP installs
- OpenCode plugins and config-based installs

## Core architectural decision

We will not build a universal vendor manifest or a fake universal marketplace format.

We will build a universal lifecycle control plane with:

1. one normalized source and state model
2. one application-layer install, update, remove, repair, and sync workflow
3. per-vendor target adapters underneath

This is the same strategic separation already established elsewhere in the repo:

- `plugin.yaml` stays the minimal universal plugin core
- `targets/...` stay vendor-specific authored inputs
- `publish/...` stays publication metadata
- the new control plane becomes the lifecycle layer above generated artifacts and above vendor-native install flows

## Capability baseline

The current target baseline should be treated as follows.

| Target | Native install surface | Native update surface | Restart or reload expectation | Scope model | Evidence class |
|-------|----------|----------|----------|----------|----------|
| Claude | marketplace add plus plugin install | marketplace refresh plus startup auto-update when enabled | plugin reload after updates | user, project, local, managed | confirmed |
| Codex | marketplace catalog plus plugin bundle | update local plugin dir plus restart Codex | restart expected for local changes | repo marketplace or personal marketplace | confirmed |
| Gemini | extension install or link | update, update all, auto-update, migrated source | restart required after management operations | user and workspace for enable or disable | confirmed |
| Cursor | MCP config, one-click install, deeplink, extension API, CLI inspection | documented config and approval surfaces, plus control-plane reconcile for owned entries | config refresh and auth completion may be needed | project and global configs | confirmed facts plus policy boundary |
| OpenCode | local plugins and npm plugins | startup-driven Bun install for npm plugins plus control-plane reconcile for owned files and declarations | startup-driven | project and global configs | confirmed facts plus policy boundary |

Reading rule:

- use confirmed native behavior where the docs are explicit
- use conservative reconcile behavior where the docs are not explicit
- never silently upgrade an inference into a product promise

## Conservative gaps register

This section tracks places where the current vendor docs do not justify a stronger claim.

### Codex

Confirmed gap:

- current official docs clearly document local marketplace setup, cache location, restart behavior, and plugin directory flows
- current official docs do not give Claude-style startup auto-update guarantees for local plugins

Conservative plan result:

- model Codex as `managed refresh`, not `native auto-update`

### Cursor

Confirmed gap:

- current official docs clearly document MCP install, config locations, extension API, deeplinks, one-click install, and CLI inspection
- current official docs do not document a single package-style `update` command for MCP integrations, and instead describe server-kind-specific refresh paths

Conservative plan result:

- model Cursor updates as owned-entry reconciliation plus server-kind-specific refresh guidance, not as a fake universal package updater

### OpenCode

Confirmed gap:

- current official docs clearly document local plugin loading, npm plugin declarations, startup installation using Bun, and load order
- current official docs do not document a dedicated plugin uninstall or update command comparable to Gemini

Conservative plan result:

- model OpenCode updates as reconcile or projection followed by normal startup loading

### Claude

Confirmed gap:

- current official docs clearly document marketplace auto-update behavior
- current docs do not imply that every plugin lifecycle action should bypass marketplace state

Conservative plan result:

- keep marketplace as a first-class object in adapter state

### General rule

Where the docs stop:

- do not invent stronger lifecycle claims
- keep the adapter conservative
- surface manual steps when needed

## Why this is the right direction

Confirmed vendor facts show the ecosystems are similar at the lifecycle level but different at the native packaging level:

- Claude has plugin marketplaces, scopes, and first-class plugin auto-update behavior
- Codex has plugin bundles and marketplaces, but its local update story is closer to refresh and restart semantics
- Gemini has the strongest extension update contract, including `install`, `update`, `update --all`, `--auto-update`, and native redirect metadata
- Cursor supports installable MCP integrations through config, one-click flows, and CLI management
- OpenCode supports local and npm plugin loading with startup-driven dependency install

That means the stable abstraction boundary is not "one bundle format". The stable boundary is "one lifecycle engine".

## Goals

- Give users a very small command surface for install and update.
- Keep native vendor semantics real and visible in adapters.
- Support safe update and removal with rollback and repair.
- Make "new target support was added in a later release" a first-class supported case.
- Scale to new agents by adding adapters, not rewriting use cases.
- Fit current repository architecture, especially clean composition roots and port/adapter boundaries.
- Be testable without live vendor CLIs by default, with opt-in live evidence for each adapter.

## Non-goals

- Do not define one universal marketplace file for Claude, Codex, Gemini, Cursor, and OpenCode.
- Do not collapse all target differences into `plugin.yaml`.
- Do not assume every target is a runtime plugin lane.
- Do not require every target to support native auto-update in the same way.
- Do not make `curl | bash` the core abstraction. Bootstrap scripts are UX wrappers, not the main architecture.

## Product framing

The control plane manages `Agent Integrations`.

An `Agent Integration` may expose one or more `Deliveries`.

Examples:

- Claude marketplace plugin delivery
- Codex marketplace plugin delivery
- Gemini extension delivery
- Cursor MCP delivery
- OpenCode plugin delivery

This vocabulary is better than forcing every target into the word "plugin" with identical meaning.

## Lifecycle phases

The control plane must separate lifecycle phases that vendor docs often blur together:

1. `resolve` - identify the requested source and resolve it to an immutable ref when possible
2. `plan` - inspect native state and produce a reconcile or mutation plan
3. `prepare` - fetch or stage source material without claiming the integration is usable
4. `materialize` - create or patch the vendor-native objects that represent the integration
5. `activate` - complete any vendor-specific step required before the integration becomes usable
6. `enable` - transition from disabled to enabled when the native platform models that separately
7. `reload_or_restart` - satisfy documented runtime refresh requirements such as `/reload-plugins`, a new thread, or a CLI restart
8. `verify` - re-inspect native state and persist the final installation record

Rules:

- `prepared` means source material exists or config is staged, but the control plane does not yet claim the integration is usable
- `installed` means the documented native representation exists and inspection can observe it
- `activation_pending` means the native representation exists, but vendor-documented activation is still required before normal use
- `reload_required`, `restart_required`, and `new_thread_required` are post-apply hints, not substitute install states
- adapters must not collapse `materialize` and `activate` unless the vendor docs justify doing so

## Hard vendor constraints

### Claude

Confirmed vendor facts:

- plugins are installed from marketplaces
- direct installs default to user scope
- the UI supports user, project, local, and managed scopes
- project-scope installs are written into `.claude/settings.json`
- marketplaces can be refreshed manually
- Claude Code can automatically update marketplaces and installed plugins at startup when auto-update is enabled
- official Anthropic marketplaces have auto-update enabled by default
- third-party and local development marketplaces have auto-update disabled by default
- after plugin updates, users should run `/reload-plugins`
- removing a marketplace uninstalls any plugins installed from that marketplace
- `strictKnownMarketplaces` restrictions can block marketplace additions entirely or limit them to an allowlist
- seed-managed marketplaces are read-only, and `remove` or `update` against them fails with guidance to ask an administrator to update the seed image
- when updating all marketplaces, seed-managed entries are skipped and other marketplaces still update
- seed directories take precedence over matching user-configured marketplaces on startup
- background auto-updates for private marketplaces cannot rely on interactive credential helpers; vendor docs require provider tokens in the environment for that flow
- `strictKnownMarketplaces` in managed settings can completely block marketplace additions or restrict them to an allowlist
- marketplace sources and plugin sources are pinned independently, and plugin sources can use `ref` or exact `sha`
- if both `plugin.json` and `marketplace.json` specify a plugin version, the plugin manifest version silently wins
- release channels can be modeled as separate marketplaces pinned to different refs or SHAs of the same repository
- relative-path plugin sources work only when the marketplace itself is added via Git, not via a direct URL to `marketplace.json`
- for relative `directory` or `file` sources, path resolution happens against the repository main checkout, so git worktrees still share the same marketplace location
- npm plugin sources are installed via `npm install`
- installed plugins are copied into cache locations, and vendor docs explicitly distinguish `${CLAUDE_PLUGIN_ROOT}` from `${CLAUDE_PLUGIN_DATA}` for persistent data that must survive plugin updates

Architectural inference:

- Claude adapter should prefer native marketplace workflows over file-copy hacks whenever possible
- marketplace identity and plugin identity should be tracked separately in state

Project policy:

- `plugin-kit-ai update` for Claude should map to native marketplace or plugin update semantics first, not bypass them
- Claude adapter should treat project-scope state in `.claude/settings.json` as a first-class native object
- Claude adapter must treat seed-managed marketplaces as `admin_managed` and never attempt mutation against them
- Claude adapter must treat `strictKnownMarketplaces` violations as policy blockers, not generic install failures
- Claude adapter version resolution must model marketplace-source pinning separately from plugin-source pinning

Primary sources:

- <https://code.claude.com/docs/en/discover-plugins>
- <https://code.claude.com/docs/en/plugin-marketplaces>

### Codex

Confirmed vendor facts:

- local marketplaces live under `$REPO_ROOT/.agents/plugins/marketplace.json` or `~/.agents/plugins/marketplace.json`
- local marketplace entries use `source.path` relative to the marketplace root
- installed copies live under `~/.codex/plugins/cache/$MARKETPLACE_NAME/$PLUGIN_NAME/$VERSION/`
- local plugins use `local` as the cache version segment
- Codex loads from the installed cache copy, not directly from the marketplace source directory
- plugin enable or disable state is stored in `~/.codex/config.toml`
- after changing a local plugin, docs instruct the user to update the plugin directory and restart Codex
- after installing a plugin, docs tell users to start a new thread and then use it
- uninstalling a plugin removes the plugin bundle from Codex, but bundled apps stay installed until managed in ChatGPT
- marketplace metadata requires `policy.installation`, `policy.authentication`, and `category` on each plugin entry
- `policy.installation` uses values such as `AVAILABLE`, `INSTALLED_BY_DEFAULT`, and `NOT_AVAILABLE`
- `policy.authentication` decides whether auth happens on install or first use
- `source.path` must stay relative to the marketplace root, start with `./`, and stay inside that root
- OpenAI docs say more plugin capabilities are coming soon and self-serve publishing to the official directory is not yet the main documented path

Architectural inference:

- Codex adapter must treat marketplace catalog, source bundle, and installed cache as separate concerns
- Codex adapter must own refresh and reconcile logic directly
- Codex adapter must not assume Claude-style startup auto-update because current docs do not promise it
- current official docs document interactive plugin-browser installation clearly, but do not document a standalone non-interactive `codex plugin install` command analogous to Gemini or Claude

Project policy:

- Codex is a managed-refresh target, not a native-auto-update target
- Codex removal logic must distinguish plugin-bundle ownership from bundled app ownership
- Codex adapter must not synthesize undocumented cache-write activation flows merely because cache paths are documented
- Codex adapter must treat marketplace catalog policy as discovery metadata, not as authoritative proof that a plugin is currently installed, activated, or authenticated on this machine

Primary sources:

- <https://developers.openai.com/codex/plugins>
- <https://developers.openai.com/codex/plugins/build>

### Gemini

Confirmed vendor facts:

- install command supports source, ref, auto-update, and pre-release flags
- sources can be GitHub repositories or local paths
- Gemini copies the extension during installation
- docs explicitly say `gemini extensions update` is required to pull source changes
- update commands exist for one extension and for all extensions
- extension management operations take effect after restarting the CLI session
- `gemini extensions link` exists for local development workflows
- `gemini-extension.json` supports native redirect metadata
- if redirect metadata is present, Gemini CLI checks the new source for updates and migrates automatically when an update is found
- enable and disable operations support user and workspace scopes
- extension management commands are intended for terminal use rather than interactive CLI mode
- Gemini loads extensions from `<home>/.gemini/extensions`
- Gemini user settings live in `~/.gemini/settings.json`, and workspace settings live in `<project>/.gemini/settings.json`
- Gemini also supports system defaults files and system settings files, and system settings override all other settings files for all users on the machine
- documented system-defaults paths are `/etc/gemini-cli/system-defaults.json` on Linux, `C:\\ProgramData\\gemini-cli\\system-defaults.json` on Windows, and `/Library/Application Support/GeminiCli/system-defaults.json` on macOS
- documented system-settings paths are `/etc/gemini-cli/settings.json` on Linux, `C:\\ProgramData\\gemini-cli\\settings.json` on Windows, and `/Library/Application Support/GeminiCli/settings.json` on macOS
- when Gemini starts, it loads all extensions and merges their configurations, and workspace configuration takes precedence on conflicts
- extension settings are stored in the extension directory, with values generally written to a `.env` file; when a setting is marked `sensitive: true`, the value is stored in the system keychain and obfuscated in the UI
- extension settings are updated through `gemini extensions config <name> [setting] [--scope <scope>]`
- in untrusted folders, Gemini CLI restricts extension install, update, and uninstall, ignores workspace settings, and does not connect MCP servers
- when trust is enabled, the trusted-folders feature itself is disabled by default and must be enabled in user settings
- Gemini stores trust decisions centrally in `~/.gemini/trustedFolders.json`
- trust resolution checks the IDE trust signal first when IDE integration is active, and falls back to `~/.gemini/trustedFolders.json` when IDE trust is unavailable
- `security.blockGitExtensions` blocks installing and loading extensions from Git
- `security.allowedExtensions` is a regex allowlist for extension sources and overrides `security.blockGitExtensions` when non-empty
- extension-contributed policy rules run in tier 2 alongside workspace-defined policies, below user and admin policies
- Gemini CLI ignores any `allow` decisions or `yolo` mode configurations contributed by extension policies
- Gemini configuration precedence also includes environment variables and command-line arguments above settings files

Architectural inference:

- Gemini is the strongest current native reference for version transitions and source migration
- Gemini link mode should be modeled separately from regular install mode

Project policy:

- the control plane should mirror Gemini’s distinction between installed source, linked source, and migrated replacement source
- the Gemini adapter must treat extension settings as user-owned values and avoid clobbering them during update or repair
- the Gemini adapter must surface trust-mode restrictions as environment blockers rather than misclassifying them as broken installs
- the Gemini adapter must not infer that an extension can auto-grant approval through bundled policy files
- the Gemini adapter must treat system settings as `admin_managed` and environment or CLI overrides as volatile non-owned layers
- the Gemini adapter should treat `~/.gemini/trustedFolders.json` as observed environment state, not a target for automatic mutation
- the Gemini adapter must treat IDE-reported trust as higher-priority observed state than local trust-file contents when both are present

Primary sources:

- <https://geminicli.com/docs/extensions/>
- <https://geminicli.com/docs/extensions/reference/>
- <https://geminicli.com/docs/extensions/releasing/>
- <https://geminicli.com/docs/cli/trusted-folders/>
- <https://geminicli.com/docs/reference/configuration>

### Cursor

Confirmed vendor facts:

- Cursor’s installable integration surface is MCP
- project config lives in `.cursor/mcp.json`
- global config lives in `~/.cursor/mcp.json`
- Cursor supports one-click MCP installation from its collection
- Cursor supports `Add to Cursor` deeplinks for MCP installation
- Cursor supports programmatic MCP registration through `vscode.cursor.mcp.registerServer()`
- `cursor-agent` uses the same MCP configuration as the editor
- the CLI supports MCP inspection and auth flows such as listing servers and logging into a server
- the CLI reports where a server comes from, such as project or global configuration
- the CLI follows the same configuration precedence as the editor: project, then global, then nested discovery from parent directories
- Cursor documents support for MCP protocol capabilities beyond tools, including prompts, roots, and elicitation
- the chat UI can enable or disable MCP tools directly
- `cursor-agent mcp disable <identifier>` removes a server from the local approved list
- Cursor supports `stdio`, `SSE`, and `Streamable HTTP` transports
- official Cursor docs include server-type-specific refresh guidance rather than one universal update command

Architectural inference:

- Cursor does not currently expose the same package-bundle lifecycle as Claude or Gemini
- the safest update abstraction is config reconciliation plus server-kind-specific refresh guidance, not a fake package update command

Project policy:

- Cursor adapter is an installable integration adapter, not a package-bundle adapter
- Cursor adapter should own project-vs-global config projection
- deeplink generation should live above the adapter layer as a UX surface, not inside install-state mutation logic
- Cursor adapter must not treat tool toggles or local approval state as equivalent to install or uninstall
- Cursor adapter inspect logic must respect parent-directory discovery so it does not patch the wrong effective config layer
- Cursor adapter should default to owned-entry reconciliation and explicit restart guidance, and should reserve destructive refresh steps such as remove-plus-readd for repair or for server kinds where the vendor docs explicitly describe that workflow

Primary sources:

- <https://docs.cursor.com/context/mcp>
- <https://docs.cursor.com/cli/mcp>
- <https://docs.cursor.com/deeplinks>
- <https://docs.cursor.com/en/tools/mcp>
- <https://docs.cursor.com/en/context/mcp-extension-api>

### OpenCode

Confirmed vendor facts:

- local plugins are loaded from `.opencode/plugins/` and `~/.config/opencode/plugins/`
- local plugin files are automatically loaded at startup
- npm plugins are declared in config and installed automatically using Bun at startup
- npm packages and dependencies are cached under `~/.cache/opencode/node_modules/`
- plugin load order is global config, project config, global plugin directory, project plugin directory
- duplicate npm packages with the same name and version are loaded once
- OpenCode config supports JSON and JSONC
- OpenCode config files are merged together rather than replaced
- OpenCode resolves effective config from documented remote, global, project, `.opencode`, and managed layers
- project config is loaded between global config and `.opencode` directories
- managed configuration stays above standard user-managed layers
- upward discovery still stops at the nearest Git directory for project-scoped resolution
- when OpenCode starts, it searches for project config in the current directory and traverses upward to the nearest Git directory
- OpenCode also supports remote configuration from `.well-known/opencode` and managed configuration from system-managed paths
- OpenCode file-based managed settings are admin-controlled and loaded above standard config sources, while macOS managed preferences sit above them as the highest non-user-overridable tier
- current troubleshooting guidance recommends clearing `~/.cache/opencode` if plugin installation is stuck
- troubleshooting docs also call out older installs that still use `~/.local/share/opencode/opencode.jsonc` as a global config location
- troubleshooting docs recommend isolating plugin issues by temporarily removing the `plugin` key or setting it to an empty array, and by moving plugin directories out of the way before clearing cache
- `.opencode` and `~/.config/opencode` use plural subdirectory names such as `plugins/`, but singular forms remain supported for backward compatibility

Architectural inference:

- OpenCode has startup-driven installation behavior, but current docs do not define a dedicated plugin update command
- the safe abstraction is reconcile or projection, not package-manager-style update semantics
- local file projection and npm package projection should be modeled separately

Project policy:

- OpenCode adapter must distinguish between local projection and package-based projection
- OpenCode adapter must preserve JSONC-compatible user config when patching plugin declarations
- OpenCode adapter inspect logic must account for custom config roots and upward project-config discovery
- OpenCode adapter must treat remote and managed config layers as non-owned unless the architecture explicitly adds enterprise administration support
- OpenCode adapter must model file-config precedence and directory overlay precedence as separate mechanisms
- OpenCode repair should try owned-plugin isolation and declaration cleanup before cache clearing, because the official troubleshooting flow treats cache clearing as a later step
- OpenCode adapter should normalize singular and plural legacy subdirectory forms during inspection, but emit canonical plural paths in new writes

Primary source:

- <https://opencode.ai/docs/plugins/>
- <https://opencode.ai/docs/config/>
- <https://opencode.ai/docs/troubleshooting/>

## Unsupported assumptions

The control plane must not assume any of the following:

- that Cursor has the same native package lifecycle as Claude, Codex, or Gemini
- that Codex supports startup auto-update for local plugins in the same way as Claude
- that OpenCode’s startup-driven npm install is equivalent to a stable explicit update command
- that one target’s scope model can be copied directly to another target without translation
- that a config layer discovered by an adapter is necessarily mutable by the current user
- that every MCP-backed delivery is tool-only; official docs already show richer capability surfaces on at least some targets

## External implementation references

These are references, not contracts:

- `amtiYo/agents` is a strong reference for desired-state sync across many agent clients, generated-artifact management, local overrides, and crash-safe sync locking
- `agent-resources` is a strong reference for multi-tool installer architecture, especially tool registry, per-tool path semantics, multi-target sync, and rollback
- `smithery` is a strong reference for client-aware MCP connection lifecycle
- `Alph` is a strong reference for local-first MCP setup with detector or planner or writer or validator separation, atomic writes, validation, and rollback
- `ai-config` is a useful reference for declarative multi-agent provisioning of instructions, hooks, skills, subagents, and MCP from a curated shared source
- `Agentloom` is a strong reference for canonical `.agents` state, additive source resolution, lockfile tracking, and one-way canonical-to-native sync
- `snowfort-ai/config` is a useful reference for centralized engine-state inspection, adapter-based schema validation, and backup-aware config patching
- `agnostic-ai` is a useful reference for canonical repo-owned configuration with generated IDE outputs and strategy-based MCP projection
- `task-master-ai` is a useful UX reference for one-click Cursor install and simple Claude MCP add flows
- `agent-rules-sync` is a useful reference for direction policies, portable projection, daemonized drift sync, and backup strategy
- `universal-plugins-for-ai-agents` is a strong reference for authored-source versus generated-output boundaries across Claude, Codex, Gemini, Cursor, and OpenCode
- `claude-notifications-go` bootstrap is a strong operational reference for idempotent install, update, reinstall, and repair behavior
- `mcp-config-manager` is a weaker but still useful reference for cross-client MCP config editing, backup flows, and disabled-server handling

Reference repositories and docs:

- `amtiYo/agents` - <https://github.com/amtiYo/agents>
- `agent-resources` - <https://github.com/kasperjunge/agent-resources>
- `Smithery` docs - <https://smithery.ai/docs/use/connect>
- `Alph` - <https://github.com/Aqualia/Alph>
- `ai-config` - <https://github.com/azat-io/ai-config>
- `Agentloom` - <https://github.com/farnoodma/agentloom>
- `snowfort-ai/config` - <https://github.com/snowfort-ai/config>
- `agnostic-ai` - <https://github.com/betagouv/agnostic-ai>
- `task-master-ai` - <https://github.com/eyaltoledano/claude-task-master>
- `agent-rules-sync` - <https://github.com/dhruv-anand-aintech/agent-rules-sync>
- `universal-plugins-for-ai-agents` - <https://github.com/777genius/universal-plugins-for-ai-agents>
- `claude-notifications-go` - <https://github.com/777genius/claude-notifications-go>
- `mcp-config-manager` - <https://github.com/holstein13/mcp-config-manager>
- `Agentloom` docs - <https://agentloom.sh/docs>
- `AllAgents` - <https://www.allagents.dev/>

Important rule:

- we should borrow architectural ideas, not clone product assumptions

Emerging market references to watch:

- `AllAgents` - plugin-registry and multi-workspace framing around "write once, sync to many clients"

Concrete patterns worth borrowing:

- `agent-resources`: per-tool path strategy objects instead of scattering tool-specific paths through orchestration code
- `agent-resources`: validated declarative dependencies and explicit default-source resolution instead of ad hoc source parsing
- `amtiYo/agents`: split between repo-declared desired state and user-local override state for MCP entries and secrets
- `amtiYo/agents`: stale-lock handling with owner metadata and atomic managed file writes
- `Alph`: explicit detector or registry or safe-edit layers instead of mixing detection, planning, and mutation in one command
- `ai-config`: declarative adapter manifests with per-scope path resolution and component installers
- `Agentloom`: canonical lockfile plus sync-manifest split between imported-source tracking and generated-output tracking
- `Agentloom`: managed-key merge for shared MCP config should preserve unmanaged fields instead of rewriting whole server objects
- `Agentloom`: generated-file manifests should normalize paths relative to workspace or home where possible so cleanup survives machine moves
- `Agentloom`: source discovery should support nested plugin roots discovered from marketplace metadata, not just one hard-coded source layout
- `snowfort-ai/config`: centralized engine-state caches are useful for `doctor` and UI or automation, but raw deep-merge patching is not enough for ownership-safe lifecycle control
- `agnostic-ai`: canonical repo-owned config plus generated outputs is useful, but symlink-heavy provisioning and raw overwrite strategies are weaker than explicit owned-entry reconciliation
- `claude-notifications-go`: idempotent bootstrap with version check, repair, and reinstall fallback

## External reference findings

The following findings are worth keeping in the plan because they directly influence `integrationctl` design.

### `amtiYo/agents`

Observed patterns from code:

- a registry of supported integrations with per-integration binary requirements
- repo-declared desired state in `.agents/agents.json`
- user-local mutable overrides in `.agents/local.json`
- generated target-native artifacts written into `.agents/generated/`
- generated per-target state files are kept next to generated artifacts instead of being hidden inside target configs
- integration sync implemented through a hook registry instead of hard-coded branching
- stale lock files include owner metadata such as pid, token, and start time
- secrets are split out of shared config into local overrides using placeholder projection
- trust state is inspected separately from normal config state
- sync can run in a read-only drift-check mode instead of always mutating
- config tracks schema version and last source fingerprint so drift can be reasoned about explicitly

Architectural consequence for `integrationctl`:

- keep desired workspace declaration separate from user-local mutable overrides
- use an explicit adapter registry, not scattered per-target branching
- treat secret projection and placeholder substitution as first-class install concerns
- keep locks crash-safe and owner-aware
- keep generated target-native artifacts under explicit module ownership, not mixed with authored declarations
- expose read-only drift detection as a first-class use case, not only best-effort status text
- keep adapter-owned state outside native target config files when the native format is not a reliable ownership store

### `agent-resources`

Observed patterns from code:

- `ToolConfig` centralizes per-tool path semantics and CLI behavior
- named source resolver supports explicit source selection and ordered fallback
- installed resources are matched using persisted metadata, not only path names
- remote fetch uses partial clone with fallback to full clone
- legacy installed naming is migrated carefully instead of being broken by schema changes
- stable install metadata is written into each installed resource directory so future sync does not rely on guessed names
- stable install ids are built from either the resolved local absolute path or the remote source plus handle, which makes rename and migration handling much safer than pure folder-name matching
- source resolution is ordered and explicit, with a default source plus named-source overrides instead of hidden host-specific heuristics
- clone failure handling is classified into auth, not-found, and network cases, which makes repair and user guidance much cleaner than raw git stderr

Architectural consequence for `integrationctl`:

- centralize target path and CLI traits in adapter capabilities or adapter config
- persist enough metadata to match installed resources robustly across upgrades and renames
- separate source resolution from installation logic
- support migration of legacy state and native object naming without destructive resets
- keep dependency declaration parsing and source-selection policy outside of target mutation code
- every installable delivery should have a stable identity that survives rename and path-shape changes
- identity construction should prefer resolved source identity plus canonical handle over generated install path names
- source resolvers should return typed failure categories, not only opaque process errors

### `Smithery`

Confirmed product patterns from docs:

- client-aware install flow via `smithery mcp add <url> --client <name>`
- separate remote-connection mode via Smithery Connect
- explicit `mcp update` and `mcp remove` for Smithery-managed connections
- deep-link protocol for client-specific MCP installation
- managed OAuth, automatic token refresh, and write-only credential storage
- remote connections expose explicit status such as `connected`, `auth_required`, and `error`
- scoped service tokens are used instead of exposing full API keys to agents or browsers

Architectural consequence for `integrationctl`:

- deep links belong in a UX or distribution layer above core mutation logic
- auth-bearing remote connections should be modeled separately from local config projection
- secure credentials should stay out of shared lock files and out of readable state when possible
- remote hosted connections should be treated as a different delivery family from local projected config
- auth state must be machine-readable and separate from ordinary install drift or health state

### `Alph`

Observed patterns from code and docs:

- registry-driven provider model with explicit provider contracts for `detect`, `configure`, `remove`, `list`, and optional `validate` or `rollback`
- read-only detection is separated from configuration writes
- configure flow is staged as detect -> build config -> preview -> confirm -> safe apply
- `safeEdit` implements backup -> validate -> atomic write -> validate -> rollback
- architecture is explicitly split into detector, planner, writer, validator, preview, and command layers
- redacted previews and redacted status output are first-class UX behavior, not an afterthought
- provider contracts explicitly separate `detect`, `configure`, `remove`, `list`, `has`, optional `validate`, and optional `rollback`
- atomic write falls back conservatively when plain rename is not safe on the current filesystem boundary

Architectural consequence for `integrationctl`:

- install and update use cases should keep detection, planning, apply, and verification as separate steps with typed outputs
- safe mutation needs a reusable primitive with backup, validation, and rollback semantics
- previews should be produced from the same plan model used for apply, not from ad hoc command text
- secret redaction belongs in report rendering and diagnostics by default
- adapter contracts should stay narrow and capability-driven instead of growing one generic mutable manager API
- the safe-write primitive should allow rename-first with explicit fallback behavior for filesystem edge cases

### `ai-config`

Observed patterns from code and docs:

- one interactive run chooses agents, one install scope, and selected MCP servers from a curated source tree
- adapters expose per-scope config paths and supported component sets
- installation is componentized into instructions, commands, skills, hooks, subagents, and MCP
- MCP installation is adapter-specific merge logic over native config files
- provisioning is primarily declarative copy and merge, with custom installer hooks for target-specific steps
- Codex enablement includes extra feature-flag mutation, followed by a documented restart requirement
- MCP install is intentionally thin: adapter-specific merge returns updated content and the installer writes it directly, without deeper ownership or post-apply verification

Architectural consequence for `integrationctl`:

- target adapters should declare supported component surfaces and scope-specific native paths explicitly
- component installers are useful, but they must feed into lifecycle state and verification instead of remaining copy-only utilities
- restart and reload requirements should be modeled as activation or post-apply requirements, not left as prose only
- curated-source provisioning is useful for bootstrap flows, but it is not enough by itself for robust update or repair semantics
- raw merge-and-write helpers are not sufficient for a production lifecycle engine unless they are wrapped with ownership checks, verification, and rollback

### `Agentloom`

Observed patterns from code and docs:

- canonical local scope is a `.agents/` directory containing entities, `mcp.json`, `agents.lock.json`, `settings.local.json`, and `.sync-manifest.json`
- `init` is explicitly the provider-to-canonical bootstrap step, while `sync` is intentionally one-way from canonical state to provider-native outputs
- source import is additive and priority-ordered across multiple candidate directories
- imported source tracking and generated output tracking are separated into different files with different responsibilities
- sync is entity-aware and preserves untouched generated outputs via manifest merge
- stale generated files are removed by diffing previous and next sync manifests
- Codex has provider-specific metadata in the manifest because its native output shape is special
- scope resolution is explicit and deterministic across local versus global operation modes
- shared MCP config entries are merged by replacing only managed keys on owned entries while preserving unmanaged keys already present in the native config
- sync manifest paths are normalized to relative workspace or home paths when possible so manifests stay portable across machines
- source preparation supports local paths, GitHub slugs, and git URLs, resolves commits, and can discover nested plugin roots through marketplace metadata before looking for canonical agents or MCP sources

Architectural consequence for `integrationctl`:

- bootstrap or import state should not be mixed with generated-output bookkeeping
- one-way reconcile from canonical state to native outputs is a strong default after bootstrap, because it avoids hidden bidirectional drift
- generated-file manifests are useful for cleanup and entity-scoped sync without touching unrelated outputs
- provider-specific sync metadata may live in the manifest when a target has a structurally special native shape
- source resolution order should be explicit, stable, and testable instead of ad hoc
- shared-config mutation should preserve unmanaged fields by replacing only adapter-owned keys on adapter-owned entries
- generated manifests should prefer normalized relative paths for portability and safer stale-output cleanup
- source resolution should understand nested plugin repository layouts instead of assuming a single flat canonical tree

### `snowfort-ai/config`

Observed patterns from code and docs:

- a centralized `CoreService` keeps an in-memory detected-state snapshot for all registered engines and emits state changes to consumers
- adapters follow a small `EngineAdapter` contract with `detect`, `read`, `validate`, `write`, and `getConfigPath`
- patch application validates before write, creates backups, writes, and then refreshes cached state
- deep merge is customized so fields like `mcpServers` are replaced rather than recursively merged when that is safer for removal semantics
- timestamped backup records include the original path and serialized data for restore operations
- runtime state caches include detected or not-detected flags and last-modified timestamps, which is useful for diagnostics and repeated inspection
- current implementation is weaker than its README claim of atomic writes: the base adapter writes files directly, and strict validation is currently bypassed in code

Architectural consequence for `integrationctl`:

- a normalized inspected-state cache is useful for `doctor`, future UIs, and repeated plan operations
- adapters should expose a compact read or validate or write contract internally, but lifecycle orchestration still needs stronger ownership and activation semantics above that layer
- merge strategy must be field-aware; some config maps need replacement semantics instead of recursive merge
- backup services are useful infrastructure, but backup alone does not replace journaled lifecycle state
- documentation claims and actual mutation safety should be verified against code before we promote a pattern into a design rule

### `agnostic-ai`

Observed patterns from code and docs:

- the repo uses a canonical `.ai/` directory as the committed source of truth and treats IDE-specific outputs as generated artifacts
- IDE setup is primarily provisioning-oriented through symlinks and templated local config rather than full lifecycle management
- MCP projection is strategy-based, with target templates declaring either whole-file overwrite or merge-under-key behavior
- update flow re-installs plugins and refreshes IDE configuration from the canonical source after checking git state
- update behavior is effectively a repository refresh plus recopy of selected plugin directories and rerun of migrations, not a per-delivery reconcile engine
- MCP merge strategy for some targets writes the whole managed key directly, and the overwrite strategy replaces the whole target file
- convenience wrappers fetch and run update logic remotely, which makes UX simple but further separates the wrapper from durable lifecycle state

Architectural consequence for `integrationctl`:

- keeping one canonical authored source with generated native outputs is a strong pattern and aligns with our authored-vs-materialized separation
- strategy-based projection is useful, but whole-file overwrite is too blunt for a shared lifecycle engine unless the target file is fully adapter-owned
- provisioning UX can be very simple on top of a stronger control plane, but symlink or template convenience should remain above lifecycle state and verification
- recopy-based updates are acceptable for canonical source trees, but they do not replace per-target ownership tracking, verification, or repair semantics

### `task-master-ai`

Confirmed product patterns from README and docs:

- one-click Cursor installation is exposed as a Cursor deeplink with placeholder env values
- Claude quick install is documented as `claude mcp add ...`
- manual MCP config remains as a fallback for editors that do not support the deeplink path
- product docs distinguish between MCP-client env config and local `.env` usage
- the installation guide treats API-key presence as a post-install verification concern, not as proof that install itself succeeded

Architectural consequence for `integrationctl`:

- "one command install" UX may be implemented as thin wrappers above the core control plane
- deeplink generation belongs in a distribution or UX layer, not in the lifecycle engine itself
- install docs should always keep a manual fallback path next to convenience install paths
- shared config projection should distinguish secret-bearing env from non-secret runtime flags
- install success, auth readiness, and functional verification must remain separate result dimensions

### `agent-rules-sync`

Observed patterns from code and docs:

- component-level direction policy is explicit: `bidirectional`, `push`, `pull`
- settings and hooks are projected as a portable subset instead of copied raw
- machine-specific settings and absolute-path rules are stripped during projection
- watch or daemon mode and one-shot sync mode are separate entrypoints over the same sync concerns
- timestamped backups are created before overwriting synced content
- plugin-managed skills are intentionally excluded from general skill sync
- hook commands referencing machine-local script paths are rewritten to repo-relative paths, and the referenced scripts are copied into the repo so the projected config remains runnable

Architectural consequence for `integrationctl`:

- future workspace `sync` should treat direction policy as configuration, not hidden behavior
- projection of portable config must be explicit and lossy-by-design when the source contains machine-local values
- backup and repair policy should be reusable across one-shot and long-running sync entrypoints
- the control plane must not mutate or sync vendor-managed plugin payloads as if they were user-managed skills or rules
- portable projection may require rewriting referenced paths and copying dependent helper artifacts, not just stripping forbidden keys

## Concrete implementation rules extracted from external code

These are not vendor facts. They are engineering rules we should adopt because multiple external implementations converge on them.

- preserve unmanaged fields when patching shared native config; replace only adapter-managed keys on adapter-owned entries
- keep imported-source tracking separate from generated-output manifests and separate again from installed or activated lifecycle state
- normalize generated-manifest paths to relative workspace or home paths when possible so stale-file cleanup survives machine moves
- implement source discovery beyond one happy-path tree; support local paths, git URLs, GitHub slugs, and nested plugin roots discovered from marketplace metadata
- make merge strategy field-aware; some maps need replacement semantics for safe removal, while others can be deep-merged
- implement safe mutation as backup -> validate -> atomic write -> re-validate -> rollback
- allow atomic-write fallback behavior for filesystem edge cases instead of assuming rename is always enough
- treat portable projection as a real transformation step: strip machine-local values, rewrite embedded paths, and copy dependent helper artifacts when needed
- keep bootstrap and provisioning wrappers thin; they should call stable lifecycle use cases instead of carrying a second installer implementation

### `universal-plugins-for-ai-agents`

Observed patterns from repo layout:

- authored source of truth now lives under `plugin/`
- generated native artifacts are committed at plugin root
- one shared `.mcp.json` is referenced by both Claude and Codex package manifests
- generated outputs differ by target, but the authored source remains unified
- the same shared MCP declaration is also projected into Gemini and OpenCode native config shapes

Architectural consequence for `integrationctl`:

- installation logic should consume normalized authored metadata and target-specific deliveries, not treat generated outputs as if they were one universal native package
- shared source artifacts may feed multiple target adapters, but ownership and lifecycle still remain target-specific
- generator and installer must remain separate responsibilities even when they operate on the same normalized manifest
- target-native manifests should be treated as materialized deliveries, not as the canonical authored input
- normalized shared server definitions may be reused across deliveries, but per-target projection rules still belong to the target adapter layer

### `mcp-config-manager`

Observed patterns from repo and examples:

- cross-client MCP config editing is centered around direct file mutation and backup creation
- disabled servers are stored out of line instead of being fully removed from memory
- server syncing across clients is treated mainly as config-shape translation, not as lifecycle management

Architectural consequence for `integrationctl`:

- simple config-manager tools are useful for editing and backup ergonomics, but they are not a substitute for install state, activation state, and ownership tracking
- disabled or detached state can be represented explicitly instead of deleting all remembered delivery metadata
- our control plane should go beyond config copying by preserving lifecycle state, evidence, and verification

### `claude-notifications-go`

Observed operational patterns:

- bootstrap checks prerequisites early
- version detection happens before mutation
- update and repair paths are idempotent
- reinstall is a fallback, not the first step
- update is followed by explicit version verification and recovery reinstall when the expected version was not activated
- compatibility shims are used to avoid breaking already-running sessions that still hold old cached paths

Architectural consequence for `integrationctl`:

- `doctor`, `repair`, and bootstrap wrappers should share the same lifecycle engine
- prefer detect, validate, and repair before reinstall
- bootstrap scripts should be treated as wrappers over stable use cases, never as a second hidden installer implementation
- upgrade apply should support post-activation verification and recovery fallback, not assume that a successful native command means the new version is actually live

## Target architecture

### Layering

The intended architecture is:

1. Domain
2. Application use cases
3. Ports
4. Adapters
5. Composition roots

This should match the style already used by `install/plugininstall`.

### Proposed code layout

Illustrative, not frozen:

```text
install/integrationctl/
  domain/
  ports/
  usecase/
  adapters/
    fs/
    git/
    github/
    claude/
    codex/
    gemini/
    cursor/
    opencode/
  integrationctl.go
```

CLI composition remains outside the module:

- `cli/internal/app`
- `cli/cmd/plugin-kit-ai/...`

Rule:

- the CLI must call a facade and must not wire adapter graphs directly

### Module extraction and repository strategy

Recommendation:

- build `integrationctl` as a separate Go module inside this repository first

Rationale:

- the lifecycle engine is still tightly coupled to `plugin.yaml` normalization, delivery generation, evidence policy, and compatibility rules in this repository
- splitting into a second repository now would create version-skew risk between manifest evolution and installer behavior
- keeping the module inside the monorepo preserves clean boundaries without paying early release-management overhead

Ownership split:

- `install/integrationctl` owns lifecycle domain types, use cases, ports, adapters, state, lock files, journal, evidence registry, and target inspection or mutation logic
- root `plugin-kit-ai` packages own authored manifest validation, normalization, target artifact generation, publishing workflows, and top-level CLI UX composition
- bootstrap scripts and one-line install wrappers must call stable `integrationctl` use cases instead of reimplementing lifecycle logic

Boundary rules:

- `integrationctl` may consume normalized lifecycle inputs from the root project, but it must not depend on generator internals or publishing internals
- generated native artifacts are inputs to target adapters only after they have been materialized as deliveries
- the module must expose a stable facade so the CLI and any future wrappers do not reach into adapters directly

Extraction criteria for a future separate repository:

- facade API is stable across at least two release cycles
- another repository or binary genuinely needs the module as a library
- manifest and delivery schemas stop changing rapidly
- release cadence for installer logic diverges materially from the rest of `plugin-kit-ai`

Until those criteria are met:

- keep the module in-repo and version it with the rest of `plugin-kit-ai`

## SOLID and clean-architecture rules

### Single Responsibility

- Domain types describe lifecycle concepts only
- Use cases orchestrate workflows only
- Adapters implement one external concern each
- CLI parses flags and prints results only

### Open/Closed

- adding a new target must require adding a new adapter and registration, not editing core install and update orchestration logic

### Liskov Substitution

- every target adapter must satisfy the same behavioral contract for planning, applying, status inspection, and removal
- adapters may return "unsupported for this source" or "unsupported on this machine", but they must do so through the same result model

### Interface Segregation

- do not define one giant `PlatformManager` interface
- keep ports narrow and use-case focused

### Dependency Inversion

- use cases depend on ports
- adapters depend on domain and port contracts
- composition roots choose concrete adapters

### DRY

- shared lifecycle orchestration must live in use cases, not be duplicated across target adapters
- state persistence, journaling, evidence lookup, locking, and reporting should each have one reusable implementation path unless a target constraint proves otherwise
- adapters should implement only genuinely target-specific translation, inspection, activation, and mutation logic
- bootstrap wrappers, CLI commands, and future automation entrypoints must reuse the same facade and use cases instead of forking installer logic

## Domain model

### Core entities

#### `IntegrationRef`

Describes what the user asked to install.

Examples:

- GitHub repo
- git URL
- local path
- release artifact URL
- marketplace source reference

#### `IntegrationManifest`

Normalized model resolved from source artifacts.

It should contain only what the control plane needs:

- identity
- version
- compatible targets
- delivery definitions
- release metadata needed for updates
- migration hints such as replacement source

This is not a replacement for `plugin.yaml`. It is a runtime-normalized lifecycle view.

#### `Delivery`

A target-specific installable unit.

Examples:

- `claude-marketplace-plugin`
- `codex-marketplace-plugin`
- `gemini-extension`
- `cursor-mcp`
- `opencode-plugin`

Each delivery declares:

- target family
- delivery kind
- source material needed
- capability surface exposed by the native target, such as MCP tools, prompts, roots, elicitation, commands, skills, hooks, or agents
- required native capabilities
- update capability
- remove capability
- scope capability

Rule:

- the control plane must not flatten every MCP-backed delivery into "just a list of tools"
- delivery metadata should preserve the capability surface because different agents expose and govern those capabilities differently

#### `InstallPolicy`

User intent that survives beyond a single command.

Fields should include:

- `scope`
- `auto_update`
- `adopt_new_targets`
- `allow_prerelease`
- `target_selection`
- `channel_preference`
- `repair_on_mismatch`

#### `InstallationRecord`

Persistent state for one installed integration.

Should capture:

- integration identity
- original source
- resolved version
- installed targets
- adapter-specific install metadata
- policy
- timestamps
- health state

#### `ReconcilePlan`

Desired versus actual diff.

This is the most important domain object after `InstallationRecord`.

It should answer:

- what is already correct
- what needs to be installed
- what needs to be updated
- what needs repair
- what new targets are now available
- what should be removed

### Value objects

- `IntegrationID`
- `Version`
- `TargetID`
- `DeliveryKind`
- `InstallScope`
- `RequestedSourceRef`
- `ResolvedSourceRef`
- `SourceDigest`
- `ManifestDigest`
- `HealthStatus`
- `LockOwner`

### Domain errors

Use explicit error families, not generic text errors:

- source resolution error
- manifest resolution error
- unsupported target error
- incompatible machine error
- state conflict error
- lock acquisition error
- install apply error
- update apply error
- repair apply error
- removal error

## Ports

### `SourceResolverPort`

Resolves `IntegrationRef` into fetchable source material.

Responsibilities:

- parse source input
- fetch metadata
- resolve default branch or release
- normalize repository references
- resolve immutable source identity such as git commit, release asset URL, or local digest
- surface whether the source was resolved from a floating ref or a pinned ref

### `ManifestLoaderPort`

Loads and normalizes manifest and delivery metadata from source material.

Responsibilities:

- inspect repo or archive
- detect supported deliveries
- normalize metadata into `IntegrationManifest`
- compute a stable manifest digest for drift and evidence tracking

### `TargetAdapterPort`

Primary vendor contract.

Each adapter should implement methods like:

- `Capabilities`
- `Inspect`
- `PlanInstall`
- `ApplyInstall`
- `PlanUpdate`
- `ApplyUpdate`
- `PlanRemove`
- `ApplyRemove`
- `Repair`

Important:

- planning and apply must be separate methods
- capability probing must be non-mutating
- inspect must return enough native metadata for drift detection and repair
- adapters must declare capability flags up front so use cases do not infer support from ad hoc behavior

Recommended capability fields:

- `install_mode`: `native_cli`, `config_projection`, `hybrid`
- `supports_native_update`
- `supports_native_remove`
- `supports_link_mode`
- `supports_auto_update_policy`
- `supports_scope_user`
- `supports_scope_project`
- `supports_scope_local`
- `supports_repair`
- `requires_restart`
- `requires_reload`
- `may_trigger_interactive_auth`
- `supported_source_kinds`

Recommended inspect fields:

- `owned_native_objects`
- `observed_native_objects`
- `interactive_auth_state`
- `config_precedence_context`
- `settings_files`
- `environment_restrictions`
- `source_access_state`

### `InstallationStatePort`

Persistent local state contract.

Responsibilities:

- read state
- write state atomically
- query installed records
- compare versions
- record adapter metadata
- persist operation journals for crash recovery

### `LockPort`

Coordinates safe mutation.

Responsibilities:

- acquire user-level lock
- optionally acquire workspace-level lock
- avoid concurrent state and adapter mutation

### `OperationJournalPort`

Coordinates crash-safe lifecycle execution.

Responsibilities:

- create per-operation journals before mutation starts
- append adapter checkpoints as mutation progresses
- mark operations committed, rolled back, or degraded
- expose unfinished operations for `repair` and `doctor-integrations`

### `FileSystemPort`

Needed for atomic writes, temp dirs, and safe replacements.

### `ProcessRunnerPort`

Needed where adapters call vendor CLIs.

### `ClockPort`

Needed for deterministic tests.

### `VersionCheckerPort`

Resolves whether a newer version exists and whether migration metadata applies.

## Adapter model

### Claude adapter

Responsibilities:

- native marketplace add and update flows
- plugin install, update, uninstall
- scope-aware install planning
- inspect installed plugin state
- repair marketplace or plugin mismatch

Preferred behavior:

- use Claude-native commands where documented
- store enough adapter metadata to recover from partial installs

### Codex adapter

Responsibilities:

- materialize or update marketplace entries
- manage source bundle location if needed
- refresh cached install state through supported flows
- inspect installed plugin cache and marketplace state
- reconcile differences between source bundle, marketplace entry, and installed cache

Codex-specific rule:

- because native auto-update semantics are weaker than Gemini and Claude, this adapter owns more reconcile logic directly
- because the current docs do not clearly document a standalone non-interactive install command, this adapter must separate `prepare activation` from `native activation` and surface a manual activation step when required by the documented install surface

### Gemini adapter

Responsibilities:

- `extensions install`
- `extensions update`
- `extensions update --all` where useful
- `--auto-update` policy mapping
- native redirect metadata handling
- inspect extension installation state

Gemini-specific rule:

- prefer native extension lifecycle whenever available instead of emulating update manually

### Cursor adapter

Responsibilities:

- register or update MCP integration using supported native surfaces
- inspect `.cursor/mcp.json`, global config, or CLI-managed state
- reconcile named MCP entries safely
- support one-click or deeplink generation as an optional UX surface above the adapter

Cursor-specific rule:

- treat this as installable integration management, not package-bundle management

### OpenCode adapter

Responsibilities:

- manage local plugin projection or npm/config projection
- inspect current OpenCode config and plugin directories
- reconcile package refs and plugin refs

### Adapter result contract

All adapters must return normalized result objects:

- `AdapterPlan`
- `AdapterApplyResult`
- `AdapterInspectResult`

Each result should include:

- action summary
- paths touched
- native commands run
- native objects owned or observed
- restart required
- warnings
- recoverability
- degraded state marker
- manual steps if any
- evidence class for non-obvious behavior

Important rule:

- config-based adapters must distinguish `observed_native_objects` from `owned_native_objects`
- remove and update flows may mutate only `owned_native_objects`

## Planning before mutation

Every mutating workflow should be split into:

1. resolve
2. inspect
3. plan
4. validate
5. apply
6. persist state

Never write state first and mutate adapters second.

Never mutate multiple targets without a plan object that can be logged and tested.

Every apply operation should also create an operation journal entry before the first mutation.

## Documented mutation surfaces

This section constrains implementation to vendor-documented surfaces.

| Target | Documented mutation surfaces | Non-interactive mutation confidence | Architectural consequence |
|-------|----------|----------|----------|
| Claude | `/plugin marketplace add`, `/plugin marketplace update`, `/plugin marketplace remove`, `/plugin install`, `/plugin uninstall`, `/plugin enable`, `/plugin disable`, `/reload-plugins`, native settings files | high | adapter can rely on native commands first |
| Codex | `/plugins` browser, marketplace files, plugin manifests, `~/.codex/config.toml`, documented cache layout | medium for preparation, low for full non-interactive activation | adapter may automate marketplace preparation and state inspection, but must not invent undocumented activation commands |
| Gemini | `gemini extensions install`, `update`, `update --all`, `uninstall`, `enable`, `disable`, `config`, `link` | high | adapter can rely on native commands first |
| Cursor | `.cursor/mcp.json`, `~/.cursor/mcp.json`, one-click install, `Add to Cursor`, `cursor-agent mcp list`, `cursor-agent mcp login`, `cursor-agent mcp disable`, documented FAQ refresh guidance by server type | medium | adapter should treat config projection and login state as the stable mutation surface, and treat refresh as server-kind-specific guidance rather than a universal update command |
| OpenCode | `opencode.json` or `opencode.jsonc`, `.opencode/plugins`, `~/.config/opencode/plugins`, startup loading via Bun | medium | adapter should treat file and config projection as the stable mutation surface |

Rule:

- if a vendor documents a native non-interactive lifecycle command, prefer it
- if a vendor documents config or marketplace files but not a non-interactive activation command, automate only those documented surfaces and surface any remaining activation step explicitly

## Environment restriction taxonomy

The control plane should normalize environment and policy blockers into explicit categories instead of leaking vendor-specific prose into core logic.

Recommended categories:

- `managed_policy_block`
- `trust_required`
- `source_auth_required`
- `native_auth_required`
- `native_activation_required`
- `restart_required`
- `reload_required`
- `new_thread_required`
- `source_tool_missing`
- `source_shape_unsupported`
- `read_only_native_layer`
- `volatile_override_layer`

Examples:

- Claude `strictKnownMarketplaces` and seed-managed marketplaces map to `managed_policy_block` or `read_only_native_layer`
- Gemini untrusted folders map to `trust_required`
- Gemini `security.blockGitExtensions` and `security.allowedExtensions` mismatches map to `managed_policy_block`
- Cursor servers that require explicit login map to `native_auth_required`
- Codex plugin-browser completion or new-thread guidance maps to `native_activation_required` or `new_thread_required`
- OpenCode remote or managed config layers map to `read_only_native_layer`
- Gemini environment-variable and command-line overrides map to `volatile_override_layer`

## Persistent state design

### User-level state

Recommended location:

- `~/.plugin-kit-ai/state.json`

This is the system of record for installed integrations managed by the control plane.

Recommended shape:

```json
{
  "schema_version": 1,
  "installations": [
    {
      "integration_id": "context7",
      "source": {
        "kind": "github",
        "value": "777genius/universal-plugins-for-ai-agents//plugins/context7"
      },
      "requested_source_ref": "github:777genius/universal-plugins-for-ai-agents//plugins/context7",
      "resolved_source_ref": {
        "kind": "git_commit",
        "value": "https://github.com/777genius/universal-plugins-for-ai-agents@8f0f1d8"
      },
      "resolved_version": "1.4.0",
      "source_digest": "sha256:...",
      "manifest_digest": "sha256:...",
      "policy": {
        "auto_update": true,
        "adopt_new_targets": "auto",
        "allow_prerelease": false
      },
      "targets": {
        "claude": {
          "delivery_kind": "claude-marketplace-plugin",
          "state": "installed",
          "native_ref": "context7@portable-mcp"
        },
        "gemini": {
          "delivery_kind": "gemini-extension",
          "state": "installed",
          "native_ref": "context7"
        },
        "cursor": {
          "delivery_kind": "cursor-mcp",
          "state": "installed",
          "native_ref": "context7",
          "owned_native_objects": [
            {
              "kind": "cursor_mcp_entry",
              "path": "~/.cursor/mcp.json",
              "name": "context7"
            }
          ]
        }
      },
      "last_checked_at": "2026-04-08T10:00:00Z",
      "last_updated_at": "2026-04-08T10:00:00Z"
    }
  ]
}
```

### Workspace lock file

Recommended location:

- `<repo>/.plugin-kit-ai.lock`

Purpose:

- record desired integrations for a workspace
- allow `plugin-kit-ai sync`
- make team installs reproducible

This is optional but strongly recommended.

Recommended shape:

```yaml
api_version: v1
integrations:
  - source: github:777genius/universal-plugins-for-ai-agents//plugins/context7
    version: 1.4.0
    targets:
      - claude
      - codex
      - gemini
      - cursor
    policy:
      auto_update: true
      adopt_new_targets: manual
```

Rule:

- user-level state answers "what is installed here now"
- workspace lock answers "what this workspace expects"

## Workspace declaration vs local overrides

The architecture must keep three different data classes separate:

1. `workspace declaration` - repo-owned desired integrations and shared defaults
2. `user control-plane state` - what this user actually installed, with resolved refs and health
3. `local mutable overrides` - user-specific values that must not be committed into the workspace declaration

Examples of local mutable overrides:

- Gemini extension settings stored in extension-local `.env`
- Cursor or other MCP auth completion state
- local secrets, tokens, and secret-env mappings
- user-specific approvals, trust decisions, and temporary disables
- machine-specific absolute paths

Rules:

- workspace locks must never store secrets, approval decisions, auth tokens, or machine-specific absolute paths
- adapters may read local mutable overrides when planning, but they must not rewrite them unless the vendor docs define a dedicated settings command or dedicated owned settings object
- if local mutable overrides are needed for a working install, the plan should record them as follow-up requirements instead of polluting the shared lock file
- config-driven targets should prefer minimum owned subtrees plus local overlays over full-file replacement

Architectural reason:

- this keeps team-declared desired state reproducible while preserving user-specific auth and secret material outside repo-managed artifacts

### Operation journal

Recommended location:

- `~/.plugin-kit-ai/operations/<operation-id>.json`

Purpose:

- survive process interruption between native mutation and state commit
- support deterministic repair and rollback decisions
- provide support evidence when a multi-target apply only partially succeeded

Recommended minimal shape:

```json
{
  "operation_id": "op_2026_04_08_001",
  "type": "update",
  "integration_id": "context7",
  "status": "in_progress",
  "started_at": "2026-04-08T10:00:00Z",
  "steps": [
    {
      "target": "claude",
      "action": "update_version",
      "status": "applied",
      "owned_native_objects": [
        "context7@portable-mcp"
      ]
    },
    {
      "target": "cursor",
      "action": "adopt_new_target",
      "status": "pending"
    }
  ]
}
```

## State and lock semantics

- user-level state must be written atomically
- lock file writes must also be atomic
- operation journal writes must also be atomic
- plan generation must not require a lock
- apply workflows must hold the mutation lock
- lock acquisition timeouts must be explicit and user-visible
- state writes must happen after successful adapter mutation, never before
- degraded installs must still persist enough metadata for later repair
- unfinished journals must be inspected before starting another mutation for the same integration

## Lifecycle use cases

### `AddIntegration`

User command examples:

```bash
plugin-kit-ai add <source>
plugin-kit-ai add <source> --targets all
plugin-kit-ai add <source> --targets claude,gemini,cursor
```

Responsibilities:

- resolve source
- load manifest
- detect locally available target environments
- choose compatible deliveries
- create install plan
- apply target installs
- persist user state
- optionally update workspace lock

Special rule:

- if some deliveries are unsupported on the current machine or in the selected scope, the plan must explain why they were skipped
- if a target can be fully prepared through documented surfaces but still needs a documented native activation step, the plan must record an activation boundary instead of pretending the install is complete

### `UpdateIntegration`

User command examples:

```bash
plugin-kit-ai update context7
plugin-kit-ai update --all
```

Responsibilities:

- inspect installed record
- resolve latest allowed version
- compare versions
- detect new deliveries in the newer manifest
- create reconcile plan
- apply updates target by target
- adopt new targets if policy allows
- persist updated state

Special rule:

- update must handle both version movement and target-set movement

### `RemoveIntegration`

Responsibilities:

- inspect state
- build removal plan
- remove target installs safely
- remove state only after successful adapter removal or explicit confirmed partial cleanup

### `RepairIntegration`

Responsibilities:

- inspect native state versus stored state
- detect drift, corruption, or missing native artifacts
- repair without unnecessarily reinstalling everything
- detect `auth_pending` versus broken states and avoid destructive repair while the user still needs to complete native auth
- prefer vendor-documented isolation steps, disable flows, or owned-entry detachment before cache clearing, global cleanup, or reinstall

### `SyncWorkspaceIntegrations`

Responsibilities:

- load workspace lock
- compare desired integrations against current user state
- plan installs, updates, or removals
- apply with clear diff output

## Reconcile semantics

This is the most important workflow in the whole design.

Update is not only "move from version X to version Y".

Update must also handle:

- a new target delivery was added
- a target delivery was removed
- the same target changed delivery kind
- migration to a new source was declared
- native install drifted from expected state

Recommended reconcile action classes:

- `noop`
- `install_missing`
- `update_version`
- `adopt_new_target`
- `migrate_source`
- `repair_drift`
- `remove_orphaned_target`
- `await_activation`
- `await_auth_completion`

### Catalog policy vs observed native state

Some targets expose catalog or manifest policy that influences install UX without proving the current local runtime state.

Confirmed examples:

- Codex plugin marketplace entries expose `policy.installation` and `policy.authentication`
- Claude marketplaces can declare defaults such as `enabledPlugins`, but the real installed and enabled state still lives in native scope-specific storage

Rules:

- adapter planning must always distinguish declared catalog policy from observed native state
- state transitions such as `installed`, `disabled`, `activation_pending`, and `auth_pending` must be driven by inspection of native objects, not by catalog defaults alone
- catalog policy may shape the plan, defaults, warnings, and next steps, but it must not be treated as a substitute for inspectable local evidence

### `adopt_new_targets`

Recommended policy values:

- `auto`
- `manual`
- `disabled`

Meaning:

- `auto`: newly added deliveries are installed automatically for compatible local agents
- `manual`: show them in the plan but do not apply automatically
- `disabled`: ignore newly added deliveries

Auto-adoption guardrails:

- do not auto-adopt if the new target requires a broader scope than the current policy allows
- do not auto-adopt if the new target introduces new interactive authentication or secret prompts unless policy explicitly allows it
- do not auto-adopt if the new target changes delivery kind from a lower-risk config projection to a higher-risk executable install without explicit confirmation
- do auto-adopt when the new target is compatible, same trust level, and can be applied through the existing policy safely

This directly solves the product requirement:

- plugin updates can add support for a new agent and have that support adopted automatically when policy allows

## Install flow

Recommended high-level sequence:

1. parse source
2. resolve source material
3. load normalized manifest
4. detect local agents and supported scopes
5. inspect existing install state
6. build plan
7. validate plan
8. acquire lock
9. apply per-target adapters
10. persist state atomically
11. print summary and next steps

Per-target notes:

- Claude: include marketplace add or refresh and reload guidance in the plan
- Codex: include marketplace materialization, any documented activation boundary, and restart or new-thread guidance in the plan
- Gemini: include install mode vs link mode and restart guidance in the plan
- Cursor: include project-vs-global config mutation path in the plan
- OpenCode: include local-vs-npm projection mode in the plan

## Update flow

Recommended high-level sequence:

1. load installation record
2. resolve latest permitted source version
3. load new normalized manifest
4. inspect current native target state
5. build reconcile plan
6. acquire lock
7. apply updates target by target
8. apply newly adopted targets if policy permits
9. persist state atomically
10. emit any restart or manual-auth steps

Per-target notes:

- Claude: use native plugin or marketplace update semantics where documented
- Codex: treat update as directory refresh plus restart for local installs, and do not assume bundled app teardown on plugin-only update or removal
- Gemini: use native extension update semantics, preserve native redirect metadata, and preserve user-provided extension settings
- Cursor: treat update as reconcile of MCP configuration or registered server definition
- OpenCode: treat update as reconcile of file projection or npm package declaration, followed by normal startup loading behavior; if cache corruption is detected, surface cache-rebuild repair guidance explicitly

## Remove flow

Recommended high-level sequence:

1. load installation record
2. inspect native state
3. build removal plan
4. acquire lock
5. remove target installs
6. persist state removal
7. optionally prune empty control-plane data

Per-target notes:

- Claude: removal should use native uninstall semantics and preserve marketplace identity rules
- Codex: removal must treat marketplace entry, source bundle, and cache as separate objects
- Cursor: removal means removing owned MCP entries, not sweeping unrelated config
- OpenCode: removal differs for local plugin files and npm declarations

## Repair flow

Repair should exist as a first-class workflow, not as an afterthought.

Examples of repair cases:

- state says installed but native entry is missing
- native plugin exists but points to stale source
- marketplace updated but installed copy did not refresh
- cache drift or partial deletion
- adapter metadata no longer matches filesystem

Recommended user command:

```bash
plugin-kit-ai repair <name>
```

## Atomicity and rollback

### Atomicity rule

Within one integration operation:

- either all selected target installs succeed and state is updated
- or state remains unchanged and successful partial target mutations are rolled back where possible

### Rollback rule

Rollback must be best-effort but explicit.

If full rollback is impossible:

- mark installation as `degraded`
- record exact recovery steps
- do not silently pretend success
- prefer vendor-documented isolate, disable, or owned-entry detachment steps before cache clearing, broad cleanup, or reinstall

### Why this matters

This is one of the strongest ideas to borrow from `agent-resources`.

Multi-target install without rollback creates long-tail support pain immediately.

## Adapter-specific persistence

Each adapter should be allowed to store adapter metadata inside the `InstallationRecord`.

Examples:

- Claude marketplace name and plugin key
- Codex marketplace root and source path
- Gemini extension name and source ref
- Cursor MCP entry name and scope
- OpenCode plugin or package ref

Rule:

- adapter metadata belongs in state
- it must not leak into unrelated domain entities

## Ownership and projection rules

Config-projecting adapters are the easiest place to introduce data loss. The plan must be strict here.

Rules:

- Cursor and OpenCode adapters must mutate only entries that they can prove they own
- ownership proof comes from deterministic native refs plus state metadata, not from fuzzy matching on display names alone
- if ownership is ambiguous, the adapter must stop and return a manual-resolution plan instead of guessing
- adapters must preserve unrelated keys, ordering where practical, and user-authored comments or formatting when the native format allows it
- when the native format does not support inline metadata, ownership must be tracked in control-plane state and by deterministic entry names or paths

Practical implication:

- never rewrite an entire `mcp.json` or `opencode.json` file just because one owned entry changed
- patch the minimum owned subtree and validate the resulting document before replacing it atomically
- for OpenCode, preserve valid JSONC comments and non-plugin settings because config files are merged and often user-authored

## External auth boundary

Some vendor-native installs cross into interactive authentication or app-connection flows. This must be first-class in the architecture.

Confirmed examples:

- Codex docs explicitly say some plugins ask for authentication during install, while others wait until first use
- Cursor CLI exposes MCP login flows separately from config projection
- Cursor one-click installs can rely on OAuth-backed remote MCP servers
- Claude and Gemini integrations may also bundle MCP or app surfaces that require later setup

Rules:

- install state must distinguish `prepared`, `installed`, `activation_pending`, `auth_pending`, `disabled`, `degraded`, and `removed`
- a target in `auth_pending` is not necessarily broken and should not be auto-repaired destructively
- a target in `activation_pending` is not necessarily broken and should not be auto-repaired destructively
- `update --all` must skip or clearly surface targets that require interactive auth instead of hanging or pretending success
- operation journals must be able to stop in `awaiting_user_activation` or `awaiting_user_auth` and resume later
- trust or policy restrictions such as Gemini untrusted folders must surface as `environment_restrictions`, not as silent no-ops

## Safety rules

- all filesystem writes must use temp file then atomic replace
- all directory replacements must avoid destructive broad deletes
- adapter commands must be non-interactive whenever possible
- no use of destructive global cleanup outside adapter-owned paths
- no silent mutation of user-owned config fields outside the adapter-owned section
- floating sources must be resolved to immutable refs before state commit
- repair must prefer validation and re-attachment before destructive reinstall

## Source access and offline resilience

The lifecycle engine must distinguish source-resolution failures from install-state corruption.

Confirmed vendor facts that shape this:

- Claude plugin marketplace updates can fail in offline environments, and Claude documents a keep-last-known-good mode with `CLAUDE_CODE_PLUGIN_KEEP_MARKETPLACE_ON_FAILURE=1`
- Claude also documents seeding plugin directories for fully offline deployments through `CLAUDE_CODE_PLUGIN_SEED_DIR`
- private plugin sources can require provider-specific credentials for background auto-updates
- Claude relative-path plugin sources fail when the marketplace itself was added as a direct URL instead of a Git-backed source
- Claude npm plugin sources depend on `npm install`
- Gemini installs from GitHub require `git` on the machine
- OpenCode npm plugin installation depends on Bun at startup

Rules:

- update failure due to source access must not immediately be treated as a broken install if the last-known-good native state is still usable
- adapters should prefer retaining last-known-good state over destructive cleanup when the vendor docs support that behavior
- diagnostics should distinguish `source_unreachable`, `auth_missing_for_source`, `tool_missing`, and `native_state_corrupt`
- diagnostics should also distinguish `source_shape_unsupported` when a source layout is invalid for the chosen vendor surface, such as Claude relative paths on URL-based marketplaces
- control-plane repair must not delete a working installed copy only because a refresh source is temporarily unreachable

## Config root discovery rules

For config-driven targets, discovering the correct root is part of correctness, not a convenience detail.

Rules:

- Cursor adapters must inspect effective configuration in documented precedence order and record which layer actually owns each managed entry
- OpenCode adapters must respect documented remote, global, project, `.opencode`, and managed layers, plus upward discovery to the nearest Git directory before planning any mutation
- Claude project-scope mutations must target `.claude/settings.json`, not an invented control-plane side file
- OpenCode adapters must treat file-config precedence and directory-overlay precedence as separate mechanisms:
  - file precedence: remote, global, project, managed files, macOS managed preferences
  - directory overlays: standard `.opencode` loaded after project config and before managed layers
- Gemini adapters must inspect user settings, project settings, system defaults, system settings, and documented security settings before planning mutation, and must treat environment variables and CLI arguments as volatile overlays rather than persistent mutation targets
- Gemini adapters must resolve trust state from the highest-priority documented source available:
  - IDE trust signal when IDE integration is active
  - otherwise `~/.gemini/trustedFolders.json`
- if effective config cannot be resolved unambiguously, the adapter must stop with a manual-resolution plan instead of guessing

## Scope model

Not every target has the same scope language.

The control plane should normalize a portable scope enum such as:

- `user`
- `project`
- `local`

Adapters then translate to native behavior.

Examples:

- Claude can map directly to its native scope model
- Cursor maps `project` to `.cursor/mcp.json` and `user` to `~/.cursor/mcp.json`
- OpenCode maps `project` to project config and plugin directories and `user` to global config and plugin directories
- Gemini supports `user` and `workspace` scopes for enable and disable operations; adapters should surface a clear warning when the user asks for a scope the documented command surface does not support
- Cursor inspect and planning should respect the documented project and global configuration context seen by `cursor-agent`
- OpenCode planning should respect merged config behavior instead of assuming one file fully replaces another
- Claude project scope should map to `.claude/settings.json` because that is the documented native storage for project-scoped plugin installs

Rule:

- normalized scope is user intent
- adapter translation is vendor detail

## Protection class model

Scope alone is not enough. Some vendors also have layers that are user-mutable, repo-mutable, remotely supplied, or admin-managed.

Recommended protection classes:

- `user_mutable`
- `workspace_mutable`
- `remote_default`
- `admin_managed`

Examples:

- Claude managed scope is `admin_managed`
- Claude project scope in `.claude/settings.json` is `workspace_mutable`
- Gemini trusted-folder restrictions can temporarily make a normally mutable workspace behave as effectively blocked
- Gemini system settings are `admin_managed`, while environment-variable and CLI layers are volatile and non-owned
- OpenCode remote configuration from `.well-known/opencode` is `remote_default`
- OpenCode managed system configuration is `admin_managed`
- OpenCode inline config and environment-selected custom config are volatile selection layers, not repo-owned state

Rule:

- scope chooses where the user wants an integration applied
- protection class determines whether the control plane is allowed to mutate that layer at all

## Target strategy matrix

This matrix turns the architecture into operational guidance.

| Target | Install strategy | Update strategy | Remove strategy | Restart or reload strategy |
|-------|----------|----------|----------|----------|
| Claude | native marketplace add plus plugin install with scope-aware state in native settings | native marketplace or plugin update plus reload guidance | native uninstall plus marketplace-aware cleanup when owned; removing an owned marketplace also removes its owned plugins | emit `/reload-plugins` guidance after updates |
| Codex | materialize marketplace entry plus bundle source and use the documented plugin-browser activation surface when native non-interactive activation is unavailable | refresh source bundle, reconcile marketplace, preserve cache semantics, require restart | remove owned marketplace entry and reconcile owned source material; never sweep unrelated cache blindly or claim bundled app teardown | emit restart guidance after local changes and new-thread guidance after install |
| Gemini | native extension install or link | native extension update and migrated-source handling | native extension remove or disable semantics when applicable | emit restart guidance after management operations |
| Cursor | reconcile owned MCP entry into project or global config | replace or reconcile owned MCP entry definition, and use server-kind-specific refresh guidance where docs describe it | remove owned MCP entry only; do not confuse approval toggles with uninstall | emit config refresh guidance and, when adapter inspection shows session staleness or local file changes, restart guidance |
| OpenCode | local file projection or npm plugin declaration projection | reconcile file projection or package declaration and rely on startup loading | remove owned local files or owned npm declaration only | emit startup or reload guidance; if cache corruption is detected, emit cache-rebuild repair guidance |

Project policy:

- target strategy differences are expected
- the universal command surface must not erase them
- plans and apply results must make them visible

## Version and channel policy

The control plane needs a small version policy model:

- follow latest stable
- pin exact version
- allow prerelease
- migrate to replacement source if declared

This should support:

- regular updates
- repository rename or migration
- deprecation with guided migration
- vendor-native release-channel mapping where the vendor docs support separate pinned channels

Resolution rules:

- if the user installed from a floating branch or default branch, persist both the requested ref and the resolved immutable ref
- if the user installed from a GitHub Release or release asset, persist the exact release tag and asset identity
- if the source exposes Gemini redirect metadata, preserve both old and new source lineage in state
- version comparison must operate on resolved artifacts, not only on user-entered source strings
- for Claude, model marketplace pinning and plugin-source pinning independently because they are distinct documented axes

## Bootstrap scripts

Bootstrap scripts like:

```bash
curl -fsSL "https://raw.githubusercontent.com/777genius/claude-notifications-go/main/bin/bootstrap.sh" | bash
```

remain useful, but they should become thin UX wrappers over the control plane where possible.

Recommended long-term direction:

- keep plugin-specific bootstrap scripts for easy discovery and onboarding
- have those scripts call `plugin-kit-ai add ...` or `plugin-kit-ai update ...` when the control plane is present
- preserve direct fallback behavior only where vendor-native setup truly requires it

Operational rule:

- bootstrap scripts should remain idempotent wrappers with preflight checks, repair paths, and restart guidance
- they must not become a second hidden control plane with separate lifecycle state

Recommended wrapper behavior:

1. detect whether `plugin-kit-ai` is installed
2. if present, delegate to `plugin-kit-ai add` or `plugin-kit-ai update`
3. if absent, perform vendor-native fallback install
4. if fallback install succeeds, offer or perform control-plane state reconciliation on the next run

This keeps the one-liner UX while preventing lifecycle logic from forking permanently.

Rule:

- bootstrap is a convenience surface
- the control plane is the real lifecycle system

## CLI surface proposal

Recommended first-class commands:

```bash
plugin-kit-ai add <source>
plugin-kit-ai update [name]
plugin-kit-ai update --all
plugin-kit-ai remove <name>
plugin-kit-ai repair <name>
plugin-kit-ai sync
plugin-kit-ai list
plugin-kit-ai doctor-integrations
```

Possible later commands:

```bash
plugin-kit-ai inspect-install <name>
plugin-kit-ai adopt-targets <name>
plugin-kit-ai prune
```

## Native command reference

This is an implementation aid, not a replacement for vendor docs.

Claude documented commands:

- `/plugin marketplace add <source>`
- `/plugin marketplace update <marketplace-name>`
- `/plugin marketplace remove <marketplace-name>`
- `/plugin install <plugin>@<marketplace>`
- `/plugin uninstall <plugin>@<marketplace>`
- `/plugin enable <plugin>@<marketplace>`
- `/plugin disable <plugin>@<marketplace>`
- `/reload-plugins`
- `claude plugin install ... --scope ...` and `claude plugin uninstall ... --scope ...` are also documented CLI forms

Claude documented source and versioning surfaces:

- marketplace sources and plugin sources are distinct
- marketplace sources support `ref`
- plugin sources support `ref` and exact `sha`
- relative-path plugin sources require a Git-backed marketplace source

Codex documented user surfaces:

- `codex` then `/plugins` to open the plugin browser
- uninstall from the plugin browser
- disable by editing `~/.codex/config.toml` and restarting Codex
- local or personal marketplace files under `.agents/plugins/marketplace.json`

Gemini documented commands:

- `gemini extensions install <source> [--ref <ref>] [--auto-update] [--pre-release] [--consent] [--skip-settings]`
- `gemini extensions update <name>`
- `gemini extensions update --all`
- `gemini extensions uninstall <name...>`
- `gemini extensions enable <name> [--scope <scope>]`
- `gemini extensions disable <name> [--scope <scope>]`
- `gemini extensions config <name> [setting] [--scope <scope>]`
- `gemini extensions link <path>`

Cursor documented user surfaces:

- `.cursor/mcp.json`
- `~/.cursor/mcp.json`
- one-click install from the MCP collection
- `Add to Cursor` deeplinks
- `cursor-agent mcp list`
- `cursor-agent mcp login <identifier>`
- `cursor-agent mcp disable <identifier>`

OpenCode documented user surfaces:

- `opencode.json` or `opencode.jsonc`
- `.opencode/plugins/`
- `~/.config/opencode/plugins/`
- startup loading and Bun-based npm installation
- precedence order: remote, global, custom config, project, `.opencode`, inline config, managed files, macOS managed preferences

## Diagnostics and observability

The control plane should emit machine-readable reports for CI and support tooling.

Recommended later contracts:

- `plugin-kit-ai/install-report`
- `plugin-kit-ai/update-report`
- `plugin-kit-ai/sync-report`
- `plugin-kit-ai/repair-report`

Human-readable output should always include:

- source
- resolved version
- target actions
- warnings
- restart requirements
- partial recovery instructions if needed

Recommended human output fields:

- `target`
- `delivery_kind`
- `action`
- `status`
- `restart_or_reload_required`
- `evidence_class`
- `manual_step`

Recommended machine-readable output fields:

- `source_ref`
- `resolved_version`
- `target`
- `adapter`
- `delivery_kind`
- `action_class`
- `native_object_refs`
- `restart_required`
- `reload_required`
- `evidence_class`
- `degraded`
- `manual_steps`
- `warnings`
- `requested_source_ref`
- `resolved_source_ref`
- `source_digest`
- `manifest_digest`
- `owned_native_objects`
- `observed_native_objects`
- `operation_id`
- `activation_state`
- `interactive_auth_state`
- `config_precedence_context`
- `environment_restrictions`
- `protection_class`
- `source_access_state`
- `new_thread_required`
- `volatile_override_detected`
- `trust_resolution_source`

## Testing strategy

### Unit tests

- domain policies
- plan and reconcile diffing
- version policy logic
- state read and write logic

### Adapter contract tests

- every adapter must pass the same contract suite for plan, apply, inspect, remove, and repair behavior
- contract tests should include restart-required and partial-failure cases where the target model supports them

### Filesystem tests

- atomic writes
- rollback behavior
- lock behavior
- partial failure behavior

### Fixture-driven tests

- sample source repos
- sample manifests
- sample upgrade paths with newly added targets

### Live optional tests

- one live suite per adapter
- opt-in by environment variables
- used only for confidence refresh and release evidence

Recommended live evidence priority:

1. Gemini
2. Claude
3. Cursor
4. Codex
5. OpenCode

## Evidence refresh process

This plan should stay strict over time, not just on the day it was written.

Recommended rule set:

- every adapter capability that depends on vendor docs should map to an evidence key
- every evidence key should cite one or more current official URLs
- live tests may increase confidence in an implementation, but they do not upgrade undocumented behavior into a confirmed vendor contract
- if docs or live behavior diverge, the adapter should downgrade to the more conservative mode until the discrepancy is resolved

Examples:

- Claude startup auto-update remains a confirmed capability because the docs state it directly
- Codex local refresh remains a managed-refresh capability because the docs state restart and directory update, but not strong auto-update guarantees
- Cursor update remains config reconciliation because the official docs and search-visible docs document install and CLI inspection clearly, but not a package-style update command
- OpenCode remains a projection-and-startup target because the official docs document startup loading, merged config, Bun install, and cache clearing, but not a dedicated plugin update command

Recommended implementation discipline:

- add an `evidence_key` field to every adapter capability that is not purely internal
- keep a small checked-in registry that maps `evidence_key -> official URLs -> expected claim`
- fail adapter evidence tests when an implementation claims a capability that has no matching evidence key

## Rollout plan

### Phase 1: foundation

- define domain types
- define ports
- define state format
- define planning and reconcile engine
- ship stateful `list` and dry-run planning
- define evidence classification in result objects
- define adapter contract test harness
- define conservative-gap registry tests so undocumented behavior does not silently become "supported"

### Phase 2: first adapters

- Claude adapter
- Gemini adapter
- Cursor adapter

Reason:

- these have the clearest current native install surfaces today
- Gemini gives the strongest native update reference
- Cursor gives the clearest config-projection integration reference

### Phase 3: Codex adapter

- implement Codex marketplace and refresh semantics carefully
- add stronger drift and repair logic
- start with documented marketplace preparation and inspection flows first
- add direct activation automation only if a documented native activation surface is available or newly confirmed

Reason:

- Codex is documented clearly enough to implement, but its update model is the easiest one to over-assume incorrectly

### Phase 4: OpenCode adapter

- implement local and package-aware projection paths

Reason:

- OpenCode load behavior is clear, but it should not be over-generalized into a fake explicit update system

### Phase 5: workspace sync

- introduce `.plugin-kit-ai.lock`
- support team sync and CI verification

### Phase 6: bootstrap integration

- update plugin-specific bootstrap docs and scripts to optionally route through the control plane

## Acceptance criteria

The architecture is good enough to implement when the following are all true:

- adding a new target requires adding an adapter, not rewriting core workflows
- update supports both version change and newly added target adoption
- state and native installs can drift and still be repaired
- multi-target installs support rollback or explicit degraded-state reporting
- targets with documented activation boundaries can be represented honestly as `activation_pending` without corrupting lifecycle state
- the user-facing commands stay small and understandable
- the design does not require pretending Cursor and OpenCode are package-bundle targets
- the design explicitly distinguishes confirmed vendor facts from project policy

## Risks

### Risk: over-generalized abstractions

Mitigation:

- keep adapters explicit
- keep `DeliveryKind` concrete

### Risk: trying to force one native install flow

Mitigation:

- normalize lifecycle intent only
- keep native command and config semantics inside adapters

### Risk: weak state model

Mitigation:

- make state first-class from v1
- make repair first-class from v1

### Risk: partial failure across multiple targets

Mitigation:

- separate planning from apply
- require rollback or degraded-state reporting

### Risk: vendor behavior changes

Mitigation:

- keep adapter boundaries narrow
- invest in opt-in live evidence suites

### Risk: false certainty in undocumented vendor behavior

Mitigation:

- explicitly label inference versus confirmed behavior
- avoid product promises that exceed the docs
- keep conservative fallback behavior in adapters

### Risk: mutating the wrong config or protection layer

Mitigation:

- make config root discovery explicit in inspect results
- separate `scope` from `protection_class`
- refuse mutation when the effective layer is admin-managed, remote-default, or ambiguous

## Recommended next implementation steps

1. Create the new lifecycle module skeleton under `install/integrationctl`.
2. Define domain types and the `TargetAdapterPort`.
3. Define `state.json` and `.plugin-kit-ai.lock` schemas.
4. Implement dry-run planning before any mutating command.
5. Define the evidence registry before adapter implementation starts.
6. Implement Claude and Gemini adapters first.
7. Add reconcile tests for "new target added in later release".
8. Add repair workflows before calling the system production-ready.
9. Update this plan only alongside adapter-level RFCs and matching evidence tests.

## Source appendix

Official vendor docs used to tighten this plan:

- Claude discover and install plugins: <https://code.claude.com/docs/en/discover-plugins>
- Claude plugin marketplaces: <https://code.claude.com/docs/en/plugin-marketplaces>
- Codex plugins overview: <https://developers.openai.com/codex/plugins>
- Codex build plugins: <https://developers.openai.com/codex/plugins/build>
- Gemini extensions overview: <https://geminicli.com/docs/extensions/>
- Gemini extensions reference: <https://geminicli.com/docs/extensions/reference/>
- Gemini release extensions: <https://geminicli.com/docs/extensions/releasing/>
- Gemini trusted folders: <https://geminicli.com/docs/cli/trusted-folders/>
- Gemini configuration: <https://geminicli.com/docs/reference/configuration>
- Cursor MCP docs: <https://docs.cursor.com/context/mcp>
- Cursor CLI MCP docs: <https://docs.cursor.com/cli/mcp>
- Cursor deeplinks docs: <https://docs.cursor.com/deeplinks>
- Cursor MCP directory docs: <https://docs.cursor.com/en/tools/mcp>
- Cursor MCP extension API docs: <https://docs.cursor.com/en/context/mcp-extension-api>
- OpenCode plugins docs: <https://opencode.ai/docs/plugins/>
- OpenCode config docs: <https://opencode.ai/docs/config/>
- OpenCode troubleshooting docs: <https://opencode.ai/docs/troubleshooting/>

Cursor note:

- the Cursor docs are partly harder to fetch non-interactively than the other vendor docs, so this plan intentionally keeps Cursor update semantics conservative and avoids claiming a package-style update contract that the official docs do not clearly document
- the official Cursor MCP docs and official FAQ or help content support server-kind-specific refresh guidance, but still do not justify claiming one universal package-style update command

External implementation references used as architectural examples:

- `agent-resources`: <https://github.com/kasperjunge/agent-resources>
- `amtiYo/agents`: <https://github.com/amtiYo/agents>
- `task-master-ai`: <https://github.com/eyaltoledano/claude-task-master>
- `claude-notifications-go` bootstrap: <https://github.com/777genius/claude-notifications-go/blob/main/bin/bootstrap.sh>
- `universal-plugins-for-ai-agents`: <https://github.com/777genius/universal-plugins-for-ai-agents>

Patterns worth borrowing from those references:

- `agent-resources`: explicit tool registry, path strategy per target, and rollback-aware multi-target sync
- `amtiYo/agents`: repo-declared desired state, separate local override file, stale lock metadata, and minimal managed-file rewrites
- `claude-notifications-go`: idempotent bootstrap, prerequisite checks, version verification after update, and fallback reinstall on mismatch
- `universal-plugins-for-ai-agents`: one authored source generating several vendor-native outputs without pretending those outputs are identical lifecycle lanes

## Final rule

The system should feel simple to the user because the architecture is strict internally, not because it hides real complexity behind brittle shortcuts.

That means:

- simple commands on the outside
- explicit plans and state in the middle
- narrow adapters at the edge
- real vendor semantics preserved underneath
