# Decided: Agent Plugin authoring experience

> Owner clarification (2026-09-06): historical status is not permission to delete
> useful legacy implementation, tests or design ideas. Follow the
> [capability preservation contract](./AUTHORING_CAPABILITY_PRESERVATION.md).

> Decision recorded in [ADR 0006](./adr/0006-standard-first-authoring.md) and the
> [approved implementation plan](./STANDARD_FIRST_AUTHORING_ENGINE_IMPLEMENTATION_PLAN.md).
> Standard-first authoring uses root `plugin.json`, without a required sidecar.
> The questions below are historical context; implementation follows phase gates.

The preserved `/create-plugin` page still describes the legacy `plugin.yaml` authoring workflow. Do not publish it as current Agent Plugins 1.0 guidance until the complete standard-first release gate passes.

The original decision considered these options:

1. Make the standard `plugin.json` the only authoring source.
2. Make `plugin.json` primary and allow an optional, non-installed build/publish sidecar.
3. Keep `plugin.yaml` only as an explicit legacy migration input that generates a standard package.

The decision must preserve these invariants:

- installed packages are driven by Agent Plugins 1.0 `plugin.json`;
- build or publishing metadata cannot silently override the standard manifest;
- generated output is deterministic and reviewable;
- existing legacy users receive a documented migration path.

The owner approved the standard-first direction. Public documentation cutover remains gated on a working dual-entrypoint release; v1 continues unchanged until then.
