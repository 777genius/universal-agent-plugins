# Publish JSON Contract

`plugin-kit-ai publish --format json` is the automation-facing report for the bounded top-level publish workflow.

## Stability

- Contract id: `plugin-kit-ai/publish-report`
- Current schema version: `1`
- Stability tier: public contract for CI and tooling

## Envelope

Every JSON report includes:

- `format`: always `plugin-kit-ai/publish-report`
- `schema_version`: currently `1`
- `ready`
- `status`
- `mode`
- `workflow_class`
- `detail_count`
- `details`
- `issue_count`
- `issues`
- `next_step_count`
- `next_steps`

Optional fields:

- `channel`: present for single-channel publish workflows
- `target`: present for single-channel publish workflows
- `dest`: present for local marketplace-root workflows
- `package_root`: present for local marketplace-root workflows
- `warning_count` and `warnings`: present for multi-channel planning and for channel-level warnings
- `channel_count` and `channels`: present for `publish --all --dry-run`

## Workflow Classes

- `local_marketplace_root`: local Codex or Claude marketplace-root materialization flow
- `repository_release_plan`: Gemini repository or release publication planning flow
- `multi_channel_plan`: authored-channel orchestration from `publish --all --dry-run`

## Status Semantics

- `ready`: the bounded publish workflow is fully ready for the requested channel
- `needs_channels`: `publish --all --dry-run` found no authored publication channels under `publish/...`
- `needs_attention`: `publish --all --dry-run` found one or more authored channels that still need follow-up
- `needs_repository`: Gemini repository or release publication planning found missing Git or GitHub prerequisites

Local Codex and Claude marketplace-root flows currently report `ready` when the bounded publish workflow can proceed.

## Issue Records

`issues` is the structured explanation surface for bounded publish gaps.

Each issue record includes:

- `code`
- `message`

Current issue codes:

- `gemini_git_cli_unavailable`
- `gemini_git_repository_missing`
- `gemini_origin_remote_missing`
- `gemini_origin_not_github`

## Array and Map Guarantees

The following fields are always present in schema version `1`:

- `details`
- `issues`
- `next_steps`

`details` is always an object, and `issues` plus `next_steps` are always arrays, never `null`.

For `multi_channel_plan`, `warnings` and `channels` are also always arrays, never `null`.

## Compatibility Rules

- Additive fields may appear in future schema versions
- Breaking changes require a new `schema_version`
- Consumers should branch first on `format`, then on `schema_version`
- Consumers should treat `workflow_class` as the primary dispatch field for channel-family automation
