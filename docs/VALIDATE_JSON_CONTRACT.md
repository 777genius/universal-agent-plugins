# Validate JSON Contract

`plugin-kit-ai validate --format json` is the automation-facing validation report for package-standard plugin repos.

## Stability

- Contract id: `plugin-kit-ai/validate-report`
- Current schema version: `1`
- Stability tier: public contract for CI and tooling

## Envelope

Every JSON report includes:

- `format`: always `plugin-kit-ai/validate-report`
- `schema_version`: currently `1`
- `requested_platform`: the explicit `--platform` value when present
- `outcome`: one of `passed`, `failed`, or `failed_strict_warnings`
- `ok`: convenience boolean for green/red automation checks
- `strict_mode`: whether `--strict` was enabled
- `strict_failed`: true only when strict mode failed because warnings were treated as errors
- `warning_count`
- `failure_count`
- `platform`: the enabled-target summary returned by the validator
- `checks`
- `warnings`
- `failures`

## Outcome Semantics

- `passed`: no failures, and either strict mode was off or there were no warnings
- `failed`: at least one validation failure exists
- `failed_strict_warnings`: no validation failures exist, but `--strict` failed because warnings were present

## Array Guarantees

The following fields are always arrays in schema version `1`, never `null`:

- `checks`
- `warnings`
- `failures`

## Failure And Warning Records

Warnings expose:

- `kind`
- `path` when known
- `message`

Failures expose:

- `kind`
- `path` when known
- `target` when the failure belongs to a specific target lane
- `message`

## Compatibility Rules

- Additive fields may appear in future schema versions
- Breaking changes require a new `schema_version`
- Consumers should branch first on `format`, then on `schema_version`
- Consumers should prefer `outcome` over re-deriving state from counters
