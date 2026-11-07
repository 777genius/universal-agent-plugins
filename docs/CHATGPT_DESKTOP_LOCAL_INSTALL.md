# Install UAP plugins in ChatGPT Desktop (local)

ChatGPT Desktop's local Work/Codex plugin marketplace is distinct from **ChatGPT Web** and from the remote ChatGPT Apps / MCP connections. Universal Agent Plugins (UAP) exposes **`--target chatgpt-desktop`**, a user-facing alias routed through the **existing Codex CLI target** for local Desktop plugins. This does not create a second native installation. It does not register a ChatGPT cloud app or grant workspace permissions.

## Install a compatible plugin

1. Install ChatGPT Desktop with local Work/Codex support and, for automatic native registration, the supported Codex CLI. You need to sign in to the desktop app yourself.
2. In the [UAP plugin directory](https://universal-agent-plugins.com/plugins/), open an eligible plugin and choose **ChatGPT Desktop** in the agent selector. The optional Desktop guidance then appears below the command. This option is enabled only for a current, installable **Codex user-scope** release; the separately listed **ChatGPT Web** option requires a verified remote app binding.
3. Copy the exact command from that plugin's card. For example, an eligible Context7 release may use:

   `npx -y universal-agent-plugins add context7 --target chatgpt-desktop`

   UAP selects and validates the signed distribution and performs its existing managed Codex marketplace/plugin lifecycle. The command may show authentication or other activation steps. If the Codex CLI is unavailable, UAP prints manual setup rather than claiming an automatic installation.
4. On macOS or Windows, you can run `codex app` to open an installed ChatGPT Desktop app (or start its installer when missing). On Windows it may print a path to open. This is a user action, not performed automatically by UAP. If the CLI prints a `codex://plugins/...` link, open it **only on your own computer**. It opens the local plugin detail in Desktop, not a silent installation. If the URL handler is unavailable, open ChatGPT Desktop manually.
5. **If the CLI verified the plugin as installed and enabled, do not install it again in Desktop.** Restart or refresh ChatGPT Desktop if needed, find the plugin in **Plugins > Personal**, and start a new **local** Work/Codex thread to test its Skill or MCP tools. If the native CLI was unavailable or its listing was inconclusive, finish installation in Desktop instead. Complete any provider OAuth inside the client.

**What success means:** With a supported CLI, UAP already performs the native marketplace registration, plugin installation and `codex plugin list --json` verification automatically. The installed local copy persists across CLI processes in the selected Codex profile. This is not the same as proving ChatGPT Desktop loaded the plugin, that the MCP server ran, or that OAuth completed. A custom `CODEX_HOME` may point to a profile Desktop does not use; UAP never silently rewrites another profile. Desktop runtime verification requires step 5 on a real installed, signed-in app.

## Maintain or remove

Use the same UAP command family and target, not a second Desktop-specific installer:

`npx -y universal-agent-plugins list`

This lists UAP-managed installations; for native Codex registration independently run `codex plugin list --json`. Neither output certifies that the signed-in Desktop app executed the plugin.

`npx -y universal-agent-plugins update <plugin-name> --target chatgpt-desktop`

`npx -y universal-agent-plugins repair <plugin-name> --target chatgpt-desktop`

`npx -y universal-agent-plugins remove <plugin-name> --target chatgpt-desktop`

The older `--target codex` spelling remains valid and uses the same native plugin installation; do not run both variants to install twice. The `chatgpt-desktop` alias is available **only after the CLI release containing this change**. Before that release, use `--target codex` and verify the installed plugin in Desktop. Follow the CLI's exact manual uninstall/confirmation instructions when requested. UAP only removes its own managed marketplace entries and files; it must never overwrite another user's or organization's marketplace configuration.

## Troubleshooting

- **No ChatGPT Desktop on this computer?** On macOS and Windows, `codex app` opens the installed app or starts the installer. Codex CLI verification alone does not prove Desktop activation; UAP does **not** auto-launch the app.
- **No Codex CLI?** UAP can prepare an eligible package but may require installing the CLI or completing the registration in the app. Do not change files under `~/.codex` or `~/.agents` by hand to bypass ownership checks.
- **Plugin not visible?** Confirm you are in the **local** Plugins directory and that the selected workspace/profile permits personal marketplaces. Check `echo "${CODEX_HOME:-$HOME/.codex}"` and `codex plugin list --json`; a custom `CODEX_HOME` may install into a different profile. Restart Desktop if needed. Do not copy or overwrite app-owned configuration by hand. Use the optional generated deep link only if it refers to your own managed package.
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
