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
