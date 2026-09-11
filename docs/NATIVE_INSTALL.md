# Native CLI installation

`agentplugins` is a Go binary. The native installation path does not require
Node.js, Python, npm, or pip.

## Homebrew on macOS and Linux

```bash
brew install 777genius/agentplugins/agentplugins
agentplugins add context7
```

The tap installs the same platform-specific binary published in the signed
GitHub release. Its formula pins the release URL and SHA-256 for every supported
macOS and Linux architecture.

## macOS and Linux

Install the latest published release and immediately run a command:

```bash
curl -fsSL https://raw.githubusercontent.com/777genius/universal-agent-plugins/main/install.sh \
  | sh -s -- add context7
```

The default destination is `$HOME/.local/bin/agentplugins`. Set an explicit
destination or version by downloading the installer first. Environment
assignments before a pipeline are not portable to every shell, so this form is
deliberately explicit:

```bash
curl -fsSLo /tmp/agentplugins-install.sh \
  https://raw.githubusercontent.com/777genius/universal-agent-plugins/main/install.sh
AGENTPLUGINS_VERSION=0.1.44 AGENTPLUGINS_BIN_DIR="$HOME/bin" \
  sh /tmp/agentplugins-install.sh
```

## Windows PowerShell

```powershell
irm https://raw.githubusercontent.com/777genius/universal-agent-plugins/main/install.ps1 | iex
& "$HOME\.local\bin\agentplugins.exe" add context7
```

Use the script directly for an explicit version or destination:

```powershell
Invoke-WebRequest `
  https://raw.githubusercontent.com/777genius/universal-agent-plugins/main/install.ps1 `
  -OutFile "$env:TEMP\agentplugins-install.ps1"
& "$env:TEMP\agentplugins-install.ps1" `
  -Version 0.1.44 `
  -BinDir "$HOME\bin"
```

## What the installer verifies

The installer:

1. accepts only stable `agentplugins-vX.Y.Z` release tags;
2. selects the exact asset for the detected OS and architecture;
3. requires exactly one matching entry in the release `checksums.txt`;
4. verifies the downloaded binary's SHA-256;
5. executes only `agentplugins version` and requires the exact release version;
6. atomically replaces the destination only after all checks pass.

Published release assets also carry GitHub artifact attestations. The release
pipeline verifies those attestations independently before publishing the npm
facade. The lightweight native installer intentionally requires only `curl`
plus a standard SHA-256 utility on Unix, or PowerShell on Windows.

## Pinning and automation

Use `AGENTPLUGINS_VERSION` to avoid resolving `latest`:

```bash
AGENTPLUGINS_VERSION=0.1.44 sh ./install.sh
```

Automation can set `AGENTPLUGINS_OUTPUT_FILE`. The installer appends the exact
version, tag, path, asset name, and verified SHA-256 as `key=value` records.

Mirrors and GitHub Enterprise can override the repository, API base, and
release host with `AGENTPLUGINS_REPOSITORY`, `AGENTPLUGINS_API_BASE`, and
`AGENTPLUGINS_RELEASE_BASE_URL`.

## Runtime requirements belong to plugins

The manager itself has no Node.js dependency when installed natively. A plugin
may still require Node.js, Python, a browser, or another executable. Those are
package runtime requirements, not CLI installation requirements, and are
reported during planning before the plugin is changed on disk.

The npm facade remains available for users who prefer `npx`:

```bash
npx universal-agent-plugins add context7
```

## After installation

If `agentplugins` is not found, add the installation directory to `PATH` as
printed by the installer, or invoke the binary by its full path. The one-shot
Unix command above already invokes the installed binary by its full path.

Follow the CLI's activation or sign-in instructions, then start a new agent
session. Installation does not by itself prove activation or plugin runtime.
See the [client compatibility evidence](https://777genius.github.io/universal-agent-plugins/docs/en/reference/client-compatibility.html)
for tested versions and platform limitations, or [installer support](./SUPPORT.md#universal-agent-plugins-installer)
if you need help.
