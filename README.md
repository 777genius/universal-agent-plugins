![Universal Agent Plugins](assets/hero.png)

# Universal Agent Plugins

[![Validate](https://github.com/777genius/universal-agent-plugins/actions/workflows/validate.yml/badge.svg)](https://github.com/777genius/universal-agent-plugins/actions/workflows/validate.yml)
[![Live E2E](https://github.com/777genius/universal-agent-plugins/actions/workflows/live-e2e.yml/badge.svg)](https://github.com/777genius/universal-agent-plugins/actions/workflows/live-e2e.yml)
[![Agent Plugins 1.0](https://img.shields.io/badge/Agent%20Plugins-1.0.0-7257FF)](https://agent-plugins.org/specification)
[![License](https://img.shields.io/badge/license-Apache--2.0-20A4C8)](LICENSE)

Give your AI agent ready-made abilities: search current documentation, navigate
code, debug browsers, work with cloud tools, and more. Pick one plugin and add
others only when you need them.

This repository contains 26 open-source plugins packaged for the
[Agent Plugins 1.0](https://agent-plugins.org/specification) standard.
Portable packages use a root `plugin.json`. For Codex and ChatGPT, CI generates
official-layout `.codex-plugin/plugin.json` packages under
[`compat/openai`](compat/openai), validates them with OpenAI's `plugin-creator`,
and follows the [OpenAI plugin build guide](https://developers.openai.com/plugins/build/plugins).
The installer below is a community CLI, not an OpenAI product.

## Try one plugin

Context7 is an easy first choice. It finds current library documentation and
requires no account. You need Node.js 22 or newer:

```bash
npx universal-agent-plugins add context7
```

The CLI detects Codex/ChatGPT, Cursor, GitHub Copilot/VS Code, and Kiro. If more
than one is present, choose one from the prompt. To choose directly:

```bash
npx universal-agent-plugins add context7 --target cursor
```

Open a new chat or session in the client you selected and ask:

```text
Use Context7 to find the current Playwright quick start and summarize it with source links.
```

That's it. Every plugin is independent, so you never need to install the whole
catalog or follow a chain of plugins. Client activation and OAuth can still
require a visible confirmation; see the short [client setup guide](docs/QUICKSTART.md).

## All plugins

| Plugins |  |  |
| --- | --- | --- |
| <img src="assets/icon.png" width="20" height="20" alt=""> [Agent Code Navigator](plugins/agent-code-navigator) | <img src="assets/plugin-icons/atlassian.svg" width="20" height="20" alt=""> [Atlassian](plugins/atlassian) | <img src="assets/plugin-icons/googlechrome.svg" width="20" height="20" alt=""> [Chrome DevTools](plugins/chrome-devtools) |
| <img src="assets/plugin-icons/cloudflare.svg" width="20" height="20" alt=""> [Cloudflare](plugins/cloudflare) | <img src="assets/plugin-icons/cloudflare.svg" width="20" height="20" alt=""> [Cloudflare Bindings](plugins/cloudflare-bindings) | <img src="assets/plugin-icons/cloudflare.svg" width="20" height="20" alt=""> [Cloudflare Docs](plugins/cloudflare-docs) |
| <img src="assets/plugin-icons/cloudflare.svg" width="20" height="20" alt=""> [Cloudflare Observability](plugins/cloudflare-observability) | <img src="assets/plugin-icons/cloudflare.svg" width="20" height="20" alt=""> [Cloudflare Radar](plugins/cloudflare-radar) | <img src="assets/plugin-icons/context7.png" width="20" height="20" alt=""> [Context7](plugins/context7) |
| <img src="assets/plugin-icons/docker.svg" width="20" height="20" alt=""> [Docker Hub](plugins/docker-hub) | <img src="assets/plugin-icons/figma.svg" width="20" height="20" alt=""> [Figma](plugins/figma) | <img src="assets/plugin-icons/firebase.svg" width="20" height="20" alt=""> [Firebase](plugins/firebase) |
| <img src="assets/plugin-icons/github.svg" width="20" height="20" alt=""> [GitHub](plugins/github) | <img src="assets/plugin-icons/gitlab.svg" width="20" height="20" alt=""> [GitLab](plugins/gitlab) | <img src="assets/plugin-icons/greptile.png" width="20" height="20" alt=""> [Greptile](plugins/greptile) |
| <img src="assets/plugin-icons/heroku.png" width="20" height="20" alt=""> [Heroku](plugins/heroku) | <img src="assets/plugin-icons/hubspot.svg" width="20" height="20" alt=""> [HubSpot CRM](plugins/hubspot-crm) | <img src="assets/plugin-icons/hubspot.svg" width="20" height="20" alt=""> [HubSpot Developer](plugins/hubspot-developer) |
| <img src="assets/plugin-icons/linear.svg" width="20" height="20" alt=""> [Linear](plugins/linear) | <img src="assets/plugin-icons/neon.svg" width="20" height="20" alt=""> [Neon](plugins/neon) | <img src="assets/plugin-icons/notion.svg" width="20" height="20" alt=""> [Notion](plugins/notion) |
| <img src="assets/plugin-icons/sentry.svg" width="20" height="20" alt=""> [Sentry](plugins/sentry) | <img src="assets/plugin-icons/statsig.png" width="20" height="20" alt=""> [Statsig](plugins/statsig) | <img src="assets/plugin-icons/stripe.svg" width="20" height="20" alt=""> [Stripe](plugins/stripe) |
| <img src="assets/plugin-icons/supabase.svg" width="20" height="20" alt=""> [Supabase](plugins/supabase) | <img src="assets/plugin-icons/vercel.svg" width="20" height="20" alt=""> [Vercel](plugins/vercel) |  |

Each one installs separately. See [plugins to try first](docs/HERO_PLUGINS.md)
for copy-ready examples. Exact authentication and test status are in the
[test matrix](docs/TEST_MATRIX.md).

## Use them with your agent

Agent Plugins 1.0 gives every package a shared structure. Compatible clients can
reuse the parts they support, while installation, permissions, and OAuth remain
client-specific.

| Client | Delivery | Activation |
| --- | --- | --- |
| Codex / ChatGPT | OpenAI compatibility package | Follow the exact CLI or in-app install hint |
| Cursor | Native Agent Plugin | Reload, then verify discovery |
| GitHub Copilot CLI | Native plugin + managed marketplace | Installed and verified automatically |
| VS Code | Shared Copilot plugin when its CLI is available | Automatic, otherwise the exact setting is shown |
| Kiro | Native folder package | Follow the exact Power import hint |

All 26 packages pass the standard schemas. That does not mean every service or
OAuth flow has been tested in every client, and the standard is not a universal
marketplace. See the [test matrix](docs/TEST_MATRIX.md) for exact results and the
[compatibility guide](docs/COMPATIBILITY.md) before connecting a private service.

Package lifecycle proof is broader than runtime proof: all 26 pass isolated
materialization/removal, and the five starter plugins pass 25/25 add/remove
flows across Codex, Cursor, Copilot, VS Code, and Kiro projections. The new
`agentplugins 0.1.5` CI covers package lifecycle and projections, not client
process, tool, or OAuth runtime. Separately, audited interactive evidence proves
15/15 real runtime checks across Codex, Cursor, and Kiro, including authenticated
read-only Notion calls in all three. A separate sanitized Codex check also
passed Figma OAuth and read-only `whoami`; it does not cover other clients.
Post-merge live run
[`31332320890`](https://github.com/777genius/universal-agent-plugins/actions/runs/31332320890)
also proves native Copilot 0.1.5 install/list/remove lifecycle for the five
starter plugins; it does not prove Copilot tool runtime or OAuth.

## Safety

- Review a plugin's tools and scopes before enabling it.
- Start with read-only tasks, especially after OAuth.
- Never place tokens in `plugin.json`, `mcp.json`, or committed headers.
- A valid package can still expose destructive tools.

See [SECURITY.md](SECURITY.md) for reporting and security boundaries.

## About this project

This repository rebuilds the portable subset of
[`universal-plugins-for-ai-agents`](https://github.com/777genius/universal-plugins-for-ai-agents)
without `plugin-kit-ai` as its authoring layer. Contributions are welcome; see
[CONTRIBUTING.md](CONTRIBUTING.md).

This is an independent community project maintained by 777genius. It is not
affiliated with or endorsed by OpenAI or the vendors represented in the catalog.
Original project material is licensed under [Apache 2.0](LICENSE).
