<img width="2172" height="724" alt="Universal Agent Plugins - install and create plugins across AI agents with one command" src="assets/readme-banner.png" />

<p align="center">
    <img src="landing/public/client-icons/openai.svg" width="32" height="32" alt="Codex and ChatGPT" />&nbsp;&nbsp;
    <img src="landing/public/client-icons/cursor.svg" width="32" height="32" alt="Cursor" />&nbsp;&nbsp;
    <img src="landing/public/client-icons/github-copilot.svg" width="32" height="32" alt="GitHub Copilot CLI" />&nbsp;&nbsp;
    <img src="landing/public/client-icons/vscode.svg" width="32" height="32" alt="VS Code" />&nbsp;&nbsp;
    <img src="landing/public/client-icons/kiro.svg" width="32" height="32" alt="Kiro" />&nbsp;&nbsp;
    <img src="landing/public/client-icons/claude.svg" width="32" height="32" alt="Claude Code" />&nbsp;&nbsp;
    <img src="landing/public/client-icons/gemini.svg" width="32" height="32" alt="Gemini CLI" />&nbsp;&nbsp;
    <picture><source media="(prefers-color-scheme: dark)" srcset="assets/client-icons/opencode-dark.svg"><img src="landing/public/client-icons/opencode.svg" width="32" height="32" alt="OpenCode" /></picture>&nbsp;&nbsp;
    <picture><source media="(prefers-color-scheme: dark)" srcset="assets/client-icons/cline-dark.svg"><img src="landing/public/client-icons/cline.svg" width="32" height="32" alt="Cline" /></picture>&nbsp;&nbsp;
    <picture><source media="(prefers-color-scheme: dark)" srcset="assets/client-icons/windsurf-dark.svg"><img src="landing/public/client-icons/windsurf.svg" width="32" height="32" alt="Windsurf" /></picture>&nbsp;&nbsp;
    <picture><source media="(prefers-color-scheme: dark)" srcset="assets/client-icons/grok-dark.svg"><img src="landing/public/client-icons/grok.svg" width="32" height="32" alt="Grok Build" /></picture>&nbsp;&nbsp;
    <picture><source media="(prefers-color-scheme: dark)" srcset="assets/client-icons/kimi-dark.svg"><img src="landing/public/client-icons/kimi.svg" width="32" height="32" alt="Kimi Code" /></picture>&nbsp;&nbsp;
    <img src="landing/public/client-icons/qoder.svg" width="32" height="32" alt="Qoder" />&nbsp;&nbsp;
    <img src="landing/public/client-icons/pi.svg" width="32" height="32" alt="Pi" />
</p>

[![Required](https://github.com/777genius/universal-agent-plugins/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/777genius/universal-agent-plugins/actions/workflows/ci.yml)
[![Codecov](https://codecov.io/gh/777genius/universal-agent-plugins/graph/badge.svg)](https://codecov.io/gh/777genius/universal-agent-plugins)
[![npm](https://img.shields.io/npm/v/universal-agent-plugins?label=npm)](https://www.npmjs.com/package/universal-agent-plugins)
[![Agent Plugins 1.0](https://img.shields.io/badge/Agent%20Plugins-1.0.0-7257FF)](https://agent-plugins.org/specification)
[![Agent Skills 1.0](https://img.shields.io/badge/Agent%20Skills-1.0.0-7257FF)](https://agentskills.my/specification)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

Install, update, and remove Agent Plugins 1.0 packages across your AI agents with one CLI.
Save your setup in `uap.toml` and `uap.lock` to restore it on another computer or share it with a project.

[Documentation](https://universal-agent-plugins.com/docs/) ·
[Browse plugins](https://universal-agent-plugins.com/plugins/) ·
[Build a plugin](#build-plugins)

[Quick start](#quick-start) · [Project installation](#install-for-one-project) ·
[Supported clients](#supported-clients) · [SDK](#already-built-with-uap-sdk)

<a href="https://universal-agent-plugins.com/#demo" title="Watch the Universal Agent Plugins demo"><img width="680" height="392" alt="Universal Agent Plugins video preview — watch the demo" src="https://github.com/user-attachments/assets/33833cd2-f525-49b2-9181-5a4d643d2f5b" /></a>

## Use plugins

### Quick start

With Node.js 22+:

```bash
npx universal-agent-plugins add context7
```

The CLI finds compatible agents, including Codex and Claude Code. Choose agents
when prompted, follow any printed setup steps, then start a new agent session.

**Choose your agents**

<img width="423" height="234" alt="Choose which AI agents receive the plugin" src="https://github.com/user-attachments/assets/3f8786ea-bb3a-4869-aabe-99b245364b3d" />

**Review the installation plan — Playwright example**

<img width="1079" height="584" alt="Review the Playwright installation plan for Codex, Claude Code and Gemini CLI" src="https://github.com/user-attachments/assets/898d66b9-3942-4725-9a26-ec2fe01ad7c0" />

**Follow installation progress — Playwright example**

<img width="669" height="125" alt="Track Playwright installation progress across three AI agents" src="https://github.com/user-attachments/assets/3bd5d39f-5b35-4475-8b0c-94b85e992d29" />

### Install the CLI

The native CLI does not require Node.js. Install it once to use `agentplugins` directly.

**macOS and Linux — Homebrew:**

```bash
brew install 777genius/agentplugins/agentplugins
agentplugins add context7
```

**macOS and Linux — native installer:**

```bash
curl -fsSL https://raw.githubusercontent.com/777genius/universal-agent-plugins/main/install.sh | sh -s -- add context7
```

**Windows — PowerShell:**

```powershell
irm https://raw.githubusercontent.com/777genius/universal-agent-plugins/main/install.ps1 | iex
& "$HOME\.local\bin\agentplugins.exe" add context7
```

**npm — Node.js 22+:**

```bash
npm install -g universal-agent-plugins
agentplugins add context7
```

The native installers verify the binary's published SHA-256 and version and
install into `$HOME/.local/bin` (override with `AGENTPLUGINS_BIN_DIR`). Individual
plugins can require additional runtimes; the CLI checks these before installation.
See [native installation](docs/NATIVE_INSTALL.md) for options and troubleshooting.

The verified native release is [CLI **0.1.95**](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.95),
including local management and native Pi extensions. npm and Homebrew can lag;
check `agentplugins version` before using release-specific features. Features
in `main` may require a source build.

## What the CLI does

- Installs a package in one or several agents, delivering the components each supports.
- Saves dependencies and exact package pins for repeatable installation.
- Updates, repairs, and removes files it manages while preserving unrelated content.
- Reports remaining activation, authentication, and reload steps.

## Install for one project

Use project scope to keep a repository's plugins separate from your user setup.
Explicit project scope requires CLI 0.1.93+.

Open your project folder and choose a compatible package from the
[catalog](https://universal-agent-plugins.com/plugins/).

**Add a plugin**

```sh
cd /path/to/project
agentplugins add github:OWNER/REPOSITORY@FULL_COMMIT_SHA//PACKAGE_PATH \
  --scope project --target codex
```

Replace the example source with the package’s pinned Git source from the catalog;
omit `//PACKAGE_PATH` for a package at the repository root. Short catalog names
such as `context7` currently work only in user scope.

Commit `uap.toml` and `uap.lock`; add `.uap/` to `.gitignore`.
From the root of another checkout, run:

```sh
agentplugins install --locked --scope project
```

To install standalone Skills instead:

```sh
agentplugins skills add owner/repository --ui --scope project
```

Replace `owner/repository` with the actual Skills repository. Use
`--project-root /path/to/project` when running outside the project folder.

Add and maintenance commands otherwise default to user scope.

Client support varies by component. ChatGPT has no native project installation.
See [project installation](docs/PROJECT_INSTALLATION.md) for lifecycle commands
and [project client support](docs/PROJECT_CLIENT_SUPPORT.md) for paths and activation.

## Restore your plugin setup

To move your user setup to another computer, export it into an empty transfer folder:

```sh
agentplugins deps export /path/to/transfer
```

Copy that folder to the new computer, install the target agents, and restore:

```sh
agentplugins deps import /path/to/transfer --install
```

Export and import use the user environment by default and transfer only
`uap.toml` and `uap.lock`. Exact packages must still be available from their
sources or the destination cache. Authentication, secrets, client settings, and
plugin data need separate setup or backup.

Run `agentplugins deps status` to check supported targets. See the
[dependency guide](docs/UAP_DEPENDENCIES.md) and
[restore guide](https://universal-agent-plugins.com/docs/en/use/manage.html#restore-on-another-machine)
for supported environments and recovery.

## Any Agent Plugins 1.0 package

Install reviewed registry packages, local packages, Git sources over HTTPS or
SSH, or complete HTTPS/local archives. `plugin.json` is the only supported
authoring manifest.

| File or folder | Purpose |
| --- | --- |
| `plugin.json` | Required package manifest |
| `skills/` | Optional reusable instructions |
| `mcp.json` | Optional MCP servers |
| `hooks/` | Optional client-specific extensions |

Skills and MCP are portable components; hooks and other native extensions depend
on the client. See [direct package sources](docs/DIRECT_PACKAGE_SOURCES.md) for
source syntax, credentials, and immutable archive pins.

For standalone Agent Skills, use `agentplugins skills add` with a GitHub repository
or an HTTPS publisher's `.well-known` Skills index. Publisher installs require
an exact `--skill` name. See [installing Skills](docs/AGENT_SKILLS_INSTALLER.md).

## Supported clients

The table describes user-scope delivery. See the separate
[project support table](docs/PROJECT_CLIENT_SUPPORT.md) for project installations.

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
| <picture><source media="(prefers-color-scheme: dark)" srcset="assets/client-icons/grok-dark.svg"><img src="landing/public/client-icons/grok.svg" width="24" height="24" alt="" align="middle"></picture> Grok Build | native skills/MCP plugin; Grok CLI install and verification when available |
| <picture><source media="(prefers-color-scheme: dark)" srcset="assets/client-icons/kimi-dark.svg"><img src="landing/public/client-icons/kimi.svg" width="24" height="24" alt="" align="middle"></picture> Kimi Code | native skills/MCP plugin and managed user registry; reload required |
| <img src="landing/public/client-icons/qoder.svg" width="24" height="24" alt="" align="middle"> Qoder | native skills/MCP plugin; Qoder CLI install and verification when available |
| <img src="landing/public/client-icons/pi.svg" width="24" height="24" alt="" align="middle"> Pi | native extensions in released user scope; project Skills/MCP are separately qualified in source builds |

Pi native extensions ship in the 0.1.95 user-scope release. Project Pi
Extensions, Skills, and MCP preparation are qualified in source builds and do
not ship in that release. See the [Pi extension contract](docs/PI_NATIVE_EXTENSION_CONTRACT.md).

Grok Build, Kimi Code, and Qoder install portable skills and MCP servers in user
scope, with stdio and Streamable HTTP support. Qoder also preserves native
hooks, commands, and bundled binaries. See [portable commands](docs/UAP_COMMANDS.md)
for command delivery and client limits.

For guided Context7 setup in ChatGPT and Kiro:

```bash
npx universal-agent-plugins add context7 --target chatgpt,kiro --no-save
```

ChatGPT requires Developer Mode and custom apps, plus manual package installation
and selection in a new chat. `--no-save` is required here because ChatGPT
preparation is not a portable saved dependency. Prepare it separately if you
want to save other clients. See the [Context7 setup guide](docs/CONTEXT7_CHATGPT_KIRO_E2E.md).

A prepared installation may still need a reload, OAuth, or client approval.
Codex supports stdio and Streamable HTTP MCP, but excludes SSE components;
see [transport details](docs/CODEX_TRANSPORT_EVIDENCE.md). The
[compatibility reference](https://universal-agent-plugins.com/docs/en/reference/client-compatibility.html)
records tested client versions, platforms, and remaining steps.

## More commands

Omit `--target` to choose agents interactively; name them explicitly in scripts.

```bash
# Find and inspect packages
agentplugins search docs
agentplugins info context7

# Install in specific agents
agentplugins add context7 --target codex,claude

# Maintain an installed plugin
agentplugins update context7 --target codex,claude
agentplugins repair context7 --target codex,claude
agentplugins remove context7 --target codex,claude
agentplugins outdated --all
agentplugins update --all
agentplugins doctor

# Validate and try a local package
agentplugins validate ./my-plugin
agentplugins add ./my-plugin
```

Local directory installs do not save portable dependencies. Use a pinned direct
source when you want a setup that another computer can restore.

## Find and verify plugins

[Browse the catalog](https://universal-agent-plugins.com/plugins/) or use
`agentplugins search`. Reviewed Directory entries and unreviewed Discovery
results are labelled separately. Discovery metadata is not an endorsement;
packages are validated again before installation.

Direct installs do not need a registry entry. To list your package, see
[registry contributions](https://github.com/777genius/universal-agent-plugins-registry/blob/main/CONTRIBUTING.md).

## Lifecycle safety

The CLI validates packages and preflights selected clients before changing them.
Use `--dry-run` to preview managed changes; security checks may still download
scanner binaries or metadata. No install telemetry is sent.

Updates and removal check recorded ownership. Unrelated files and externally
edited entries are preserved; conflicts come with recovery instructions.
Failures roll back where ownership permits. Check each target's result after a
multi-agent operation. Repair checks managed files; authentication and runtime
behavior still need verification in the client.

See [removal safety](docs/REMOVAL_SAFETY.md) for ownership, retained data, and
external uninstall requirements. The
[installer automation principle](docs/INSTALLER_AUTOMATION_PRINCIPLE.md)
explains which steps the CLI completes and which require your involvement.

UAP is an independent community project, unaffiliated with OpenAI, Agent Plugins,
or the client vendors.

## Already built with UAP SDK

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

The [Go SDK](sdk/README.md) provides typed runtime hooks and client-specific
handlers. Reusable [Go installer components](install/integrationctl/agentplugins/installer/README.md)
support client integration.
The [TypeScript Management SDK](sdks/typescript/README.md), published as
`universal-agent-plugins-management-sdk`, controls the CLI 0.1.95+ management
engine. Pin SDK and engine versions independently; see the
[management guide](https://universal-agent-plugins.com/docs/en/use/manage.html#local-management-api).

## Build plugins

Create a package with a root `plugin.json`, with optional `skills/` and `mcp.json`,
then validate and install it using the local-package commands above.

The current npm and native installer releases do not include `agentplugins author`.
In qualified authoring builds, `agentplugins author` is the public authoring entrypoint.
These commands require a qualified authoring build or a current source build:

```bash
agentplugins author init ./my-plugin --template skill --name my-plugin
agentplugins author validate ./my-plugin
agentplugins author inspect ./my-plugin
agentplugins author test ./my-plugin
```

The [Build guide](https://universal-agent-plugins.com/docs/en/build/) covers
Skills, remote and stdio MCP, hybrid packages, and installation. Start with the
[Use / Build quickstart](https://universal-agent-plugins.com/docs/en/guide/quickstart.html).
Native installer [releases](https://github.com/777genius/universal-agent-plugins/releases)
use `agentplugins-v*` tags.

<details>
<summary>Source-build authoring requirements</summary>

In qualified/source builds, MCP execution is available through `author test` and `author dev`.
Dependency bootstrap apply currently requires Linux; dry-run planning can be used on other platforms.
Local stdio runtime checks and development require Linux, successful process-containment and bubblewrap preflights, and a supported system Node path.
See the [stdio guide](https://universal-agent-plugins.com/docs/en/build/mcp-stdio.html)
for prerequisites and the development loop. Client projection, export/bundle, and publication are not exposed by the current authoring CLI.
`plugin.json` is the only supported authoring manifest.

</details>

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the development workflow. Run:

```bash
go test ./...
make vet
```

[Security policy](SECURITY.md) · [Support boundary](docs/SUPPORT.md) ·
[Client E2E evidence](docs/AGENTPLUGINS_CLIENT_E2E.md)

## License

[Apache License 2.0](LICENSE). Third-party components retain their original
licenses; see [NOTICE](NOTICE).
