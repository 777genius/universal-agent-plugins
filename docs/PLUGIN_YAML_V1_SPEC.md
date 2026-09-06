# `plugin.yaml` V1 Spec

> Owner clarification (2026-09-06): historical status is not permission to delete
> useful legacy implementation, tests or design ideas. Follow the
> [capability preservation contract](./AUTHORING_CAPABILITY_PRESERVATION.md).
>
> Historical v1 design — not current Agent Plugins 1.0 guidance.
> Superseded for future authoring by [ADR 0006](./adr/0006-standard-first-authoring.md)
> and the [implementation plan](./STANDARD_FIRST_AUTHORING_ENGINE_IMPLEMENTATION_PLAN.md).
> The retained content describes legacy `plugin-kit-ai` v1 (baseline `1.2.4`).
> Root `plugin.json` is the portable standard; v2 migration is not yet available.

Spec date: 2026-04-04

This document defines the intended `plugin.yaml` v1 contract for `plugin-kit-ai`.

It is the field-level companion to:

- [Plugin Standard and Publish Plan](./PLUGIN_STANDARD_AND_PUBLISH_PLAN.md)
- [Publish Layer Spec](./PUBLISH_LAYER_SPEC.md)

## Purpose

`plugin.yaml` is the minimal core plugin manifest for `plugin-kit-ai`.

It is intentionally small.

It describes:

- plugin identity
- plugin release version
- short description
- enabled target adapters

It does **not** describe:

- vendor marketplace publication details
- vendor gallery metadata
- vendor-specific package internals
- vendor-specific UI metadata

## V1 Shape

```yaml
api_version: v1
name: my-plugin
version: 0.1.0
description: Short plugin description
targets:
  - codex-package
```

## Fields

### `api_version`

Required.

Meaning:

- version of the `plugin.yaml` schema itself

Rules:

- must be `v1`
- must be a string

Notes:

- this is the only supported schema marker for the v1 contract

### `name`

Required.

Meaning:

- stable plugin identity

Rules:

- must be a machine-friendly project name accepted by current `plugin-kit-ai` validation
- should be lowercase and slug-like
- should remain stable across releases

Notes:

- `name` is the only identity field in v1
- there is no separate `id`

### `version`

Required.

Meaning:

- plugin release version

Rules:

- must be a non-empty string
- semantic versioning is recommended

Notes:

- this is the plugin release version
- it is not the schema version

### `description`

Required.

Meaning:

- short human-facing summary of the plugin

Rules:

- must be a non-empty string
- should fit in one short sentence or phrase

### `targets`

Required.

Meaning:

- enabled `plugin-kit-ai` target adapters

Rules:

- must be a non-empty YAML sequence
- entries must be supported target ids
- entries must not be duplicated

Notes:

- this is intentionally a `plugin-kit-ai` orchestration field
- it is not a claim that every external plugin ecosystem uses the same concept directly

## Excluded From V1

These fields are intentionally excluded from `plugin.yaml` v1:

- `id`
- `authors`
- `license`
- `homepage`
- `repository`
- `keywords`
- `category`
- marketplace source metadata
- marketplace install policy
- marketplace auth policy
- Codex interface fields
- Codex app fields
- Gemini settings
- Gemini themes
- Gemini hooks
- Claude marketplace entry metadata

Reason:

- those belong either to vendor-specific target authoring or to the future `publish/...` layer

## Canonical V1 Shape

```yaml
api_version: v1
name: my-plugin
version: 0.1.0
description: Short plugin description
targets:
  - codex-package
```

## Relationship To Other Layers

### `plugin/targets/...`

Holds vendor-specific authored data.

Examples:

- `plugin/targets/codex-package/...`
- `plugin/targets/codex-runtime/...`
- `plugin/targets/claude/...`
- `plugin/targets/gemini/...`

### `plugin/publish/...`

Will hold marketplace, gallery, and catalog publication metadata.

Examples:

- `publish/codex/...`
- `publish/claude/...`
- `publish/gemini/...`

This publication layer is intentionally separate from `plugin.yaml` core identity.
The current implementation in `plugin-kit-ai` now includes both:

- an internal normalized publication summary
- authored publication-schema entrypoints under `publish/...`

Examples:

- `publish/codex/marketplace.yaml`
- `publish/claude/marketplace.yaml`
- `publish/gemini/gallery.yaml`

### Generated vendor manifests

Examples:

- `.codex-plugin/plugin.json`
- `.claude-plugin/plugin.json`
- `gemini-extension.json`

These are generated artifacts, not the primary authored source of truth.
