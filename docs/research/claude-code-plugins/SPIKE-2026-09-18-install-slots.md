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
| Claude | `.agentplugins-staging-*` leaked into `skills/` with same-name `plugin.json` | Collision (defense in depth). Live Claude 2.1.275 does **not** list dot directories |
| Claude | `plugin.json` `name` ≠ folder name | list id is `name@skills-dir`, `installPath` is the folder |
| Claude | two folders with the same `plugin.json` name | one loads; loser is `folder@skills-dir`, `enabled=false`, empty `installPath`, `errors[]`. List parser skips that loser |
| Claude | root `plugin.json` without `.claude-plugin`, nested plugin, SKILL.md-only, empty, dangling, `.DS_Store` | not listed |
| Claude | in-place edit | `installPath` unchanged; bytes live in the skills folder |
| Claude | damaged owned `ActivePath` | Indeterminate |
| Claude | `plugin list` `name@skills-dir` + `installPath` same directory as ActivePath + `scope=user` | Installed. Compare with `os.SameFile` (`/tmp` vs `/private/tmp`), not only `filepath.Clean` |
| Claude | leftover enabled `name@marketplace` without `@skills-dir` | Collision. Claude 2.1.275 lets marketplace win; skills-dir is `folder@skills-dir` failed. Disabled marketplace leftover is Absent |
| Claude | `name@skills-dir` at a different `installPath` | Collision |
| Claude | `plugin disable` skills-dir | `enabled=false`, path still absolute → installer Absent |
| Claude | in-place `plugin.json` rename | list id changes immediately; folder path unchanged |
| Claude | directory symlink under `skills/` | Claude loads it; `installPath` is the symlink path. Activator still refuses a symlink ActivePath |
| Claude | `plugin uninstall` of `@skills-dir` | fails: loaded from skills with no marketplace backing. Delete the folder (or disable). Confirms verify-only list, no uninstall CLI |
| Claude | `/tmp` `CLAUDE_CONFIG_DIR` | list `installPath` keeps `/tmp/...`; `realpath` is `/private/tmp/...`. SameFile must match |
| Claude | unicode `plugin.json` name | listed as `name@skills-dir` |
| Claude | hidden `.agentplugins-staging-*` | not listed; installer still collides defense-in-depth |
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
| Codex | local same-version refresh | restage + `plugin add` again (recopies cache); repair with a trusted CLI now re-runs that Activate |
| Codex | missing `CODEX_HOME` | `plugin list --json` can return empty `installed` using `$HOME/.codex`; installer must not treat that as a hard CLI crash. `plugin add` still needs a real home |
| Codex | add vs list paths | add `installedPath` is cache (`/private/tmp/...` on macOS); list `source.path` is managed source |
| Codex | `plugin marketplace upgrade` on local 0.152.0 | success with empty `upgradedRoots`; do not call it |
| Codex | `plugin add` without marketplace | fails |
| Codex | marketplace add twice | `alreadyAdded: true` |
| Codex | local source path with spaces | add works; config `source` may be `/private/tmp/...` |
| Codex | `plugin remove` | list empty, cache gone, marketplace stanza in `config.toml` remains, managed source remains. Uninstall must still `plugin remove` then `marketplace remove` |
| Codex | `plugin marketplace remove` while plugin installed | list cleared, marketplace stanza gone, `[plugins."name@market"] enabled=true` leftover. Repair with CLI re-adds marketplace then `plugin add` |
| Codex | versioned cache layout 0.152.0 | `plugins/cache/<market>/<plugin>/<version>/.codex-plugin/plugin.json`. File inspect must not require the older `local/` path |
| Codex | stale cache / missing list while managed source digest is intact | repair re-runs Activate (`plugin add`) when a trusted CLI is present |

## Live follow-up (same day)

Isolated `HOME` + `CLAUDE_CONFIG_DIR` / `CODEX_HOME` on Claude Code **2.1.275** and Codex CLI **0.152.0**. Operator homes were not written. Not public qualification.

Deeper pass also showed: Claude list spelling follows `CLAUDE_CONFIG_DIR` (`/tmp/...`) while `realpath` is `/private/tmp/...`; `plugin uninstall` cannot remove `@skills-dir`; Codex `plugin remove` leaves the marketplace source registered.

Round 3 also showed:

- Unicode `plugin.json` names list as `name@skills-dir`. Deleting the folder drops the list entry.
- Two skills-dir folders with the same name: one winner `name@skills-dir`, loser `folder@skills-dir` with empty `installPath`.
- Marketplace install of the same name wins over skills-dir. List has `name@marketplace` plus `folder@skills-dir` failed. There is no `name@skills-dir` winner. `plugin marketplace add` does not take `--json`.
- `plugin uninstall` of `@skills-dir` says the plugin is loaded from skills with no marketplace backing (not the older `directory_loaded` token).
- `plugin disable` keeps an absolute `installPath` with `enabled=false`.
- Hidden `.agentplugins-staging-*` is not listed.
- Codex `plugin list` with unset `CODEX_HOME` can return empty `installed` instead of crashing.
- Codex marketplace add of a path with spaces stores `/private/tmp/...` in `config.toml`. Second add is `alreadyAdded: true`.
- Codex `plugin marketplace remove` while the plugin is installed clears the list and the marketplace stanza, but leaves `[plugins."name@market"] enabled=true`. Re-add marketplace + `plugin add` restores the list.
- Codex versioned cache is `plugins/cache/<market>/<plugin>/<version>/`, not `.../local/`.

## Agent Plugins implication

Claude stays `@skills-dir` so hash/repair/stdio bind to the live folder. Codex stays marketplace + cache (local refresh = restage + `plugin add` again). Do not unify the two.
