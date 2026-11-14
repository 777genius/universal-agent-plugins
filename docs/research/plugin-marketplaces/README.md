# Codex, Claude, And Gemini Publication Research

Research date: 2026-04-04

This note records only facts confirmed by official vendor docs plus a short project-local implication section for `plugin-kit-ai`.

## Sources

- OpenAI Codex:
  - [Build plugins](https://developers.openai.com/codex/plugins/build)
  - [Plugins overview](https://developers.openai.com/codex/plugins/)
- Anthropic Claude Code:
  - [Create and distribute a plugin marketplace](https://code.claude.com/docs/en/plugin-marketplaces)
  - [Discover and install prebuilt plugins through marketplaces](https://code.claude.com/docs/en/discover-plugins)
- Gemini CLI:
  - [Gemini CLI extensions](https://geminicli.com/docs/extensions/)
  - [Release extensions](https://geminicli.com/docs/extensions/releasing/)
  - [Extension best practices](https://geminicli.com/docs/extensions/best-practices/)
  - [Gemini CLI extension gallery](https://geminicli.com/extensions/)

## Codex Marketplace

### What the marketplace is

OpenAI documents a Codex marketplace as a JSON catalog that Codex can read and install from. The official `Build plugins` page explicitly calls out:

- repo marketplace at `$REPO_ROOT/.agents/plugins/marketplace.json`
- personal marketplace at `~/.agents/plugins/marketplace.json`
- curated marketplace behind the official Plugin Directory

Source: [Build plugins](https://developers.openai.com/codex/plugins/build)

### Local plugin workflow

OpenAI documents a manual local-plugin workflow for Codex:

1. Copy the plugin folder into a plugin directory such as:
   - repo example: `$REPO_ROOT/plugins/my-plugin`
   - personal example: `~/.codex/plugins/my-plugin`
2. Add or update the marketplace file.
3. Point the plugin entry at that plugin folder with a `source` object using `source: "local"` and a relative `path`.
4. Restart Codex and verify that the plugin appears.

The docs also explicitly say those directories are examples rather than hard requirements; Codex resolves `source.path` relative to the marketplace root, not relative to `.agents/plugins/`.

Source: [Build plugins](https://developers.openai.com/codex/plugins/build)

### Marketplace metadata confirmed by docs

The Codex docs explicitly say marketplace entries should include:

- `source.path`
- `policy.installation`
- `policy.authentication`
- `category`

Documented examples and rules:

- `source.path` must stay relative to the marketplace root
- `source.path` should start with `./`
- `source.path` should stay inside that root
- documented `policy.installation` values include:
  - `AVAILABLE`
  - `INSTALLED_BY_DEFAULT`
  - `NOT_AVAILABLE`
- `policy.authentication` determines whether auth happens on install or first use

The docs show a local marketplace example with:

```json
{
  "name": "local-repo",
  "plugins": [
    {
      "name": "my-plugin",
      "source": {
        "source": "local",
        "path": "./plugins/my-plugin"
      },
      "policy": {
        "installation": "AVAILABLE",
        "authentication": "ON_INSTALL"
      },
      "category": "Productivity"
    }
  ]
}
```

Source: [Build plugins](https://developers.openai.com/codex/plugins/build)

### Install and cache behavior

OpenAI documents that Codex installs plugins from marketplaces into:

- `~/.codex/plugins/cache/$MARKETPLACE_NAME/$PLUGIN_NAME/$VERSION/`

For local plugins:

- `$VERSION` is `local`
- Codex loads the installed copy from that cache path rather than directly from the marketplace entry

The docs also say plugin on/off state is stored in:

- `~/.codex/config.toml`

Source: [Build plugins](https://developers.openai.com/codex/plugins/build)

### Relationship to the plugin bundle

Codex marketplace is not the same thing as the plugin bundle itself.

The plugin bundle is still documented as a plugin directory containing:

- `.codex-plugin/plugin.json`
- optional `skills/`
- optional `.app.json`
- optional assets and other plugin files

So the marketplace is a catalog and install mechanism around plugin directories, not a replacement for the plugin bundle contract.

Source: [Build plugins](https://developers.openai.com/codex/plugins/build)

### Practical conclusion

Codex officially supports both:

- plugin bundle authoring
- marketplace-based discovery and local/manual distribution

The two concepts are adjacent but distinct:

- bundle: `.codex-plugin/plugin.json` and plugin files
- marketplace: `.agents/plugins/marketplace.json` describing where Codex should load/install those bundles from

## Claude Marketplace

### What the marketplace is

Anthropic documents Claude Code marketplaces as a separate catalog file:

- `.claude-plugin/marketplace.json`

This file lives at the marketplace repository root and lists plugins plus their sources.

Source: [Create and distribute a plugin marketplace](https://code.claude.com/docs/en/plugin-marketplaces)

### Basic local marketplace layout

Anthropic’s walkthrough shows a local marketplace layout like:

- `my-marketplace/.claude-plugin/marketplace.json`
- `my-marketplace/plugins/<plugin>/.claude-plugin/plugin.json`
- `my-marketplace/plugins/<plugin>/skills/...`

They show a local marketplace entry like:

```json
{
  "name": "my-plugins",
  "owner": {
    "name": "Your Name"
  },
  "plugins": [
    {
      "name": "quality-review-plugin",
      "source": "./plugins/quality-review-plugin",
      "description": "Adds a /quality-review skill for quick code reviews"
    }
  ]
}
```

Source: [Create and distribute a plugin marketplace](https://code.claude.com/docs/en/plugin-marketplaces)

### Source rules

Anthropic documents that plugin-entry `source` can be either:

- a string
- an object

The docs explicitly show:

- relative local path sources like `./plugins/my-plugin`
- GitHub sources
- generic git/URL sources

They also state:

- relative paths resolve relative to the marketplace root
- the marketplace root is the directory containing `.claude-plugin/`
- `../` should not be used to escape that root

Source: [Create and distribute a plugin marketplace](https://code.claude.com/docs/en/plugin-marketplaces)

### Marketplace schema facts

Anthropic documents these required marketplace fields:

- `name`
- `owner`
- `plugins`

They also document optional metadata such as:

- `metadata.description`
- `metadata.version`
- `metadata.pluginRoot`

For plugin entries, they document required fields:

- `name`
- `source`

And optional fields include normal plugin metadata such as:

- `description`
- `version`
- `author`
- `homepage`
- `repository`
- `license`
- `keywords`
- `category`

Source: [Create and distribute a plugin marketplace](https://code.claude.com/docs/en/plugin-marketplaces)

### Adding and installing marketplaces

Anthropic documents both interactive slash commands and non-interactive CLI commands.

Documented add/install paths include:

- local directory:
  - `/plugin marketplace add ./my-marketplace`
  - `claude plugin marketplace add ./my-marketplace`
- direct `marketplace.json` path:
  - `/plugin marketplace add ./path/to/marketplace.json`
- remote URL:
  - `/plugin marketplace add https://example.com/marketplace.json`
- git or GitHub source:
  - `/plugin marketplace add https://gitlab.com/company/plugins.git`
  - `claude plugin marketplace add acme-corp/claude-plugins`

Plugin install is documented as:

- `/plugin install plugin-name@marketplace-name`

Source: [Create and distribute a plugin marketplace](https://code.claude.com/docs/en/plugin-marketplaces), [Discover and install prebuilt plugins through marketplaces](https://code.claude.com/docs/en/discover-plugins)

### Installation scopes

Anthropic documents multiple installation scopes for marketplace-installed plugins:

- user scope
- project scope
- local scope
- managed scope for admin-installed plugins

The docs say user scope is the default for direct installs, while other scopes are chosen through the UI.

Source: [Discover and install prebuilt plugins through marketplaces](https://code.claude.com/docs/en/discover-plugins)

### Managed restrictions

Anthropic documents first-class org/admin restrictions for marketplaces through managed settings, including:

- `strictKnownMarketplaces`
- allowlists for exact GitHub or URL sources
- `hostPattern`
- `pathPattern`

This is a real documented control plane around which marketplaces users may add.

Source: [Create and distribute a plugin marketplace](https://code.claude.com/docs/en/plugin-marketplaces)

### Cache / file-copy behavior

Anthropic also documents that when users install a plugin, Claude Code copies the plugin directory to a cache location. Because of that:

- plugins cannot rely on `../shared-utils` outside the plugin directory
- if files must be shared across plugins, symlinks are the documented workaround because they are followed during copying

Source: [Create and distribute a plugin marketplace](https://code.claude.com/docs/en/plugin-marketplaces)

### Practical conclusion

Claude marketplace support is broader and more explicitly tooled than Codex marketplace support in the docs:

- dedicated marketplace catalog file
- documented CLI marketplace subcommands
- documented add/install flows
- documented scope selection
- documented managed restrictions

## Codex vs Claude Marketplace: Confirmed Differences

### Codex

- marketplace files live in `.agents/plugins/marketplace.json`
- plugin entries use a structured `source` object with `source.path`, plus `policy.installation` and `policy.authentication`
- docs emphasize plugin directory install/cache behavior and restart-based pickup
- docs clearly separate marketplace catalog from plugin bundle structure

### Claude

- marketplace file lives at `.claude-plugin/marketplace.json`
- plugin entries use `source` as either string or object
- docs provide explicit CLI and slash-command marketplace management
- docs provide installation scopes and admin restrictions
- docs document richer marketplace schema and source types

## Gemini Publication: Confirmed Differences

### Gemini

- Gemini documents an extension gallery rather than a marketplace catalog file
- install sources are local paths or GitHub repositories
- gallery indexing depends on repository or release metadata rather than a local marketplace manifest
- `gemini-extension.json` must sit at the absolute repository root or release-archive root for gallery publication
- the `gemini-cli-extension` GitHub topic is part of the documented gallery-discovery path
- the best-practices doc explicitly recommends `gemini extensions link` for live CLI verification before release

Source: [Gemini CLI extensions](https://geminicli.com/docs/extensions/), [Release extensions](https://geminicli.com/docs/extensions/releasing/), [Extension best practices](https://geminicli.com/docs/extensions/best-practices/)

## Codex vs Claude vs Gemini: Confirmed Differences

- Codex uses a marketplace catalog around plugin bundles
- Claude uses a marketplace catalog around plugin bundles
- Gemini uses extension bundles plus gallery or release indexing rules
- Codex and Claude have explicit marketplace catalog manifests
- Gemini does not document the same local-catalog pattern

## Implication For This Repository

Based on the current `plugin-kit-ai` tree, the project already has strong support for:

- Codex plugin bundle authoring through `codex-package`

But it does **not** currently expose marketplace authoring as a first-class contract for Codex or Claude. In practice that means:

- bundle/package support exists
- marketplace catalog support is still research-level, not a shipped authored surface
- Gemini gallery or release publication support is also still research-level, not a shipped authored surface

Relevant project-local evidence:

- Codex package lane: [docs/generated/target_support_matrix.md](../../generated/target_support_matrix.md)
- current Codex research snapshot: [docs/research/codex-cli-plugins/README.md](../codex-cli-plugins/README.md)
- current Claude research snapshot: [docs/research/claude-code-plugins/README.md](../claude-code-plugins/README.md)
- current Gemini research snapshot: [docs/research/gemini-cli-extensions/README.md](../gemini-cli-extensions/README.md)

## Recommended Next Steps

1. Add a dedicated authored contract for marketplace catalogs instead of trying to fold marketplace metadata into plugin bundle metadata. `Увер. 9/10`, `Надёж. 9/10`, `Сложн. 7/10`
2. Keep current package/bundle support separate and document marketplace as unsupported until that contract exists. `Увер. 10/10`, `Надёж. 10/10`, `Сложн. 2/10`
3. Do not overload current `codex-package` or Claude bundle lanes with marketplace-only fields. `Увер. 10/10`, `Надёж. 10/10`, `Сложн. 2/10`
