![Universal Agent Plugins](assets/hero.png)

# Universal Agent Plugins

[![Validate](https://github.com/777genius/universal-agent-plugins/actions/workflows/validate.yml/badge.svg)](https://github.com/777genius/universal-agent-plugins/actions/workflows/validate.yml)
[![Live E2E](https://github.com/777genius/universal-agent-plugins/actions/workflows/live-e2e.yml/badge.svg)](https://github.com/777genius/universal-agent-plugins/actions/workflows/live-e2e.yml)
[![Agent Plugins 1.0](https://img.shields.io/badge/Agent%20Plugins-1.0.0-7257FF)](https://agent-plugins.org/specification)
[![License](https://img.shields.io/badge/license-Apache--2.0-20A4C8)](LICENSE)

**One CLI can add, update, remove, repair, and deliberately switch an Agent
Plugins 1.0 package across one or several explicitly selected supported
clients.** A prepared package, manual activation, runtime check, and OAuth are
reported as separate outcomes—never collapsed into “installed.”

Pick from 26 ready-made abilities: search current documentation, navigate code,
debug browsers, work with cloud tools, and more. Install one plugin and add
others only when you need them.

This repository contains 26 open-source plugins packaged for the
[Agent Plugins 1.0](https://agent-plugins.org/specification) standard.
Portable packages use a root `plugin.json`. For OpenAI hosts, CI generates
official-layout `.codex-plugin/plugin.json` packages under
[`compat/openai`](compat/openai), validates them with OpenAI's `plugin-creator`,
and follows the [OpenAI plugin build guide](https://developers.openai.com/plugins/build/plugins).
The installer below is a community CLI, not an OpenAI product.

The installer is open source and maintained in
[`777genius/plugin-kit-ai`](https://github.com/777genius/plugin-kit-ai).
`universal-agent-plugins` is the npm package; `agentplugins` is the installed
command. Its shared Go engine reads standard `plugin.json` packages and uses
client-specific adapters for Codex, ChatGPT, Cursor, GitHub Copilot/VS Code,
and Kiro.

Browse the [plugin directory](https://777genius.github.io/universal-agent-plugins/)
or [submit a plugin](registry/README.md#submit-an-external-package) through a Git-native
pull request. This community directory is not an official OpenAI registry.

## Try one plugin

Cloudflare Docs is an easy first choice. It searches current Cloudflare
documentation and requires no account. You need Node.js 22 or newer:

```bash
npx universal-agent-plugins add cloudflare-docs --target codex,cursor,kiro
```

The npm launcher requires Node.js 22+. Pass every target explicitly in
non-interactive use as one comma-separated value:

```bash
npx universal-agent-plugins add cloudflare-docs --target cursor
npx universal-agent-plugins add cloudflare-docs --target codex,cursor
```

The CLI resolves and verifies one immutable package once, preflights the whole
target set, and then reports each client outcome. It never mixes package sources
between clients in one operation.

The same CLI manages the rest of the plugin lifecycle:

```bash
npx universal-agent-plugins update cloudflare-docs --target cursor
npx universal-agent-plugins repair cloudflare-docs --target cursor
npx universal-agent-plugins remove cloudflare-docs --target cursor
```

Switching source is deliberate. For example, an existing Cloudflare Docs
installation can move from its bridge to the qualified community distribution:

```bash
npx universal-agent-plugins switch cloudflare-docs --to 777genius/cloudflare-docs
```

Open a new chat or session in the client you selected and ask:

```text
Use Cloudflare Docs to explain the current Workers environment variable and secret storage guidance with source links.
```

That's it. Every plugin is independent, so you never need to install the whole
Directory or follow a chain of plugins. Client activation and OAuth can still
require a visible confirmation; see the short [client setup guide](docs/QUICKSTART.md).

## Install any Agent Plugin

The CLI is not limited to this Directory. Install any valid Agent Plugins 1.0
package from a local directory or an immutable GitHub revision:

```bash
npx universal-agent-plugins add ./my-plugin --target cursor
npx universal-agent-plugins add \
  777genius/universal-agent-plugins@2ddbb99dd190c1792b79904f9875e6322bccd243//plugins/cloudflare-docs \
  --target cursor
```

The package can use the portable root `plugin.json` layout or the official
`.codex-plugin/plugin.json` layout with its declared sidecars. Pin GitHub
sources to a full commit SHA so every install is reproducible. Short names such
as `cloudflare-docs` resolve through this repository's reviewed Directory; external
packages do not need to be copied into it.

Directory source labels describe provenance, not endorsement. **Upstream**
means the complete package is pinned in its upstream owner's repository;
**community bridge** means a community-built package reproducibly combines
pinned upstream content with a reviewed overlay; and **community** means an
independently community-authored or packaged distribution. A full-SHA GitHub
reference or local path is a **direct source** that bypasses Directory source
selection. Community and community-bridge packages are not official vendor
packages.

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
| <img src="assets/client-icons/openai.svg" width="20" height="20" alt=""> Codex | Official-layout `.codex-plugin` package | CLI prints the exact activation steps |
| <img src="assets/client-icons/openai.svg" width="20" height="20" alt=""> ChatGPT | Only a release with a registered app binding | Manual Plugins UI and OAuth consent; disabled when no binding exists |
| <img src="assets/client-icons/cursor.svg" width="20" height="20" alt=""> Cursor | Native Agent Plugin | Reload, then verify discovery |
| <img src="assets/client-icons/github-copilot.svg" width="20" height="20" alt=""> GitHub Copilot CLI | Native plugin + managed marketplace | Installed and verified automatically |
| <img src="assets/client-icons/vscode.svg" width="20" height="20" alt=""> VS Code | Shared Copilot plugin when its CLI is available | Automatic; otherwise the exact setting is shown |
| <img src="assets/client-icons/kiro.svg" width="20" height="20" alt=""> Kiro | Native folder package | Follow the exact Power import hint |

All 26 packages pass standard schema validation; that is schema-only evidence.
Historical evidence includes 15/15 runtime checks for five starter packages
across Codex, Cursor, and Kiro, with Notion OAuth tested in those three clients;
Figma OAuth was tested separately in Codex only. A materialized or installed
package does not by itself prove activation, tool runtime, or OAuth. ChatGPT and
Copilot claims are narrower and are not generalized from other clients.

Current Directory identity comes from
[`registry/directory.json`](registry/directory.json), not copied release IDs or
digests in this launch overview. Installation coverage is broader than runtime
coverage, and the standard itself is not a universal marketplace. See the
[test matrix](docs/TEST_MATRIX.md), [verification report](docs/VERIFICATION.md),
and [compatibility guide](docs/COMPATIBILITY.md) for the exact schema-only,
materialized, runtime-tested, OAuth-tested, read-only, and not-proven boundaries.

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
affiliated with or endorsed by OpenAI or the vendors represented in the Directory.
Original project material is licensed under [Apache 2.0](LICENSE).
