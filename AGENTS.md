# Agent instructions

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
