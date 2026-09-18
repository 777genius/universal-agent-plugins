# Spike 2026-09-18: Claude vs Codex install slots

Maintainer investigation of official plugin loaders. **Not public native qualification.** Do not copy these versions into website compatibility tables or treat them as PR184 / 0.1.53 / draft 0.1.55 evidence.

Isolation: disposable `HOME` plus `CLAUDE_CONFIG_DIR` / `CODEX_HOME`. No writes into the operator’s real `~/.claude` or `~/.codex`. Binaries: **Claude Code 2.1.275**, **Codex CLI 0.152.0**.

Product decision: [ADR 0007](../../adr/0007-agentplugins-claude-skills-dir-plugin-slot.md).

## Claude `@skills-dir`

- `plugin list --json`: `id=<plugin.json name>@skills-dir`, `scope=user`, `enabled=true`, `installPath=<that folder>`.
- Edit a file in that folder: list path unchanged, file immediately new (in-place).
- Dirty neighborhood (empty dir, plain `SKILL.md` skill, dangling symlink, second plugin with a different `plugin.json` name): list succeeded. Only folders with `plugin.json` appear as plugins.
- No `plugins/cache` copy for this plugin.

## Claude marketplace

Tried both `"source": "."` and `./plugins/<name>`. Both install.

- Identity `name@<marketplace>`.
- `installPath` is `…/plugins/cache/<market>/<plugin>/<version>` (not source).
- Source edit does not change cache.
- `plugin update` no-ops same version (“already at the latest version”).
- Same-version content refresh: uninstall + install, or bump version then `plugin update` (cache then kept old and new versions).

## Codex marketplace

Codex has no in-place plugin slot.

- `CODEX_HOME` must already exist; 0.152.0 does not create it.
- `plugin add` copies to `…/plugins/cache/<market>/<plugin>/<version>`. Add JSON `installedPath` is cache.
- `plugin list --json` `source.path` is the **managed source**, not cache. Runtime bytes are cache. List path ≠ runtime path.
- Source edit does not change cache.
- `plugin marketplace upgrade` on a **local** marketplace fails: not configured as a Git marketplace. Do not call it on local sources.
- Same-version refresh: a second `plugin add` recopies cache.

## Installer case matrix

Pinned in `providers/install_slot_matrix_test.go`. Do not collapse the two rows.

| Client | Case | Expected |
| --- | --- | --- |
| Claude | target root | `ConfigRoot/skills`, not managed `clients/claude` |
| Claude | empty dir / `.DS_Store` / plain `SKILL.md` / dangling symlink / other-name plugin / malformed foreign `plugin.json` under `skills/` | skip; prepared identity not Indeterminate |
| Claude | same-name `plugin.json` sibling, including a directory symlink to the owned plugin | Collision |
| Claude | `.agentplugins-staging-*` leaked into `skills/` with same-name `plugin.json` | Collision (Claude auto-discovers it) |
| Claude | damaged owned `ActivePath` | Indeterminate |
| Claude | `plugin list` `name@skills-dir` + `installPath == ActivePath` + `scope=user` | Installed |
| Claude | leftover `name@marketplace` without `@skills-dir` | Absent for our claim |
| Claude | `name@skills-dir` at a different `installPath` | Collision |
| Claude | activate | `plugin list --json` only; no `marketplace add` / `plugin install` |
| Claude | projection | `.claude-plugin/plugin.json`; no `marketplace.json`; stage beside `skills` |
| Codex | target root | managed `clients/codex`; never `skills/` |
| Codex | empty dir / dangling symlink / foreign marketplace other namespace | skip; prepared identity not Indeterminate |
| Codex | `.agentplugins-staging-*` under the managed plugins root | skip (not a watched in-place slot) |
| Codex | same-name unqualified `plugin.json` sibling | Collision |
| Codex | damaged owned `ActivePath` | Indeterminate |
| Codex | `plugin list` `pluginId` `name@agentplugins-*` with `installed`+`enabled` | Installed; additive `source.path` is ignored |
| Codex | leftover `name@skills-dir` or another marketplace | Absent for our claim |
| Codex | activate | `marketplace add` + `plugin add name@agentplugins-*` + `list`; never `marketplace upgrade` on local |
| Codex | projection | `.agents/plugins/marketplace.json`; stage under the managed target root |
| Codex | local same-version refresh | restage + `plugin add` again (recopies cache); live spike, not a unit of this matrix |

## Agent Plugins implication

Claude stays `@skills-dir` so hash/repair/stdio bind to the live folder. Codex stays marketplace + cache (local refresh = restage + `plugin add` again). Do not unify the two.
