---
title: "Installation"
description: "Install plugin-kit-ai using supported channels."
canonicalId: "page:guide:installation"
section: "guide"
locale: "en"
generated: false
translationRequired: true
---

# Installation

Use `npx` for the fastest first plugin install. Use Homebrew when you want plugin-kit-ai installed for daily work.

## Fastest First Plugin Install

This is an optional zero-repo proof that the published install flow is live.
It does not create the plugin repo you will edit.

```bash
npx plugin-kit-ai@latest add notion
```

- This installs every supported output for that plugin.
- If your goal is to author your own plugin repo, continue to Quickstart and start with `plugin-kit-ai init ...`.

## Supported Channels

- Homebrew for the cleanest default CLI path.
- npm when your environment is already centered around npm.
- PyPI / pipx when your environment is already centered around Python.
- Verified install script as the fallback path.

## Recommended Commands

### Homebrew

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai version
```

### npm

```bash
npm i -g plugin-kit-ai
plugin-kit-ai version
```

### PyPI / pipx

```bash
pipx install plugin-kit-ai
plugin-kit-ai version
```

### Verified Script

```bash
curl -fsSL https://raw.githubusercontent.com/777genius/plugin-kit-ai/main/scripts/install.sh | sh
plugin-kit-ai version
```

To install the CLI and preview a real universal plugin without Node/npm:

```bash
curl -fsSL https://raw.githubusercontent.com/777genius/plugin-kit-ai/main/scripts/install.sh | sh -s -- add notion --dry-run
```

## Which One Should Most People Use?

- Use `npx` when you want the shortest first run and do not want a permanent install yet.
- Use Homebrew if you are on macOS and want the smoothest daily-use path.
- Use npm or pipx only when that already matches your team environment.
- Use the verified script when you need a fallback outside package-manager-first setups, including one-shot plugin commands through `sh -s -- ...`.

## After Install

Most people should continue straight to [Quickstart](/en/guide/quickstart), try a real plugin first, then create the first repo on the job-first path that matches the work.

If you chose `pipx` because your team is Python-first and you already know you want the Python path, continue with [Build A Python Runtime Plugin](/en/guide/python-runtime).

## CI Install Path

For CI, prefer the dedicated setup action instead of teaching every workflow how to download the CLI manually.

## Important Boundary

The npm and PyPI packages are install channels for the CLI. They are not runtime APIs and they are not SDKs.

See [Reference > Install Channels](/en/reference/install-channels) for the contract boundary.
