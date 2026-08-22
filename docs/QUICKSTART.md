# Client quick start

Install one Agent Plugins 1.0 package, not the whole Directory. You need Node.js
22 or newer:

```bash
npx universal-agent-plugins add cloudflare-docs --target codex,cursor,kiro
```

The CLI shows the exact package, source, and complete target plan before
changing anything. Explicit commands and targets are consent; there is no
hidden `--yes` flag. In an interactive terminal you can use the accessible
multiselect, or name targets directly:

```bash
npx universal-agent-plugins add cloudflare-docs --target cursor
npx universal-agent-plugins add cloudflare-docs --target codex,cursor
```

Comma-separated targets use one resolved package. Complete preflight happens
before mutation. Managed commit failures roll back safely owned changes;
external activation failures keep valid preparation and return a non-zero
per-target result with an exact repair step.

Supported targets:

| Target | What the CLI does | Remaining user step |
| --- | --- | --- |
| `codex` | Generates a personal OpenAI marketplace package | Runs no hidden UI actions; prints exact Codex activation steps |
| `chatgpt` | Prepares a projected package only when the selected release has a verified app binding | Install or select the registered personal app manually in ChatGPT Plugins, then start a new chat |
| `cursor` | Places the native package in Cursor's local plugin directory | Reload Cursor, then verify the plugin appears |
| `copilot` | Registers a managed marketplace, installs, and verifies through Copilot CLI | Nothing when successful |
| `vscode` | Installs automatically through Copilot CLI when available | Otherwise prints the exact `chat.pluginLocations` setting |
| `kiro` | Prepares the native package folder | Prints the exact **Powers -> Add Custom Power -> Import** steps and folder |

Lifecycle commands use the same explicit target or comma-separated targets:

```bash
npx universal-agent-plugins info cloudflare-docs
npx universal-agent-plugins doctor cloudflare-docs
npx universal-agent-plugins update cloudflare-docs --target cursor
npx universal-agent-plugins repair cloudflare-docs --target cursor
npx universal-agent-plugins remove cloudflare-docs --target cursor
npx universal-agent-plugins update cloudflare-docs --target codex,cursor
```

Switching source is deliberate. For example, move an existing Cloudflare Docs
installation from its bridge to the qualified community distribution with:

```bash
npx universal-agent-plugins switch cloudflare-docs --to 777genius/cloudflare-docs
```

`prepared`, `auth_pending`, and `manual_activation_required` are not reported as
installed. OAuth stays inside the client; the CLI never stores tokens or accepts
trust prompts automatically.

## Install a package outside the Directory

Any valid Agent Plugins 1.0 package can be installed directly. Use a local
folder while developing it, or pin a GitHub source to a full commit SHA:

```bash
npx universal-agent-plugins add ./my-plugin --target cursor
npx universal-agent-plugins add \
  777genius/universal-agent-plugins@2ddbb99dd190c1792b79904f9875e6322bccd243//plugins/cloudflare-docs \
  --target cursor
```

Directory membership is needed only for a reviewed short name such as `cloudflare-docs`;
it is not required for installation. Review an external package's skills, MCP
servers, hooks, permissions, and source before enabling it.

Directory source labels are provenance, not endorsements: `upstream` is a
complete package pinned in the upstream owner's repository; `community bridge`
is a reproducible community package built from pinned upstream content plus a
reviewed overlay; and `community` is independently community-authored or
packaged. Full-SHA GitHub references and local paths are `direct source`
installs that bypass Directory source selection. Community packages and bridges
are not official vendor packages.

Cloudflare Docs is currently the only Directory release with a verified ChatGPT
app binding:

```bash
npx universal-agent-plugins add cloudflare-docs --target chatgpt
```

This prepares and validates the package; it does not silently install or attest
the ChatGPT UI step. The registered development app passed personal-app
discovery, chat activation, and read-only runtime, but its availability remains
account/workspace-specific. Follow the printed Plugins UI step and verify it in
a new chat. The five stdio MCP packages stay Codex-only.

The portable package can also be installed through a client's native Agent
Plugins flow. Exact client/runtime/OAuth evidence is kept separately in the
[test matrix](TEST_MATRIX.md).
