<img width="2172" height="724" alt="Universal Agent Plugins - install and create plugins across AI agents with one command" src="assets/readme-banner.png" />

[![Required](https://github.com/777genius/universal-agent-plugins/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/777genius/universal-agent-plugins/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/777genius/universal-agent-plugins?label=release)](https://github.com/777genius/universal-agent-plugins/releases)
[![npm](https://img.shields.io/npm/v/universal-agent-plugins?label=npm)](https://www.npmjs.com/package/universal-agent-plugins)
[![Agent Plugins 1.0](https://img.shields.io/badge/Agent%20Plugins-1.0.0-7257FF)](https://agent-plugins.org/specification)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

Install and manage Agent Plugins 1.0 across your AI agents with one CLI.

<img width="423" height="234" alt="image" src="https://github.com/user-attachments/assets/3f8786ea-bb3a-4869-aabe-99b245364b3d" />

## Use plugins

Install, inspect, update, repair, and remove packages with the existing installer.
The [Use guide source](website/source/en/use/index.md) collects this journey.
See [Build plugins](#build-plugins--unreleased-preview) below for the unreleased authoring preview.

### Quick start

Choose your operating system and run the commands below.

<details>
<summary><strong>macOS</strong></summary>

With Homebrew:

```bash
brew install 777genius/agentplugins/agentplugins
agentplugins add context7
```

Without Homebrew, use the native installer:

```bash
curl -fsSL https://raw.githubusercontent.com/777genius/universal-agent-plugins/main/install.sh | sh -s -- add context7
```

</details>

<details>
<summary><strong>Linux</strong></summary>

Install the matching native binary and add your first plugin:

```bash
curl -fsSL https://raw.githubusercontent.com/777genius/universal-agent-plugins/main/install.sh | sh -s -- add context7
```

Homebrew also works on Linux: run `brew install 777genius/agentplugins/agentplugins`,
then `agentplugins add context7`.

</details>

<details>
<summary><strong>Windows</strong></summary>

Run in PowerShell:

```powershell
irm https://raw.githubusercontent.com/777genius/universal-agent-plugins/main/install.ps1 | iex
& "$HOME\.local\bin\agentplugins.exe" add context7
```

</details>

<details>
<summary><strong>Any OS with Node.js 22+ (npx)</strong></summary>

Run on a supported desktop OS without permanently installing the CLI:

```bash
npx universal-agent-plugins add context7
```

</details>

The CLI finds compatible agents installed on your computer.

1. Choose one or more agents if prompted. If only one is found, it is selected automatically.
2. Follow any activation or sign-in instructions printed by the CLI.
3. Start a new agent session and try the plugin.

For the verified Context7 setup in ChatGPT and Kiro:

```bash
npx universal-agent-plugins add context7 --target chatgpt,kiro
```

- ChatGPT: the CLI guides you to create a Developer Mode app for
  `https://mcp.context7.com/mcp` with **No authentication**, accepts the
  resulting `asdk_app_...` ID, and prepares a personal marketplace package.
  You still install that package and select Context7 in a new chat. When other
  clients are selected too, the CLI installs them first and reports ChatGPT as
  a separate setup step instead of cancelling the whole batch.
- Kiro: the CLI installs its owned skills and MCP entry while preserving
  unrelated configuration. On macOS and Windows it reports the remaining
  Kiro OAuth/restart step instead of claiming automatic runtime verification.

The same retained installation supports repeat add, update, repair, remove,
and reinstall. ChatGPT availability still depends on an account or workspace
where Developer Mode and custom apps are enabled.

See the [real Context7 ChatGPT and Kiro E2E evidence](docs/CONTEXT7_CHATGPT_KIRO_E2E.md).

<details>
<summary>Installation details</summary>

Homebrew and the installers select the native binary for your OS and architecture.
The installer scripts verify its published SHA-256 and reported version, then
replace the CLI atomically. They install into `$HOME/.local/bin` unless
`AGENTPLUGINS_BIN_DIR` is set.

The native CLI does not require Node.js. The `npx` option requires Node.js 22+
and downloads and runs the verified Go binary. Individual plugins may have
their own runtime requirements, which the CLI checks before installation.

The plugin package is downloaded and verified once, then prepared for each
selected agent.

</details>

[Browse plugins](https://777genius.github.io/universal-agent-plugins/plugins/)

## What the CLI does

- installs one plugin in one or several supported agents;
- updates, repairs, and removes only files it manages;
- converts the same Agent Plugins 1.0 package into each agent's native format;
- keeps activation and OAuth prompts visible to you.

## Any Agent Plugins 1.0 package

The installed package is standard-first:

```text
plugin.json
├── skills/       optional reusable instructions
├── mcp.json      optional MCP servers
└── hooks/        optional client-specific extension
```

Skills and MCP are portable components. Hooks and other client-specific
extensions depend on the target client; they do not imply portable behavior.

You can also install a local package or a pinned GitHub package without adding
it to the registry. Direct-install examples are collected near the end of this
README.

plugin.yaml is the legacy plugin-kit-ai authoring format. It is not merged with
or allowed to override plugin.json.

## Supported clients

The CLI has adapters for:

| Client | Delivery |
| --- | --- |
| <img src="landing/public/client-icons/openai.svg" width="24" height="24" alt="" align="middle"> Codex | managed package or OpenAI compatibility package |
| <img src="landing/public/client-icons/openai.svg" width="24" height="24" alt="" align="middle"> ChatGPT | verified publisher app binding, or guided personal Context7 app and marketplace preparation |
| <img src="landing/public/client-icons/cursor.svg" width="24" height="24" alt="" align="middle"> Cursor | native Agent Plugin and MCP/skills projection |
| <img src="landing/public/client-icons/github-copilot.svg" width="24" height="24" alt="" align="middle"> GitHub Copilot CLI | native plugin and managed marketplace path |
| <img src="landing/public/client-icons/vscode.svg" width="24" height="24" alt="" align="middle"> VS Code | prepared Copilot-compatible package |
| <img src="landing/public/client-icons/kiro.svg" width="24" height="24" alt="" align="middle"> Kiro | managed skills and MCP configuration; client OAuth/restart when required |
| <img src="landing/public/client-icons/claude.svg" width="24" height="24" alt="" align="middle"> Claude Code | client-specific skills/MCP projection |
| <img src="landing/public/client-icons/gemini.svg" width="24" height="24" alt="" align="middle"> Gemini CLI | client-specific configuration projection |
| <picture><source media="(prefers-color-scheme: dark)" srcset="assets/client-icons/opencode-dark.svg"><img src="landing/public/client-icons/opencode.svg" width="24" height="24" alt="" align="middle"></picture> OpenCode | client-specific configuration projection |
| <picture><source media="(prefers-color-scheme: dark)" srcset="assets/client-icons/cline-dark.svg"><img src="landing/public/client-icons/cline.svg" width="24" height="24" alt="" align="middle"></picture> Cline | client-specific configuration projection |
| <picture><source media="(prefers-color-scheme: dark)" srcset="assets/client-icons/windsurf-dark.svg"><img src="landing/public/client-icons/windsurf.svg" width="24" height="24" alt="" align="middle"></picture> Windsurf | client-specific configuration projection |

Compatibility is package-specific. A schema pass means that the package is
well-formed; it does not prove runtime, OAuth, or activation in every client.
The CLI prints installed, prepared, activation required, and authentication
pending as separate outcomes.

See the [client compatibility evidence](https://777genius.github.io/universal-agent-plugins/docs/en/reference/client-compatibility.html)
for tested client versions, platforms, and release-specific limitations.

For Codex, declared MCP SSE is unsupported; stdio and Streamable HTTP retain
their existing adapter support. Valid SSE components are excluded from Codex
delivery without invalidating the package. See [transport evidence and lifecycle
behavior](docs/CODEX_TRANSPORT_EVIDENCE.md).

## Find and verify plugins

search combines the reviewed Registry Directory with a signed public Discovery
Index containing conformant package paths. Discovery records are
unreviewed metadata, not endorsements. They install only through a
publisher-qualified exact-SHA selector and are validated again before mutation.

The registry is optional for direct installs, but it makes reviewed short names,
provenance, compatibility notes, and safe discovery convenient:

[Browse the registry](https://777genius.github.io/universal-agent-plugins/plugins/) ·
[Submit a package](https://github.com/777genius/universal-agent-plugins-registry/blob/main/CONTRIBUTING.md)

## Lifecycle safety

Before changing a client, the CLI validates the source and preflights every
selected target. --dry-run prints the same plan without writing. Managed files
and state are committed together; failures roll back what ownership proves safe
or stop with a repair command. OAuth and consent remain visible and controlled
by you. No install telemetry is sent.

The CLI is an independent community project. It is not affiliated with OpenAI,
Agent Plugins, or the vendors shown above.

## More commands

For normal interactive use, omit `--target` and choose agents in the prompt.
Use `--target` in scripts, CI, or whenever you want to name clients explicitly.

```bash
# Find and inspect plugins
agentplugins search docs
agentplugins info context7

# Install in specific agents
agentplugins add context7 --target codex,cursor,kiro

# Prepare Context7 for ChatGPT and Kiro
agentplugins add context7 --target chatgpt,kiro

# Manage an installed plugin
agentplugins update context7 --target codex,cursor
agentplugins repair context7 --target codex,cursor
agentplugins remove context7 --target codex,cursor
agentplugins outdated --all
agentplugins update --all
agentplugins doctor

# Install a local Agent Plugins 1.0 package
agentplugins validate ./my-plugin
agentplugins add ./my-plugin

# Install the only Agent Plugins package found at an exact commit
agentplugins add \
  owner/repository@0123456789abcdef0123456789abcdef01234567

# Choose a package explicitly when a repository contains several
agentplugins add \
  owner/repository@0123456789abcdef0123456789abcdef01234567//path/to/plugin
```

Remote installs require a full 40-character commit SHA. Branches, tags, and
abbreviated SHAs are rejected. When no package path is given, the CLI uses a
valid root `plugin.json` or auto-selects the only valid nested package that has
`mcp.json` or `skills/`. If several packages match, it lists them and asks for
an explicit `//path`. Repositories with more than 16 possible packages require
an explicit path before candidate packages are fetched. The selected canonical
path and package digest are stored for safe replay. Direct full-SHA installations
remain immutable; use `switch` to move to another exact source. `repair` reapplies
the recorded source, and `remove` changes only files owned by the CLI.

## Build plugins — unreleased preview

<a id="authoring-and-development"></a>

The [prepared Build guide source](website/source/en/build/index.md) describes
root `plugin.json` with optional `skills/` and `mcp.json`, followed by the offline
init → validate → inspect → static test loop and an installer planner handoff.
`agentplugins author` and `plugin-kit-ai` are the two prepared entrypoints to
one standard authoring engine. The standard-first authoring CLI is not released.
This is unreleased preview documentation, not
an announcement that `plugin-kit-ai@2` is available on npm. Public deployment
remains gated on release acceptance; the installer instructions above retain
their existing behavior. See the [Use / Build quickstart](https://777genius.github.io/universal-agent-plugins/docs/en/guide/quickstart.html).

Authoring validation and project doctor are distinct from installer
`agentplugins validate` and `agentplugins doctor`. Static checks do not prove
runtime execution, OAuth, or client activation. Runtime/dev/bootstrap,
client generation, export/bundle, and publication are deferred from this MVP.
There is no implicit YAML fallback or second supported YAML engine.

**Preparation — unreleased:** as checked on 2026-09-07, npm `universal-agent-plugins`
is 0.1.53 and the stable GitHub release is `agentplugins-v0.1.53`. npm/PyPI
`plugin-kit-ai` is 1.2.4: `plugin-kit-ai@latest` is the historical v1 tool, not
standard-first v2.

### Historical authoring and development

[Historical plugin-kit-ai v1, baseline 1.2.4](website/source/en/legacy/v1/index.md)
provides version context. Project migration is not available in v2 yet.
Maintain legacy projects using the v1 1.2.4 command set.
The [preserved authoring guide](docs/PLUGIN_KIT_AI_AUTHORING.md) explains the
historical YAML, generation, and export workflows; it is not the standard MVP.

Build one plugin and ship it to many AI agents was the legacy authoring goal.
The repository preserves Codex and Claude starters across Go, Python, and
Node/TypeScript. See the classified [starters](examples/starters/README.md),
[production examples](examples/plugins/README.md), [local examples](examples/local/README.md),
and [Skills components](examples/skills/README.md).

<details>
<summary>Historical v1 authoring and SDK reference — baseline 1.2.4</summary>

All commands, stability labels, and supported-output claims in this section
describe the preserved v1 workflow, not the unreleased standard authoring MVP.

`plugin-kit-ai` keeps authored source under `plugin/`, generates the supported outputs you need, and helps you validate the repo before handoff. This includes supported outputs for Claude, Codex, Gemini, Cursor, and OpenCode where the repo shape allows it. The honest promise is `one repo / many supported outputs`, not fake parity everywhere.

overview: [plugin-kit-ai documentation](https://github.com/777genius/universal-agent-plugins/blob/9beca10448ac50fbe526a52101d1433a12471980/website/source/en/index.md)
fastest start: [Quickstart](https://777genius.github.io/universal-agent-plugins/docs/en/guide/quickstart.html)
choose by job first: [Choose What You Are Building](https://github.com/777genius/universal-agent-plugins/blob/9beca10448ac50fbe526a52101d1433a12471980/website/source/en/guide/choose-what-you-are-building.md)
one repo, many outputs: [What You Can Build](https://github.com/777genius/universal-agent-plugins/blob/9beca10448ac50fbe526a52101d1433a12471980/website/source/en/guide/what-you-can-build.md)
honest caveat: [Support Boundary](https://github.com/777genius/universal-agent-plugins/blob/9beca10448ac50fbe526a52101d1433a12471980/website/source/en/reference/support-boundary.md)

### Legacy quick start

Use an exact v1 1.2.4 executable for these commands. The Homebrew and fallback
channels below are retained as historical references, not version-pinned setup.

```bash
# Historical Homebrew channel (not version-pinned):
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
```

npm: `npm i -g plugin-kit-ai@1.2.4` or `npx plugin-kit-ai@1.2.4 ...`

pipx (`public-beta`, only when that release is published to PyPI): `pipx install plugin-kit-ai==1.2.4`

fallback installer: `curl -fsSL https://raw.githubusercontent.com/777genius/plugin-kit-ai/main/scripts/install.sh | sh`

```bash
plugin-kit-ai init my-plugin --template online-service
plugin-kit-ai init my-plugin --template local-tool
plugin-kit-ai init my-plugin --template custom-logic
plugin-kit-ai init my-plugin
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

### Support and references

[examples/starters/README.md](examples/starters/README.md)
[examples/local/README.md](examples/local/README.md)
[docs/CHOOSING_HELPER_DELIVERY_MODE.md](docs/CHOOSING_HELPER_DELIVERY_MODE.md)
the stable local Python and Node subset on `codex-runtime` and `claude`
`doctor`, `bootstrap`, `validate --strict`, `export`, and bundle handoff for that stable local subset
`generate`, `import`, and `normalize` are still `public-beta`
[docs/generated/target_support_matrix.md](docs/generated/target_support_matrix.md)
[docs/generated/support_matrix.md](docs/generated/support_matrix.md)
[docs/SUPPORT.md](docs/SUPPORT.md)

### SDK and CLI

Go SDK packages: `github.com/777genius/plugin-kit-ai/sdk/claude`, `github.com/777genius/plugin-kit-ai/sdk/codex`, and `github.com/777genius/plugin-kit-ai/sdk/gemini`.

```bash
./bin/plugin-kit-ai doctor ./my-plugin
./bin/plugin-kit-ai bootstrap ./my-plugin
./bin/plugin-kit-ai import ./native-plugin --from codex-runtime
./bin/plugin-kit-ai capabilities --format json
```

`plugin-kit-ai validate --format json` now emits the versioned `plugin-kit-ai/validate-report` contract.
[docs/CODEX_TARGET_BOUNDARY.md](docs/CODEX_TARGET_BOUNDARY.md)
[docs/VALIDATE_JSON_CONTRACT.md](docs/VALIDATE_JSON_CONTRACT.md)

</details>

Contributor checks:

```bash
go test ./...
make vet
```

- Contributing: [CONTRIBUTING.md](CONTRIBUTING.md)
- Security policy: [SECURITY.md](SECURITY.md)
- Support boundary: [docs/SUPPORT.md](docs/SUPPORT.md)
- Native CLI installation: [docs/NATIVE_INSTALL.md](docs/NATIVE_INSTALL.md)
- Client E2E evidence: [docs/AGENTPLUGINS_CLIENT_E2E.md](docs/AGENTPLUGINS_CLIENT_E2E.md)
- Registry repository: https://github.com/777genius/universal-agent-plugins-registry

## License

Universal Agent Plugins is licensed under the [Apache License 2.0](LICENSE).
Third-party components retain their original licenses; see [NOTICE](NOTICE) for
attribution.
