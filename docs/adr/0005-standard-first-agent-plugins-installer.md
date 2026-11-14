# ADR 0005: Standard-First Agent Plugins Installer

## Status

Accepted

## Context

`plugin-kit-ai` already has a transactional lifecycle for its legacy `plugin/plugin.yaml` authoring format. Agent Plugins 1.0 defines a portable package with root `plugin.json`, optional root `mcp.json`, and immediate `skills/<name>/SKILL.md` components. The standard does not define client detection, installation layout, state, rollback, or distribution.

Treating the standard package as a legacy `IntegrationManifest` would lose unknown fields, component-level diagnostics, schema identity, and future standard extensions. Letting each client adapter parse the package would duplicate policy and make upgrades inconsistent.

## Decision

The `agentplugins` product is a separate CLI and npm facade over shared lifecycle infrastructure in this repository.

Its standard-first path is:

```text
source -> immutable PackageSnapshot -> versioned Agent Plugins loader
       -> lossless PackageEnvelope -> client detector -> delivery planner
       -> transactional lifecycle kernel -> client providers
```

The following are invariants:

- root `plugin.json` is the canonical package manifest;
- root `mcp.json` and immediate `skills/<name>/SKILL.md` are optional components;
- official Agent Plugins 1.0 schemas are embedded and pinned by digest;
- loading and validation never require network access;
- unknown standard fields are preserved losslessly;
- invalid MCP or skill components produce isolated diagnostics and do not erase valid components;
- a `PackageEnvelope` is never converted to legacy `IntegrationManifest`;
- client detection is read-only and separate from mutation planning;
- paths, credentials, OAuth, publishing, and client policy are not added to `plugin.json`;
- local and remote sources are copied into a private immutable snapshot before parsing or planning;
- physical package IDs pass one portable leaf-name invariant before path construction;
- state v2 uses a separate file and installation identity; legacy state remains a read-only migration source;
- update remains bound to loader kind, source identity, package identity, and digest;
- each client binding records the exact package revision applied to it, so multi-client updates converge independently without a false no-op;
- destructive mutations require owned-path containment and durable intent/receipt records;
- all State v2 mutations use one cross-process OS lock; dry-run and doctor never acquire it;
- confirmed mutations recover any open directory journal before replanning against authoritative state;
- desired package state is committed only after the staged directory is active and verified;
- v1 migration is explicit, digest-bound to the reviewed plan, and backed up only after complete State v2 validation;
- migrated legacy removal delegates to the original lifecycle and then reconciles State v2; it never interprets `plugin.yaml` as Agent Plugins 1.0;
- client-native trust prompts and discovery checks remain manual until an explicit terminal or client verification contract exists;
- v0.1.0 does not publish npm or change a dist-tag without explicit owner approval.
- the first npm publication is a separate protected bootstrap using a short-lived token; only after package ownership and the exact trusted-publisher binding exist may later tags use OIDC.

Release operations follow the protected
[`agentplugins` release runbook](../agentplugins-release.md).

The legacy `plugin/plugin.yaml` loader remains a separate front door. It is not read by `agentplugins add`, and discovery of a later `plugin.json` does not silently switch an existing legacy installation.

`plugin-kit.yaml` is not part of an installed package. No orchestration sidecar is introduced in v0.1.0.

## Consequences

- The standard can evolve without forcing every client adapter or legacy manifest type to change in lockstep.
- Existing process, filesystem, locking, and transaction primitives can be hardened and shared without sharing the legacy package model.
- Standard conformance, client compatibility, and runtime verification remain distinct evidence claims.
- State migration and client providers require explicit new contracts instead of a small parser-only patch.
- `agentplugins` can later move to its own repository after two stable releases without forking the engine.

## Non-Goals

- Defining a marketplace or authoring format.
- Storing or automating OAuth credentials.
- Installing into every detected client without explicit selection.
- Supporting plugin dependencies, background updates, telemetry, or silent adoption in v0.1.
- Publishing the npm package as part of implementation or CI validation.

## Rejected Alternatives

- Convert Agent Plugins packages into `IntegrationManifest`.
  This is lossy and couples future standard evolution to a legacy target-oriented model.

- Parse `plugin.json` independently inside each client adapter.
  This duplicates schema policy and creates incompatible interpretations.

- Make `plugin-kit.yaml` an installed sidecar.
  This creates a competing package standard and pressures authors to maintain two manifests.

- Build a second installer engine for the npm-facing product.
  This duplicates state, rollback, recovery, and client adapter behavior.

- Install short names from a mutable default branch.
  This makes versions non-reproducible and weakens supply-chain binding.
