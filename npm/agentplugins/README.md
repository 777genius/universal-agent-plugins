# Universal Agent Plugins

Install and manage Agent Plugins 1.0 across your AI agents with one CLI.

```bash
npx universal-agent-plugins add context7
```

The CLI finds compatible agents installed on your computer and asks where to
install the plugin. Choose one or several. The package is downloaded and
verified once, then prepared for every agent you selected.

You need Node.js 22 or newer.

The verified native baseline is CLI 0.1.95. npm and Homebrew can lag behind the
native release; check `npx universal-agent-plugins version` or
`agentplugins version` for the channel you installed. Automatic current-directory
project selection requires 0.1.94. On 0.1.93, use explicit project scope instead.
For a newer feature, install a suitable exact native version using the
[native installation guide](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md#pinning-and-automation);
switching to another package manager does not by itself ensure a newer version.

To use the same native Go CLI without Node.js, install it with Homebrew or the
verified scripts in the [project README](https://github.com/777genius/universal-agent-plugins#quick-start).

## Quick start

1. Run the command above.
2. Select the agents you want to use. One detected agent is selected
   automatically; with several agents, the CLI shows a multi-select.
3. Follow any activation or sign-in instruction printed by the CLI.
4. Start a new agent session and use the plugin.

For the verified Context7 setup in ChatGPT and Kiro:

```bash
npx universal-agent-plugins add context7 --target chatgpt,kiro --no-save
```

The mixed command uses `--no-save`: ChatGPT preparation is not a portable
dependency. Prepare ChatGPT separately when saving the other clients.

For ChatGPT, follow the printed Developer Mode steps, use
`https://mcp.context7.com/mcp` with **No authentication**, and rerun the
printed command with the `asdk_app_...` ID. The CLI then prepares the personal
marketplace package; installing it and selecting Context7 remain visible
ChatGPT steps. For Kiro, the CLI manages its owned skills and MCP entry while
preserving unrelated configuration, then reports any OAuth or restart step.
macOS and Windows preparation does not claim automatic runtime verification.

See the [real Context7 ChatGPT and Kiro E2E evidence](https://github.com/777genius/universal-agent-plugins/blob/main/docs/CONTEXT7_CHATGPT_KIRO_E2E.md).

[Browse plugins](https://universal-agent-plugins.com/plugins/) ·
[Read the documentation](https://universal-agent-plugins.com/docs/)

## What it does

- installs one plugin in one or several supported agents;
- updates, repairs, and removes only files it manages;
- converts the same Agent Plugins 1.0 package into each agent's native format;
- keeps activation and OAuth prompts visible to you.

Supported clients include Codex, ChatGPT, Cursor, GitHub Copilot CLI, VS Code,
Kiro, Claude Code, Gemini CLI, OpenCode, Cline, Windsurf, Grok Build, Kimi Code,
and Qoder. Compatibility depends on the package, selected scope and client.
The CLI distinguishes installed components, prepared configuration and remaining
activation or sign-in steps. CLI 0.1.95 adds user-scope native Pi extensions;
portable Skills and MCP projection are outside that provider's scope. Check the
installed channel version and the version-bound native evidence before use.

See the [client compatibility reference](https://universal-agent-plugins.com/docs/en/reference/client-compatibility.html)
for version-bound evidence and limits.

## Install any Agent Plugins 1.0 package

Short names come from the signed Universal Agent Plugins Registry. You can also
install a valid package from a local directory, Git over HTTPS or SSH, supported
GitHub/GitLab/Azure browser links, or a complete HTTPS/local archive without
submitting it to the registry. These transports ship in CLI 0.1.93.
See [direct package sources](https://github.com/777genius/universal-agent-plugins/blob/main/docs/DIRECT_PACKAGE_SOURCES.md)
for ref selection, immutable pins and private Git credentials.

The standard `plugin.json` is the installation authority. Other manifests cannot
override it. To install a standalone `SKILL.md` and its resources, use the separate
`skills add` workflow described in the [Skills installer guide](https://github.com/777genius/universal-agent-plugins/blob/main/docs/AGENT_SKILLS_INSTALLER.md).

## More commands

For normal interactive use, omit `--target` and choose agents in the prompt.
Use `--target` in scripts, CI, or whenever you want to name clients explicitly.

```bash
# Find and inspect plugins
npx universal-agent-plugins search docs
npx universal-agent-plugins info context7

# Install in specific agents
npx universal-agent-plugins add context7 --target codex,cursor,kiro

# Prepare Context7 for ChatGPT and Kiro
npx universal-agent-plugins add context7 --target chatgpt,kiro --no-save

# Manage installed plugins
npx universal-agent-plugins update context7 --target codex,cursor
npx universal-agent-plugins repair context7 --target codex,cursor
npx universal-agent-plugins remove context7 --target codex,cursor
npx universal-agent-plugins outdated --all
npx universal-agent-plugins update --all
npx universal-agent-plugins doctor

# Install a local package
npx universal-agent-plugins validate ./my-plugin
npx universal-agent-plugins add ./my-plugin

# Install the only Agent Plugins package found at an exact commit
npx universal-agent-plugins add \
  owner/repository@0123456789abcdef0123456789abcdef01234567

# Choose a package explicitly when a repository contains several
npx universal-agent-plugins add \
  owner/repository@0123456789abcdef0123456789abcdef01234567//path/to/plugin
```

These GitHub selectors require a full 40-character commit SHA. Branches, tags,
and abbreviated SHAs are rejected in this spelling. When no package path is given, the CLI uses a
valid root `plugin.json` or auto-selects the only valid nested package that has
`mcp.json` or `skills/`. If several packages match, it lists them and asks for
an explicit `//path`. Repositories with more than 16 possible packages require
an explicit path before candidate packages are fetched. The selected canonical
path and package digest are stored for safe replay. Direct full-SHA installations
remain immutable; use `switch` to move to another exact source. `repair` reapplies
the recorded source, and `remove` changes only files owned by the CLI.

## Restore your setup

CLI 0.1.90 and later can transfer the exact `uap.toml` and `uap.lock` pair:

```bash
npx universal-agent-plugins deps export ./plugin-setup
# Copy plugin-setup to the destination computer, then:
npx universal-agent-plugins deps import ./plugin-setup --install
```

Export/import default to the user environment. Restore requires the supported
clients to be installed and the exact locked sources or cached bytes to remain
available. Credentials, native settings and plugin data need separate setup or
backup. Read the [management guide](https://universal-agent-plugins.com/docs/en/use/manage.html)
for confirmation and recovery behavior.

## Install for one project

Explicit project scope ships in CLI 0.1.93. Replace `/path/to/project` with an
existing directory and install the selected clients first. Save a registry
dependency, then restore its recorded lock:

```bash
npx universal-agent-plugins add context7 --target codex,claude \
  --scope project --project-root /path/to/project
npx universal-agent-plugins install --locked \
  --scope project --project-root /path/to/project
```

The first command saves the project's `uap.toml` and `uap.lock` after successful
preflight and confirmation; the second replays that saved environment. Direct
local-path installs such as `add ./my-plugin` do not create portable dependency
intent or the lock required by `install --locked`; use the saved dependency flow
above for reproducible restoration.

Pass the same scope on later maintenance commands. In CLI 0.1.94, argumentless
`install` selects project scope for a regular `uap.toml` in the exact current
directory; it never searches parents, and explicit scope wins. It strictly checks
an existing lock before install or repair; only an absent lock permits initial
resolution. `--locked` additionally requires the lock; `update` changes pins.
Portable declarations and pins live at
the project root; ownership, recovery and retained data stay local under `.uap/`.
Each client has its own project component and trust limits. ChatGPT has no native
project installation. See [project installation](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md).

## Build plugins

Create a root `plugin.json` with optional `skills/` and `mcp.json`, then validate
and install the package. The verified 0.1.95 installer does not expose
`agentplugins author`. Some examples in the
[Build guide](https://universal-agent-plugins.com/docs/en/build/) require a
qualified authoring build or a source build. The TypeScript Management SDK is
the separate npm package `universal-agent-plugins-management-sdk`; it drives
the `manage` commands of CLI 0.1.95 and newer. Its
[SDK README](https://github.com/777genius/universal-agent-plugins/blob/main/sdks/typescript/README.md)
covers engine compatibility and package checks. Pin the SDK and engine
independently.

## Safety and verification

Install planning validates the package and preflights the selected agents before
changing their managed files. A failure can leave successful changes on other
selected clients; inspect the reported per-client result and recovery guidance.
Rollback and recovery operate only where ownership is proven. `--dry-run` prints the plan without applying managed client changes; security
checks may still acquire scanner binaries or metadata.
The CLI does not execute packages during search and sends no installation
telemetry.

The npm package has no `postinstall`. On first execution it downloads the exact
versioned Go binary for macOS, Linux, or Windows, verifies its embedded SHA-256,
and caches it locally. Release builds cover x64 and arm64 on all three operating systems. Native-client
verification has a separate, versioned compatibility matrix; a binary build is
not proof of client runtime, authentication or model/tool execution.

Third-party license text is included in `THIRD_PARTY_NOTICES.txt` in this npm
package. For direct native downloads, retain the companion notice file from
the same release with the binary, including when redistributing.

- [Source and documentation](https://github.com/777genius/universal-agent-plugins)
- [Browse the plugin catalog](https://universal-agent-plugins.com/plugins/)
- [Client E2E evidence](https://github.com/777genius/universal-agent-plugins/blob/main/docs/AGENTPLUGINS_CLIENT_E2E.md)

Universal Agent Plugins is an independent community project. It is not
affiliated with OpenAI, Agent Plugins, or the supported client vendors.
