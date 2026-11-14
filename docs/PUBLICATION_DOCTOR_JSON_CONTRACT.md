# Publication Doctor JSON Contract

`plugin-kit-ai publication doctor --format json` is the automation-facing readiness report for the `publish/...` layer.

## Stability

- Contract id: `plugin-kit-ai/publication-doctor-report`
- Current schema version: `1`
- Stability tier: public contract for CI and tooling

## Envelope

Every JSON report includes:

- `format`: always `plugin-kit-ai/publication-doctor-report`
- `schema_version`: currently `1`
- `requested_target`: the explicit `--target` value when present
- `ready`: convenience boolean for publication readiness
- `status`: one of `ready`, `needs_channels`, `needs_generate`, `needs_repository`, `needs_sync`, or `inactive`
- `warning_count`
- `warnings`
- `issue_count`
- `issues`
- `next_steps`
- `publication`

When publication-capable package targets are missing authored channels, the report also includes:

- `missing_package_targets`

When `plugin-kit-ai publication doctor --dest <marketplace-root>` is used for supported local Codex or Claude flows, the report also includes:

- `local_root`

## Status Semantics

- `ready`: every publication-capable package target has an authored `publish/...` channel
- `needs_channels`: at least one publication-capable package target exists, but one or more required `publish/...` channels are missing
- `needs_generate`: authored publication inputs exist, but one or more required generated package or publication artifacts are missing
- `needs_repository`: publication metadata is authored, but repository-rooted Gemini distribution prerequisites are missing
- `needs_sync`: source publication inputs are ready, but the materialized local marketplace root is missing files or has drift
- `inactive`: no publication-capable package targets are enabled for the requested scope

## Issue Records

`issues` is the structured explanation surface for publication gaps.

Each issue record includes:

- `code`
- `message`
- `target` when the issue belongs to a specific package target
- `channel_family` when the issue belongs to a specific publication family
- `path` when a concrete authored path is relevant

Current issue codes:

- `no_publication_targets`
- `missing_channel`
- `missing_package_artifact`
- `missing_channel_artifact`
- `drifted_package_artifact`
- `drifted_channel_artifact`
- `stale_generated_artifact`
- `generate_probe_failed`
- `gemini_git_cli_unavailable`
- `gemini_git_repository_missing`
- `gemini_origin_remote_missing`
- `gemini_origin_not_github`
- `missing_materialized_package_root`
- `missing_materialized_package_artifact`
- `missing_materialized_catalog_artifact`
- `drifted_materialized_catalog_identity`
- `drifted_materialized_catalog_entry`
- `missing_materialized_catalog_entry`

## Array Guarantees

The following fields are always arrays in schema version `1`, never `null`:

- `warnings`
- `issues`
- `next_steps`
- `publication.packages`
- `publication.channels`

When `local_root` is present, the following are also arrays in schema version `1`, never `null`:

- `local_root.issues`
- `local_root.next_steps`

## Local Root Payload

`local_root` appears only when `publication doctor` is asked to verify an already materialized local marketplace root via `--dest`.

It includes:

- `ready`
- `status`
- `dest`
- `package_root`
- `catalog_path`
- `issue_count`
- `issues`
- `next_steps`

## Publication Payload

`publication` reuses the normalized publication model surfaced by:

- `plugin-kit-ai publication --format json`
- `plugin-kit-ai inspect --format json`
- `plugin-kit-ai validate --format json`

It includes:

- `core`
- `packages`
- `channels`

## Exit Semantics

- exit `0`: `ready` is `true`
- exit `1`: `ready` is `false`

Consumers should rely on both:

- shell exit code
- structured `ready` and `status`

## Compatibility Rules

- Additive fields may appear in future schema versions
- Breaking changes require a new `schema_version`
- Consumers should branch first on `format`, then on `schema_version`
- Consumers should prefer `status` over re-deriving readiness from package or channel counts
