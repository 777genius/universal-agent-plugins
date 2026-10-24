# Agent instructions

## VS Code is frozen (owner decision, 2026-10-16)

Do not change VS Code implementation, adapters, protocols, configuration,
or dedicated fixtures/tests. Preserve existing behavior and source. New dependency-managed
installation and release work targets Claude, Codex, OpenCode, Cursor and Gemini;
VS Code remains excluded. Reopening VS Code work requires a new explicit owner
instruction.

## Standard-first authoring: preserve legacy capabilities

For this authoring program, read docs/AUTHORING_CAPABILITY_PRESERVATION.md and
the owner clarification at the top of the implementation plan. Do not delete
useful plugin.yaml implementation, dependencies, tests or design documentation
because plugin.json is narrower or a standard-first command no longer calls it.
Retiring command exposure is distinct from removing source. Preserve unresolved
capabilities outside the standard dependency graph. Any deletion requires an
inventory item and explicit owner acceptance. Do not add implicit YAML fallback
or a second supported engine. All hosted workers for this program use
gpt-6-astra with service tier default; fast mode is not authorized.

## Public wording and release boundaries

Read the early-public-checkpoint owner decision in the implementation plan.
D5 qualifies executable release; it does not block truthful public wording.
Preserve PR190 availability labels and historical links. Never use
DOCS_PREPARATION_PREVIEW in production or equate installer release proof with
standard authoring release qualification. Use gpt-6-astra / medium / default
for current hosted tasks; no fast.

## Owner release authorization (2026-10-08)

An explicit owner request to make or publish a release authorizes choosing and
publishing the next suitable unused version within the same product and channels.
Respect explicit constraints and report the chosen version; do not ask again
solely for version selection or skipping a cancelled, never-public candidate.
Preparation, review, and draft-only requests do not authorize public publication.
All CI, independent review, environment, artifact, draft/promotion, and immutable
tag requirements remain mandatory.

## Where things live (repositories and sites)

- `777genius/universal-agent-plugins` (this repo): CLI (`cli/`, `install/`),
  landing (`landing/`, https://universal-agent-plugins.com/), public docs source
  (`website/`, https://universal-agent-plugins.com/docs/en/).
- `777genius/universal-agent-plugins-registry`: signed directory feeds
  (`catalog/`, `registry/`). Its old site (`site/`) is retired:
  https://777genius.github.io/universal-agent-plugins-registry/ shows a
  "has moved" notice.
- The Directory (plugin list and install buttons) is `landing/pages/plugins/`
  in this repo, published at https://777genius.github.io/universal-agent-plugins/plugins/.
- The landing build verifies the registry's signed feeds with
  `cmd/agentplugins-registry-mirror`, so Directory data follows the registry.
- Install buttons in the Directory and the landing catalog must match the
  source kinds the released CLI accepts. Check `agentplugins skills add --help`
  (or the release notes) before changing them.
- Public docs are published only for `index`, `use/*`, `build/*`,
  `guide/quickstart`, `reference/client-compatibility` and the authoring API.
  Other `website/source/en` pages are archived and not published.
- Pages under `use/` and `build/` are journey pages. They need released status
  and routes in all five maintained locales (`en`, `ru`, `es`, `fr`, `zh`).
