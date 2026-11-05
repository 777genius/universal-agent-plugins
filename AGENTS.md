# Agent instructions

## Architecture and contribution guardrails

Before implementation changes, read [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
for the applicable package boundaries and [CONTRIBUTING.md](CONTRIBUTING.md)
for actual lint and test commands. Check current `main` and overlapping PRs
before touching a package. Preserve all owner decisions below.

- **Domain:** business rules and invariants, no external I/O. Reuse the
  existing Go domain types instead of creating another source of truth.
- **Ports/use cases:** depend on narrow contracts, not concrete adapters;
  keep client-specific protocol and filesystem effects behind existing seams.
- **Clients:** implement capabilities in `clients/<id>` and register them
  through `clients/all` at a composition root. Do not introduce client-ID
  branches into generic planners, providers, or use cases.
- **Composition:** executable entrypoints assemble concrete client registries;
  libraries receive their dependencies explicitly.
- **Safety:** preserve source/path containment, foreign-file ownership,
  immutable package identity, locking, receipts, rollback and recovery, public
  JSON and exit statuses. Never weaken a refusal just to make tests pass.
- **Simplicity:** reuse existing parsers, validation and adapters. Introduce an
  interface only for a real dependency boundary or testable requirement; do
  not add a second installer, resolver, state engine, or feature flag.
- **Scope:** do not modify frozen VS Code behavior, hand-edit generated files,
  widen legacy lint exemptions, or silently change public/release contracts.
- **Verification:** new Go files are limited to 500 effective lines by `revive`;
  complexity, duplication and import rules are enforced by `golangci-lint`.
  Run `make lint` and `make test-core` for relevant Go changes. For lifecycle
  changes also run the affected native, rollback and cross-platform E2E.
  Verify actual cases on the final commit: queued, cancelled, skipped or
  evidence-only workflows are not proof that the product passed.

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
or a second supported engine. All hosted Codex workers for this program use
gpt-6.1-sol with reasoning effort low for ordinary tasks and service tier default;
raise effort only when the task requires it. Astra and fast mode are not authorized.

## Public wording and release boundaries

Read the early-public-checkpoint owner decision in the implementation plan.
D5 qualifies executable release; it does not block truthful public wording.
Preserve PR190 availability labels and historical links. Never use
DOCS_PREPARATION_PREVIEW in production or equate installer release proof with
standard authoring release qualification. Use gpt-6.1-sol / low / default
for ordinary hosted Codex tasks; raise effort only when needed. No Astra or fast.
This owner instruction of 2026-10-26 supersedes older Astra assignments in plans
and handoffs.

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
- Public docs are published only for `index`, `use/*`, `build/*`, `sdk/*`,
  `guide/quickstart`, `reference/client-compatibility` and the authoring API.
  Other `website/source/en` pages are archived and not published.
- Pages under `use/` and `build/` are journey pages. They need released status
  and routes in all nine maintained locales (`en`, `ru`, `uk`, `zh`, `es`, `hi`, `ar`, `pt`, `fr`).
