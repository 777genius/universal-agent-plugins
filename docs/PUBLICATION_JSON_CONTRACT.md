# Publication JSON Contract

`plugin-kit-ai publication --format json` is the automation-facing summary report for the publication layer.

## Stability

- Contract id: `plugin-kit-ai/publication-report`
- Current schema version: `1`
- Stability tier: public contract for CI and tooling

## Envelope

Every JSON report includes:

- `format`: always `plugin-kit-ai/publication-report`
- `schema_version`: currently `1`
- `requested_target`: the explicit `--target` value when present
- `warning_count`
- `warnings`
- `publication`

## Array Guarantees

The following fields are always arrays in schema version `1`, never `null`:

- `warnings`
- `publication.packages`
- `publication.channels`

## Publication Payload

`publication` includes:

- `core`
- `packages`
- `channels`

It is the same normalized publication model surfaced by:

- `plugin-kit-ai inspect --format json`
- `plugin-kit-ai validate --format json`
- `plugin-kit-ai publication doctor --format json`

## Compatibility Rules

- Additive fields may appear in future schema versions
- Breaking changes require a new `schema_version`
- Consumers should branch first on `format`, then on `schema_version`
- Consumers should treat `publication` as the canonical summary payload and `warnings` as advisory metadata
