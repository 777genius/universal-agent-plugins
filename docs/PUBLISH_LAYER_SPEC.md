# Publish Layer Spec

> Owner clarification (2026-09-06): historical status is not permission to delete
> useful legacy implementation, tests or design ideas. Follow the
> [capability preservation contract](./AUTHORING_CAPABILITY_PRESERVATION.md).

Spec date: 2026-04-04

This document defines the intended `publish/...` layer for `plugin-kit-ai`.

It is the publication-channel companion to:

- [Plugin Standard and Publish Plan](./PLUGIN_STANDARD_AND_PUBLISH_PLAN.md)
- [plugin.yaml V1 Spec](./PLUGIN_YAML_V1_SPEC.md)
- [Codex, Claude, and Gemini publication research](./research/plugin-marketplaces/README.md)

## Purpose

`publish/...` is the authored layer for marketplace, gallery, and catalog metadata.

It is intentionally separate from:

- `plugin.yaml`
- `targets/...`
- generated vendor manifests

This layer answers:

- how a package should be published or indexed
- which vendor publication family it targets
- which channel-specific metadata is needed beyond package identity

It does **not** answer:

- what the core plugin identity is
- how vendor package manifests are authored
- how target-specific features such as Gemini themes or Codex interface data are authored

## Layering

The intended architecture is:

1. `plugin.yaml`
2. `targets/...`
3. optional `publish/...`
4. generated vendor manifests and publication artifacts

Meaning:

- `plugin.yaml` defines minimal universal plugin identity
- `targets/...` define vendor adaptation inputs
- `publish/...` defines publication-channel metadata
- generated files expose vendor-visible outputs at the filesystem locations that real vendor tooling expects

## Core principles

### Keep `plugin.yaml` small

`plugin.yaml` remains the minimal core standard.

It must not absorb publication-channel concerns such as:

- marketplace source metadata
- install policy
- authentication policy
- gallery indexing knobs
- release-discovery knobs

### Separate package identity from publication metadata

A package and a publication channel are not the same thing.

Examples:

- Codex package bundle is not the same as Codex marketplace catalog metadata
- Claude plugin bundle is not the same as Claude marketplace catalog metadata
- Gemini extension package is not the same as Gemini gallery indexing requirements

### Keep vendor filesystem expectations real

Vendor-visible files must still exist where vendor tooling expects them.

Examples:

- `.codex-plugin/plugin.json`
- `.claude-plugin/plugin.json`
- `gemini-extension.json`

`publish/...` does not replace those files.

## Scope of the first publication layer

The first publication layer should cover:

- channel identity
- source layout expectations
- install or discovery semantics
- channel-specific optional metadata

It should not yet try to automate:

- remote publishing credentials
- release uploading
- registry mutation
- marketplace synchronization jobs

Those belong to later workflow phases.

## Channel families

Current intended channel families:

- `codex-marketplace`
- `claude-marketplace`
- `gemini-gallery`

These are intentionally separate families, not aliases of one another.

## Illustrative root layout

⚠️ These paths are illustrative and not frozen forever.

```text
publish/
  codex/
    marketplace.yaml
  claude/
    marketplace.yaml
  gemini/
    gallery.yaml
```

The exact filenames may still evolve, but the conceptual split should remain:

- one sub-root per vendor publication family
- no fake universal `marketplace.yaml` for all vendors

## What belongs in `publish/...`

### Codex marketplace

Expected authored concerns:

- marketplace identity
- plugin listing metadata
- source-root strategy
- installation policy
- authentication policy
- category

Reason:

Official Codex docs describe a marketplace catalog around plugin bundles, with `source.path`, `policy.installation`, `policy.authentication`, and `category`.

### Claude marketplace

Expected authored concerns:

- marketplace identity
- marketplace owner metadata
- plugin source metadata
- optional listing metadata
- scope or install guidance where needed

Reason:

Official Claude docs describe a separate `.claude-plugin/marketplace.json` catalog with marketplace-level and plugin-entry-level metadata.

### Gemini gallery

Expected authored concerns:

- gallery-facing metadata not already implied by the package
- repository or release publication intent
- optional indexing or release hints

Reason:

Official Gemini docs describe gallery discovery through repository or release rules, not through a separate marketplace catalog file.

Current enforced contract:

- `repository_visibility` must stay `public`
- `github_topic` must stay `gemini-cli-extension`
- `distribution: git_repository` requires `manifest_root: repository_root`

Reason:

Official Gemini docs say the gallery crawler looks for public GitHub repositories tagged with `gemini-cli-extension`, and they require `gemini-extension.json` at the absolute root of the repository or release archive.

## What does not belong in `publish/...`

These stay elsewhere:

- plugin `name`, `version`, `description`
- target enablement
- Codex interface data
- Codex app data
- Claude plugin internals
- Gemini contexts, hooks, themes, settings
- portable MCP definitions
- portable skills

Reason:

Those are core identity, vendor package inputs, or portable authored surfaces, not publication-channel metadata.

## Relationship to the internal publication model

`plugin-kit-ai` now has an internal normalized publication summary.

That model:

- starts from `plugin.yaml`
- includes publication-capable target adapters
- is visible today via `plugin-kit-ai inspect --format json`
- is also exposed through `plugin-kit-ai publication --format json`
- is also exposed through `plugin-kit-ai publication doctor --format json`

This internal model should become the bridge between:

- authored package inputs
- future authored `publish/...` metadata
- generated publication artifacts

The publication readiness contract is documented in [Publication Doctor JSON Contract](./PUBLICATION_DOCTOR_JSON_CONTRACT.md).
The publication summary contract is documented in [Publication JSON Contract](./PUBLICATION_JSON_CONTRACT.md).

## Planned rollout

### Step 1

Keep publication modeling internal only.

Status:

- completed as the first bridge layer

### Step 2

Define authored `publish/...` schemas for each channel family.

Status:

- started and partially implemented
- current authored schema entrypoints are:
  - `publish/codex/marketplace.yaml`
  - `publish/claude/marketplace.yaml`
  - `publish/gemini/gallery.yaml`
- current implementation validates those schemas during package discovery
- current implementation surfaces them through `plugin-kit-ai inspect`

### Step 3

Generate and validate publication artifacts from:

- `plugin.yaml`
- `targets/...`
- portable authored files
- `publish/...`

Status:

- started and partially implemented
- current implementation renders and validates the Codex repo-level marketplace artifact `.agents/plugins/marketplace.json`
- current implementation renders and validates the Claude marketplace artifact `.claude-plugin/marketplace.json`
- current implementation also provides `plugin-kit-ai publication materialize --target codex-package|claude --dest <marketplace-root>` as the safe local marketplace-root workflow
- current implementation also provides `plugin-kit-ai publication remove --target codex-package|claude --dest <marketplace-root>` as the safe local marketplace-root pruning workflow
- current implementation also provides `plugin-kit-ai publication doctor --dest <marketplace-root>` as the local-root verification workflow for already materialized Codex or Claude marketplace roots
- current implementation also provides `--dry-run` on local materialize/remove workflows so local marketplace mutations can be previewed before writing
- current implementation also exposes `plugin-kit-ai publish --format json` as the versioned `plugin-kit-ai/publish-report` contract for top-level bounded publish workflows
- current implementation does not generate a separate Gemini gallery artifact because official Gemini docs do not define one
- current implementation instead validates Gemini gallery publication metadata, surfaces it through `plugin-kit-ai inspect`, and exposes `plugin-kit-ai publish --channel gemini-gallery --dry-run` as a bounded repository or release publication plan with factual Git and GitHub readiness checks
- current implementation now exposes `publish --all --dry-run` as an authored-channel orchestration plan across those workflow classes
- current implementation intentionally does not expose `publish --all` apply mode, because local marketplace materialization and Gemini repository/release planning are still different workflow classes

### Step 4

Add optional workflow automation such as:

- publish validation
- repo sync helpers
- PR generation
- release-channel orchestration

## Fixed decisions

### 1. `publish/...` stays separate from `plugin.yaml`

`(🎯 10/10) (🛡️ 10/10) (🧠 3/10)`

### 2. Publication channels are vendor-specific families

`(🎯 10/10) (🛡️ 10/10) (🧠 4/10)`

### 3. Vendor manifests remain generated artifacts

`(🎯 10/10) (🛡️ 10/10) (🧠 4/10)`

### 4. Gemini is modeled as a gallery family, not a marketplace clone

`(🎯 10/10) (🛡️ 10/10) (🧠 4/10)`
