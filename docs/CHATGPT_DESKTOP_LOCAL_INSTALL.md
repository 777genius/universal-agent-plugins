# Install UAP plugins in ChatGPT Desktop (local)

ChatGPT Desktop's local Work/Codex plugin marketplace is distinct from **ChatGPT Web** and from the remote ChatGPT Apps / MCP connections. Universal Agent Plugins (UAP) uses the **existing Codex CLI target** for local Desktop plugins. It does not register a ChatGPT cloud app or grant workspace permissions.

## Install a compatible plugin

1. Install ChatGPT Desktop with local Work/Codex support and, for automatic native registration, the supported Codex CLI. You need to sign in to the desktop app yourself.
2. In the [UAP plugin directory](https://universal-agent-plugins.com/plugins/), open an eligible plugin and expand **ChatGPT Desktop (local)**. This option only appears for a current, installable **Codex user-scope** release.
3. Copy the exact command from that plugin's card. For example, an eligible Context7 release may use:

   `npx -y universal-agent-plugins add context7 --target codex`

   UAP selects and validates the signed distribution and performs its existing managed Codex marketplace/plugin lifecycle. The command may show authentication or other activation steps. If the Codex CLI is unavailable, UAP prints manual setup rather than claiming an automatic installation.
4. On macOS or Windows, you can run `codex app` to open an installed ChatGPT Desktop app (or start its installer when missing). On Windows it may print a path to open. This is a user action, not performed automatically by UAP. If the CLI prints a `codex://plugins/...` link, open it **only on your own computer**. It opens the local plugin detail in Desktop, not a silent installation. If the URL handler is unavailable, open ChatGPT Desktop manually.
5. In **Plugins > Personal**, find the plugin from the local marketplace, install or enable it as prompted, and restart ChatGPT Desktop if required. Start a new **local** Work/Codex thread and check that the plugin's Skill or MCP tools function. Complete any provider OAuth inside the client.

**What success means:** `codex plugin list --json` can verify local **Codex** registration and enablement. It does not prove that ChatGPT Desktop loaded the plugin, that the MCP server ran, or that the user's account completed OAuth. Desktop runtime verification requires step 5 on a real installed, signed-in app.

## Maintain or remove

Use the same UAP command family and target, not a second Desktop-specific installer:

`npx -y universal-agent-plugins update <plugin-name> --target codex`

`npx -y universal-agent-plugins repair <plugin-name> --target codex`

`npx -y universal-agent-plugins remove <plugin-name> --target codex`

Follow the CLI's exact manual uninstall/confirmation instructions when requested. UAP only removes its own managed marketplace entries and files; it must never overwrite another user's or organization's marketplace configuration.

## Troubleshooting

- **No ChatGPT Desktop on this computer?** On macOS and Windows, `codex app` opens the installed app or starts the installer. Codex CLI verification alone does not prove Desktop activation; UAP does **not** auto-launch the app.
- **No Codex CLI?** UAP can prepare an eligible package but may require installing the CLI or completing the registration in the app. Do not change files under `~/.codex` or `~/.agents` by hand to bypass ownership checks.
- **Plugin not visible?** Confirm you are in the **local** Plugins directory, that the selected workspace/profile permits personal marketplaces, and restart the Desktop app. Use the optional generated deep link only if it refers to your own managed package.
- **ChatGPT cloud remains disabled in the UAP picker?** That is intentional when no verified remote `app_binding` exists. A local Codex marketplace is **not** a cloud ChatGPT app registration.
- **Need a project installation?** Check the plugin's separate native Codex project compatibility first. This guide covers **This computer** only. ChatGPT cloud project installs remain unsupported.

## Organization and team distribution

An organization administrator can import and sync a **GitHub-backed marketplace** through ChatGPT workspace admin settings, then approve policies and role access. That administrative workflow is different from a personal local marketplace. Prepare compatible repositories using the existing UAP **publication** command (which already emits `.agents/plugins/marketplace.json`), then follow the official ChatGPT admin import. Initial import, trust, policy, sign-in, external provider consent and role management remain the administrator's or user's actions; the local CLI does not mutate enterprise workspace settings.

## Verification and limits

Automated CI can check marketplace JSON identity, path containment, URL encoding, native CLI process contracts and browser UX without ChatGPT Desktop. It cannot certify the native Desktop runtime without a signed-in real app on a physical host. Do not treat a fixture or a skipped native check as successful Desktop E2E.

Official references:
- https://developers.openai.com/plugins/build/plugins
- https://learn.chatgpt.com/docs/app/commands
- https://learn.chatgpt.com/docs/cli/reference
- https://help.openai.com/en/articles/20001504-importing-and-syncing-plugin-marketplaces-from-github
