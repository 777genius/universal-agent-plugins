# ADR 0007: Agent Plugins Claude `@skills-dir` plugin slot

## Status

Accepted

## Context

Claude Code has two official plugin loaders that share `.claude-plugin/plugin.json`:

1. **Skills-directory plugins (in-place).** A directory directly under the skills root with `plugin.json` loads as `name@skills-dir`. Official `claude plugin init` writes `~/.claude/skills/<name>/`. Runtime is that folder. This is a full plugin slot (skills, MCP, and Claude-native commands/agents/hooks if present), not SKILL.md-only.
2. **Marketplace install.** `marketplace add` + `plugin install name@marketplace` copies the plugin to `…/plugins/cache/<marketplace>/<plugin>/<version>`. Runtime is the cache, not the source.

Agent Plugins 1.0 portable packages expose skills + MCP. The installer synthesizes Claude `plugin.json`; authors do not write Claude-native plugins. Hash, repair, and stdio binding need the bytes Claude actually executes.

Isolated CLI spikes on 2026-09-18 (Claude Code 2.1.275, Codex CLI 0.152.0, disposable `HOME` / `CLAUDE_CONFIG_DIR` / `CODEX_HOME`) showed:

- `@skills-dir` `installPath` is the in-place folder; source edits are live.
- Marketplace `installPath` is cache; `plugin update` no-ops same-version content; same-version refresh needs uninstall+install or a version bump.
- Codex has no in-place plugin slot. `plugin add` copies to cache; list `source.path` is managed source; `plugin marketplace upgrade` on a local marketplace fails.

The legacy integrationctl Claude adapter (`install/integrationctl/adapters/claude`) already uses marketplace for `plugin.yaml` packages. That is a different product lane.

## Decision

Agent Plugins Claude user-scope delivery is the official **in-place `@skills-dir` plugin slot**: `ConfigRoot/skills/<physicalArtifactID>`, identity `DeclaredName@skills-dir` with `installPath == ActivePath` and `scope=user`. Activation is kernel dirswap plus verify-only `claude plugin list --json`. Prepared identity walks `skills/` but does not treat hostile neighbors as a registry: only a proven same-name `plugin.json` collides.

Codex stays marketplace + cache. The legacy `plugin.yaml` Claude adapter stays marketplace. Agent Plugins Claude does not synthesize `marketplace.json` or call `plugin install`.

## Consequences

- Stdio and package digest bind to `plan.ActivePath`, which is the live Claude plugin root.
- Staging must sit beside `skills`, not inside it, so a plugin-shaped staging directory cannot leak into `plugin list`.
- Repair restages and verifies; it does not re-run marketplace CLI.
- Neighbors such as dangling symlinks, empty dirs, Finder `.DS_Store`, and plain `SKILL.md` skills do not fail-close install/repair.
- Later installer-core Parts 5–8 may move Claude files into `clients/claude`; this decision is about the slot, not package layout.

This spike is maintainer investigation, not public native-release qualification. Historical `@skills-dir` compatibility labels stay as written.

## Non-Goals

- Expanding portable projection to Claude-native commands, agents, hooks, or LSP.
- Project-scope `.claude/skills/`.
- Changing the legacy `plugin.yaml` Claude adapter.
- Unifying Claude delivery with Codex marketplace.
- Publishing laptop spike versions as client-compatibility release evidence.

## Rejected Alternatives

- Unify Agent Plugins Claude with Codex marketplace / `plugin install`. Cache path appears after projection, same-version `plugin update` no-ops, and stdio baked to managed source would split from cached skills.
- Scan `~/.claude/skills` as a fail-closed registry. A shared skills root is not private; dangling links and empty dirs are not competing plugin claims.
- Call `plugin install` without a marketplace catalog. That is not an official skills-dir load path.
