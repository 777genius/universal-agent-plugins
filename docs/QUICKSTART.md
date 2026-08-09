# Client quick start

Install one Agent Plugins 1.0 package, not the whole catalog. You need Node.js
22 or newer:

```bash
npx universal-agent-plugins add context7
```

The CLI shows the exact package plan and asks before changing anything. If it
finds several clients, select one. You can also name the target directly:

```bash
npx universal-agent-plugins add context7 --target cursor
```

Supported targets:

| Target | What the CLI does | Remaining user step |
| --- | --- | --- |
| `codex` | Generates a personal OpenAI marketplace package | Runs no hidden UI actions; prints exact Codex CLI and ChatGPT/Codex app steps |
| `cursor` | Places the native package in Cursor's local plugin directory | Reload Cursor, then verify the plugin appears |
| `copilot` | Registers a managed marketplace, installs, and verifies through Copilot CLI | Nothing when successful |
| `vscode` | Installs automatically through Copilot CLI when available | Otherwise prints the exact `chat.pluginLocations` setting |
| `kiro` | Prepares the native package folder | Prints the exact **Powers -> Add Custom Power -> Import** steps and folder |

Lifecycle commands use the same explicit target:

```bash
npx universal-agent-plugins info context7
npx universal-agent-plugins doctor context7
npx universal-agent-plugins update context7 --target cursor
npx universal-agent-plugins remove context7 --target cursor
```

`prepared`, `auth_pending`, and `manual_activation_required` are not reported as
installed. OAuth stays inside the client; the CLI never stores tokens or accepts
trust prompts automatically.

The portable package can also be installed through a client's native Agent
Plugins flow. Exact client/runtime/OAuth evidence is kept separately in the
[test matrix](TEST_MATRIX.md).
