# Universal Agent Plugins

Install and manage Agent Plugins 1.0 across your AI agents with one CLI.

```bash
npx universal-agent-plugins add context7
```

The CLI finds compatible agents installed on your computer and asks where to
install the plugin. Choose one or several. The package is downloaded and
verified once, then prepared for every agent you selected.

You need Node.js 22 or newer.

To use the same native Go CLI without Node.js, install it with Homebrew or the
verified scripts in the [project README](https://github.com/777genius/universal-agent-plugins#quick-start).

## Quick start

1. Run the command above.
2. Select the agents you want to use. One detected agent is selected
   automatically; with several agents, the CLI shows a multi-select.
3. Follow any activation or sign-in instruction printed by the CLI.
4. Start a new agent session and use the plugin.

[Browse 2,500+ plugins](https://777genius.github.io/universal-agent-plugins/plugins/)

## What it does

- installs one plugin in one or several supported agents;
- updates, repairs, and removes only files it manages;
- converts the same Agent Plugins 1.0 package into each agent's native format;
- keeps activation and OAuth prompts visible to you.

Supported clients include Codex, ChatGPT, Cursor, GitHub Copilot CLI, VS Code,
Kiro, Claude Code, Gemini CLI, OpenCode, Cline, and Windsurf. Compatibility is
package-specific, and the CLI tells you what is installed automatically and
what still needs an activation step.

## Install any Agent Plugins 1.0 package

Short names come from the signed Universal Agent Plugins Registry. You can also
install a valid package directly from a local directory or a pinned GitHub
source without submitting it to the registry.

The standard `plugin.json` is the installation authority. `plugin.yaml` remains
legacy authoring input and cannot override `plugin.json`.

## More commands

For normal interactive use, omit `--target` and choose agents in the prompt.
Use `--target` in scripts, CI, or whenever you want to name clients explicitly.

```bash
# Find and inspect plugins
npx universal-agent-plugins search docs
npx universal-agent-plugins info context7

# Install in specific agents
npx universal-agent-plugins add context7 --target codex,cursor,kiro

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

Remote installs require a full 40-character commit SHA. Branches, tags, and
abbreviated SHAs are rejected. When no package path is given, the CLI uses a
valid root `plugin.json` or auto-selects the only valid nested package that has
`mcp.json` or `skills/`. If several packages match, it lists them and asks for
an explicit `//path`. Repositories with more than 16 possible packages require
an explicit path before candidate packages are fetched. The selected canonical
path and package digest are stored for safe replay. Direct full-SHA installations
remain immutable; use `switch` to move to another exact source. `repair` reapplies
the recorded source, and `remove` changes only files owned by the CLI.

## Safety and verification

Every operation validates the package and preflights all selected agents before
changing managed files. Multi-agent failures roll back the CLI-owned changes or
stop with a repair instruction. `--dry-run` prints the plan without writing.
The CLI does not execute packages during search and sends no installation
telemetry.

The npm package has no `postinstall`. On first execution it downloads the exact
versioned Go binary for macOS, Linux, or Windows, verifies its embedded SHA-256,
and caches it locally. Native release tests cover x64 and arm64 on all three
operating systems.

- [Source and documentation](https://github.com/777genius/universal-agent-plugins)
- [Browse the plugin catalog](https://777genius.github.io/universal-agent-plugins/plugins/)
- [Client E2E evidence](https://github.com/777genius/universal-agent-plugins/blob/main/docs/AGENTPLUGINS_CLIENT_E2E.md)

Universal Agent Plugins is an independent community project. It is not
affiliated with OpenAI, Agent Plugins, or the supported client vendors.
