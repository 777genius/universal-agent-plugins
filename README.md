<img width="2172" height="724" alt="Universal Agent Plugins - install and create plugins across AI agents with one command" src="assets/readme-banner.png" />

[![Required](https://github.com/777genius/universal-agent-plugins/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/777genius/universal-agent-plugins/actions/workflows/ci.yml)
[![Codecov](https://codecov.io/gh/777genius/universal-agent-plugins/graph/badge.svg)](https://codecov.io/gh/777genius/universal-agent-plugins)
[![npm](https://img.shields.io/npm/v/universal-agent-plugins?label=npm)](https://www.npmjs.com/package/universal-agent-plugins)
[![Agent Plugins 1.0](https://img.shields.io/badge/Agent%20Plugins-1.0.0-7257FF)](https://agent-plugins.org/specification)
[![Agent Skills 1.0](https://img.shields.io/badge/Agent%20Skills-1.0.0-7257FF)](https://agentskills.my/specification)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

Install and manage Agent Plugins 1.0 across your AI agents with one CLI.

## Table of contents

- [Use plugins](#use-plugins)
  - [Quick start](#quick-start)
- [What the CLI does](#what-the-cli-does)
- [Restore your plugin setup](#restore-your-plugin-setup)
- [Install for one project](#install-for-one-project)
- [Any Agent Plugins 1.0 package](#any-agent-plugins-10-package)
- [Supported clients](#supported-clients)
- [Already built with UAP SDK](#already-built-with-uap-sdk)
- [Find and verify plugins](#find-and-verify-plugins)
- [Lifecycle safety](#lifecycle-safety)
- [More commands](#more-commands)
- [Build plugins](#build-plugins)
- [Contributing](#contributing)
- [License](#license)

**Keep a reproducible set of plugins and dependencies, and carry it to another
computer.** The manifest records what you need; the lockfile pins the exact
packages to restore. This workflow ships in the published CLI; see
[Restore your plugin setup](#restore-your-plugin-setup). For the published
installer, start with the [Quick start](#quick-start).

Our [installer automation principle](docs/INSTALLER_AUTOMATION_PRINCIPLE.md) is to complete every ownership-safe client step automatically and ask the user only for genuine external approvals, authentication, or reloads.

[Read the documentation](https://universal-agent-plugins.com/docs/en/) ·
[Browse plugins](https://777genius.github.io/universal-agent-plugins/plugins/)

<a href="https://universal-agent-plugins.com/"><img width="495" height="285" alt="image" src="https://github.com/user-attachments/assets/33833cd2-f525-49b2-9181-5a4d643d2f5b" /></a>
<img width="423" height="234" alt="image" src="https://github.com/user-attachments/assets/3f8786ea-bb3a-4869-aabe-99b245364b3d" />
<img width="1079" height="584" alt="image" src="https://github.com/user-attachments/assets/898d66b9-3942-4725-9a26-ec2fe01ad7c0" />
<img width="669" height="125" alt="image" src="https://github.com/user-attachments/assets/3bd5d39f-5b35-4475-8b0c-94b85e992d29" />

## Use plugins

Install, inspect, update, repair, and remove packages with the existing installer.
The [Use guide source](website/source/en/use/index.md) collects this journey.
See [Build plugins](#build-plugins) below to create and check a package.

### Quick start

With Node.js 22+, run:

```bash
npx universal-agent-plugins add context7
```

The CLI automatically finds compatible agents, including Codex and Claude Code.
Choose agents if several are found; a single detected agent is selected automatically.
Follow the printed setup steps, then start a new agent session.

The current verified native release is CLI **0.1.94**, also available through
Homebrew. Distribution channels can update at different times. Check
`agentplugins version` (or `npx universal-agent-plugins version`) before using
0.1.94-only automatic project selection. If your selected channel is older,
use the native/Homebrew installation below or keep the explicit project-scope
commands supported since 0.1.93.

<details>
<summary>Other installation options</summary>

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

To install the latest installer globally with Node.js 22+:

```bash
npm install -g universal-agent-plugins
```

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

## Restore your plugin setup

These commands are part of the published CLI (`agentplugins` 0.1.90 and later).
The [Quick start](#quick-start) above covers installation.

On the source computer, open a folder to use for the transfer and run:

```sh
agentplugins deps export
```

Copy that folder to the new computer, install the required target clients,
and open the copied folder. Then restore the recorded plugins and dependencies:

```sh
agentplugins deps import --install
```

The optional directory argument defaults to the current folder. By default,
export reads the global user environment and import writes it. Only `uap.toml` and `uap.lock`
are transferred. Restore verifies exact locked packages from still-available
sources or the destination cache for supported dependency clients: Claude Code,
Codex, OpenCode, Cursor and Gemini CLI.

Authentication, client settings, secrets and plugin data need separate setup
or backup. Transferring the files does not guarantee offline installation.
See [Restore on another machine](https://universal-agent-plugins.com/docs/en/use/manage.html#restore-on-another-machine)
for confirmation prompts, supported restore behavior and recovery.

[Install Agent Skills](docs/AGENT_SKILLS_INSTALLER.md) with `agentplugins skills add`
from a GitHub repository or from an HTTPS publisher that serves a `.well-known`
Skills index. Publisher installs take an exact `--skill` name. The local browser
picker and saved dependencies are included.

## Install for one project

**Released in CLI 0.1.93:** select project scope explicitly:

```sh
agentplugins add owner/repo --alias review --target cursor \
  --scope project --project-root /path/to/project
agentplugins install --locked --scope project --project-root /path/to/project
agentplugins skills add owner/repository --ui \
  --scope project --project-root /path/to/project
```

The portable `uap.toml` and `uap.lock` live in that project. Local receipts,
recovery state and retained data live under `.uap/`; the user's environment
remains separate. Add and maintenance commands default to user scope; explicit project scope
without `--project-root` uses the current directory. In CLI 0.1.94, argumentless
`agentplugins install` selects project scope when that exact directory contains a
regular `uap.toml`; it never searches parents. Explicit scope wins. An existing
lock is checked strictly before install/repair, while a missing lock permits
first-install resolution. Use `update` to advance pins; `--locked` also requires
a lock to exist. Native capabilities and activation
requirements depend on the client. ChatGPT has no native project installation.
See [Project installation](docs/PROJECT_INSTALLATION.md)
for supported-target reporting, update/repair/removal and portable restore.
The [current project support table](docs/PROJECT_CLIENT_SUPPORT.md) records each
client's component paths and activation limits; user-scope compatibility below
is a separate contract.

VS Code project scope prepares `.github/skills` and owned `.vscode/mcp.json`
servers for one local folder. Its Prepared result leaves workspace/server trust
and runtime selection to the client. See [VS Code project preparation](docs/PROJECT_INSTALLATION.md#vs-code-project-preparation).

## Any Agent Plugins 1.0 package

Install from the registry, a local package, Git over HTTPS or SSH, or a complete
HTTPS archive. `plugin.json` is the sole supported authoring manifest.

<details>
<summary>Package layout and portability</summary>

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

**Released in CLI 0.1.93:** direct sources include arbitrary HTTPS and SSH Git,
GitHub/GitLab/Azure browser URLs, and complete HTTPS/local ZIP/TAR/TAR.GZ/TGZ
packages. See [Direct complete package sources](docs/DIRECT_PACKAGE_SOURCES.md)
for syntax, native credentials, immutable archive byte pins and supported limits.
These complete-package `agentplugins add` transports do not import standalone
skill files as plugins. The separate `agentplugins skills add`
workflow above installs raw Agent Skills through deterministic normalization.

Other manifest formats are ignored and cannot override `plugin.json`.

</details>

## Supported clients

The CLI has adapters for:

| Client | Delivery |
| --- | --- |
| <img src="landing/public/client-icons/openai.svg" width="24" height="24" alt="" align="middle"> Codex | plugin package prepared for Codex |
| <img src="landing/public/client-icons/openai.svg" width="24" height="24" alt="" align="middle"> ChatGPT | app setup; Context7 includes guided personal setup (Developer Mode and custom apps required) |
| <img src="landing/public/client-icons/cursor.svg" width="24" height="24" alt="" align="middle"> Cursor | native plugin with skills and MCP servers |
| <img src="landing/public/client-icons/github-copilot.svg" width="24" height="24" alt="" align="middle"> GitHub Copilot CLI | native plugin installed through a managed marketplace |
| <img src="landing/public/client-icons/vscode.svg" width="24" height="24" alt="" align="middle"> VS Code | plugin package prepared for Copilot |
| <img src="landing/public/client-icons/kiro.svg" width="24" height="24" alt="" align="middle"> Kiro | skills and MCP setup; OAuth/restart may be required |
| <img src="landing/public/client-icons/claude.svg" width="24" height="24" alt="" align="middle"> Claude Code | native Claude Code plugin installation |
| <img src="landing/public/client-icons/gemini.svg" width="24" height="24" alt="" align="middle"> Gemini CLI | skills and MCP configuration |
| <picture><source media="(prefers-color-scheme: dark)" srcset="assets/client-icons/opencode-dark.svg"><img src="landing/public/client-icons/opencode.svg" width="24" height="24" alt="" align="middle"></picture> OpenCode | skills and MCP configuration |
| <picture><source media="(prefers-color-scheme: dark)" srcset="assets/client-icons/cline-dark.svg"><img src="landing/public/client-icons/cline.svg" width="24" height="24" alt="" align="middle"></picture> Cline | skills and MCP configuration |
| <picture><source media="(prefers-color-scheme: dark)" srcset="assets/client-icons/windsurf-dark.svg"><img src="landing/public/client-icons/windsurf.svg" width="24" height="24" alt="" align="middle"></picture> Windsurf | MCP configured; skills prepared for manual use |
| <img src="landing/public/client-icons/grok.svg" width="24" height="24" alt="" align="middle"> Grok Build | native skills/MCP plugin; Grok CLI install and verification when available |
| <img src="landing/public/client-icons/kimi.svg" width="24" height="24" alt="" align="middle"> Kimi Code | native skills/MCP plugin and managed user registry; reload required |
| <img src="landing/public/client-icons/qoder.svg" width="24" height="24" alt="" align="middle"> Qoder | native skills/MCP plugin; Qoder CLI install and verification when available |

Grok Build, Kimi Code, and Qoder accept automatic user-scope installation of portable skills and MCP servers through `agentplugins add ./my-plugin --target grok,kimi,qoder`. Grok Build and Kimi Code support stdio/HTTP MCP; the Qoder adapter currently supports stdio MCP. The CLI verifies native registration and supports update and removal. A running client still needs a reload, and MCP authentication/connectivity is verified in that client.

Qoder also preserves target-native `hooks/**`, `commands/**`, and bundled `bin/**` files for native discovery. Its Go SDK supports `SessionStart`, `UserPromptSubmit`, `PostToolUse`, and `Stop`. These native files are separate from portable hook and command conversion; portable agents, hooks, commands, and LSP components remain outside these three adapters' supported scope.

Compatibility depends on the package and client. The CLI tells you what is
installed, prepared, or still needs activation or sign-in.

See the [client compatibility evidence](https://universal-agent-plugins.com/docs/en/reference/client-compatibility.html)
for tested client versions, platforms, and release-specific limitations.

<details>
<summary>Context7 setup for ChatGPT and Kiro</summary>

```bash
npx universal-agent-plugins add context7 --target chatgpt,kiro --no-save
```

This mixed example uses `--no-save` because ChatGPT preparation is not a saved
portable dependency. To keep other clients in a saved environment, prepare
ChatGPT separately.

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

</details>

<details>
<summary>Compatibility and Codex MCP transport limits</summary>

A schema pass means that the package is well-formed; it does not prove runtime,
OAuth, or activation in every client.

For Codex, declared MCP SSE is unsupported; stdio and Streamable HTTP retain
their existing adapter support. Valid SSE components are excluded from Codex
delivery without invalidating the package. See [transport evidence and lifecycle
behavior](docs/CODEX_TRANSPORT_EVIDENCE.md).

</details>

## Already built with UAP SDK

Build with reusable Go installer components and typed runtime hooks. The
[TypeScript Management SDK](sdks/typescript/README.md) is available in source as
a development package; it has no public npm release.

<table>
  <tr>
    <td width="104" align="center">
      <a href="https://github.com/777genius/agent-notifications"><img src="landing/public/showcase/agent-notifications.png" width="80" height="80" alt="Agent Notifications logo" /></a>
    </td>
    <td>
      <strong><a href="https://github.com/777genius/agent-notifications">Agent Notifications</a></strong><br />
      Desktop notifications, sounds, and click-to-focus for AI coding agents.
    </td>
  </tr>
</table>

Read the [Go SDK integration guide](sdk/README.md) for typed runtime hooks and
client-specific handlers.

## Find and verify plugins

Use `agentplugins search` or browse the registry to find plugins.
Reviewed registry entries and unreviewed discovery results are labelled separately.

<details>
<summary>How discovery results are verified</summary>

`search` combines the reviewed Registry Directory with a signed public Discovery
Index containing conformant package paths. Discovery records are
unreviewed metadata, not endorsements. They install only through a
publisher-qualified exact-SHA selector and are validated again before mutation.

</details>

The registry is optional for direct installs, but it makes reviewed short names,
provenance, compatibility notes, and safe discovery convenient:

[Browse the registry](https://777genius.github.io/universal-agent-plugins/plugins/) ·
[Submit a package](https://github.com/777genius/universal-agent-plugins-registry/blob/main/CONTRIBUTING.md)

## Lifecycle safety

The CLI checks packages before installation and changes only files it manages.
Use `--dry-run` to preview changes. OAuth and consent stay under your control.
No install telemetry is sent.

Removal checks recorded ownership before changing managed files or configuration
entries. Foreign content and externally edited entries are preserved; unresolved
ownership produces a conflict or a recovery instruction. Plugin data and shared
source caches can be retained intentionally. Some clients require an explicit
external uninstall before UAP can settle its own binding.

Lifecycle tests include file inventories and foreign-content checks. Real native
runs, deterministic adapter tests and manual setup are different kinds of
evidence; their coverage depends on the client, version, platform and scope.
See [removal safety](docs/REMOVAL_SAFETY.md) and the
[client compatibility reference](https://universal-agent-plugins.com/docs/en/reference/client-compatibility.html)
for those boundaries.

<details>
<summary>Ownership and rollback</summary>

Before changing a client, the CLI validates the source and preflights every
selected target. `--dry-run` prints the plan without applying managed client changes.
Security checks may still acquire scanner binaries or metadata. Managed files
and state are committed together; failures roll back what ownership proves safe
or stop with a repair command. Preview does not activate a plugin or sign you in.
Repair checks managed files; it does not prove external OAuth or runtime behavior.
A multi-target operation can have partial results; inspect each target and its
recovery instruction before retrying.

</details>

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
agentplugins add context7 --target codex,claude

# Manage an installed plugin
agentplugins update context7 --target codex,claude
agentplugins repair context7 --target codex,claude
agentplugins remove context7 --target codex,claude
agentplugins outdated --all
agentplugins update --all
agentplugins doctor

# Install a local Agent Plugins 1.0 package
agentplugins validate ./my-plugin
agentplugins add ./my-plugin
```

<details>
<summary>Install directly from an exact GitHub commit</summary>

```bash
# Install the only Agent Plugins package found at an exact commit
agentplugins add \
  owner/repository@0123456789abcdef0123456789abcdef01234567

# Choose a package explicitly when a repository contains several
agentplugins add \
  owner/repository@0123456789abcdef0123456789abcdef01234567//path/to/plugin
```

These historical GitHub selectors require a full 40-character commit SHA.
Branches, tags, and abbreviated SHAs are rejected in this spelling. When no
package path is given, the CLI uses a
valid root `plugin.json` or auto-selects the only valid nested package that has
`mcp.json` or `skills/`. If several packages match, it lists them and asks for
an explicit `//path`. Repositories with more than 16 possible packages require
an explicit path before candidate packages are fetched. The selected canonical
path and package digest are stored for safe replay. Direct full-SHA installations
remain immutable; use `switch` to move to another exact source. `repair` reapplies
the recorded source, and `remove` changes only files owned by the CLI.

</details>

## Build plugins

Create a portable Agent Plugins 1.0 package manually from a root `plugin.json`, with
optional `skills/` and `mcp.json`, then validate and install it with the commands
above. The current npm and native installer releases do not include
`agentplugins author`.

The [Build guide](website/source/en/build/index.md) covers package structure,
remote MCP, stdio MCP, hybrid packages, and the handoff to installation. Its
authoring commands require a qualified authoring build or a source build.
See the [Use / Build quickstart](https://universal-agent-plugins.com/docs/en/guide/quickstart.html).
[Native Agent Plugins releases](https://github.com/777genius/universal-agent-plugins/releases) use the `agentplugins-v*` tag prefix.

<details>
<summary>Qualified/source-build authoring commands (not in the current installer release)</summary>

In qualified authoring builds, `agentplugins author` is the public authoring
entrypoint. These examples require such a build or a current source build;
installing the latest npm or native installer does not provide them.

Create and check a Skill package:

```bash
agentplugins author init ./my-plugin --template skill --name my-plugin
agentplugins author validate ./my-plugin
agentplugins author inspect ./my-plugin
agentplugins author test ./my-plugin
```

Current source builds also include JSON maintenance, dependency bootstrap, and
a continuous MCP development loop. For a separate stdio MCP package, start with
an absent `./local-tools` destination:

```bash
agentplugins author init ./local-tools --template mcp-stdio --runtime node \
  --name local-tools --description "Local Node MCP tools"
agentplugins author normalize ./local-tools --document plugin.json
agentplugins author normalize ./local-tools --document mcp.json --write
```

After reviewing the generated source and satisfying the Linux dependency and
runtime prerequisites in the [stdio guide](https://universal-agent-plugins.com/docs/en/build/mcp-stdio.html),
select its actual server explicitly:

```bash
agentplugins author bootstrap ./local-tools --dry-run
agentplugins author bootstrap ./local-tools
agentplugins author dev ./local-tools --server local-tools
```

Normalization plans are read-only until `--write`. To import an existing Claude
MCP configuration, select its actual strict-JSON file and a new absent output:

```bash
agentplugins author import native ./claude.json --from claude \
  --output /absolute/path/to/new-plugin --name new-plugin \
  --description "Imported Claude MCP package" --write
```

Native import reads only the explicit strict-JSON Claude file and accepts safe
stdio `command`/`args` entries. Entries with `env`, `type` or `url`, including
HTTP servers, are skipped; it publishes only to an absent output. These
commands become supported installation guidance after a signed `agentplugins`
authoring release passes the public-channel checks.

Authoring validation and project doctor are distinct from installer
`agentplugins validate` and `agentplugins doctor`. In qualified/source builds,
MCP execution is available through `author test` and `author dev`.
Dependency bootstrap apply currently requires Linux; dry-run planning can be
used on other platforms. Local stdio runtime checks and development also require
Linux, successful process-containment and bubblewrap preflights, and a supported
system Node path. See the
[stdio guide](https://universal-agent-plugins.com/docs/en/build/mcp-stdio.html)
for those prerequisites. Client projection,
export/bundle, and publication are not exposed by the current authoring CLI.
OAuth and activation inside a supported client still require client-specific
verification.
`plugin.json` is the only supported authoring manifest. JSON maintenance does
not add a YAML reader, fallback, or migration workflow.

</details>

## Contributing

Contributor checks:

```bash
go test ./...
make vet
```

- Contributing: [CONTRIBUTING.md](CONTRIBUTING.md)
- Security policy: [SECURITY.md](SECURITY.md)
- Support boundary: [docs/SUPPORT.md](docs/SUPPORT.md)
- Native CLI installation: [docs/NATIVE_INSTALL.md](docs/NATIVE_INSTALL.md)
- Portable commands: [docs/UAP_COMMANDS.md](docs/UAP_COMMANDS.md)
- Client E2E evidence: [docs/AGENTPLUGINS_CLIENT_E2E.md](docs/AGENTPLUGINS_CLIENT_E2E.md)
- Registry repository: https://github.com/777genius/universal-agent-plugins-registry

## License

Universal Agent Plugins is licensed under the [Apache License 2.0](LICENSE).
Third-party components retain their original licenses; see [NOTICE](NOTICE) for
attribution.
