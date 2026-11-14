# Foundation Rewrite VNext

## Status

Archived maintainer reference. This document records the design target that led to the current descriptor-first runtime shape. It is not the current public contract.

Current shipped contract lives in:

- [../README.md](../README.md)
- [SUPPORT.md](./SUPPORT.md)
- [STATUS.md](./STATUS.md)
- [generated/support_matrix.md](./generated/support_matrix.md)

## Core Position

- Codex is Platform #1 for the rewrite.
- Platform-specific APIs are the stable public contract.
- Unified API is a second-layer facade built from explicit capability mappings.
- Backward compatibility with the current Claude-shaped runtime is intentionally dropped.

Codex is not just another Tier-1 platform in the plan. It is the first platform used to set naming, descriptors, scaffold defaults, docs structure, validation rules, and acceptance gates for the new foundation.

## Goals

- Replace the current Claude-specific runtime with a platform-agnostic event engine.
- Move all platform wire contracts into per-platform packages.
- Make descriptors the single source of truth for runtime wiring, docs, scaffold output, manifest metadata, validation, and coverage.
- Keep unified abstractions narrow and capability-driven instead of using them as the core abstraction.
- Remove roadmap-heavy public API and docs that are not backed by shipped code.

## Non-Goals

- Preserving `plugin-kit-ai.New()`, `OnStop()`, `OnPreToolUse()`, `OnUserPromptSubmit()`, or the current `Run()` contract.
- Keeping Claude as the architectural reference model.
- Supporting speculative unified prompt, agent, or platform-unique workflows.
- Reintroducing plugin commands into the runtime core.

## Architecture Decisions

### 1. Generic engine, zero platform types in core

The new core is built around cross-cutting runtime primitives only:

- `PlatformID`
- `EventID`
- `Envelope`
- `Codec`
- `Handler`
- `ResponsePolicy`
- `Middleware`
- `Transport`
- `ExecutionMode`

Core runtime orchestrates decode, middleware, handler invocation, encode, and platform exit semantics. It must not import Claude, Codex, or any other concrete platform types.

### 2. Descriptor-driven system

Each platform event is described by a hand-authored descriptor. A descriptor is the single source of truth for:

- wire schema and codec binding
- exit semantics
- block/allow behavior
- manifest metadata
- scaffold metadata
- unified capability tags
- docs snippets
- validation rules

Generated outputs must include:

- typed registrars
- runtime wiring tables
- scaffold data
- docs snippets and capability tables
- descriptor coverage matrix
- validation rules

Parallel hand-maintained runtime/docs/template definitions are explicitly out of scope.

### 3. Package boundaries

The monorepo should be reorganized around:

- `core`
- `platforms`
- `unified`
- `transport`
- `generate`

Per-platform wire logic lives only under `platforms/<name>`. Platform event structs also live there. Core keeps only cross-cutting concerns such as lifecycle, diagnostics, middleware, event identity, result envelopes, and plugin metadata.

### 4. Public API

The main contract becomes explicit per-platform registration:

- `app.Codex()`
- `app.Claude()`
- `app.Gemini()`
- `app.Copilot()`
- `app.Cursor()`
- `app.Windsurf()`

Each registrar exposes only events that actually exist for that platform.

Unified registration is separate:

- `app.Unified()`

Unsupported unified hooks must fail at registration time with descriptor-backed errors. No silent downgrade. No fake partial mapping.

### 5. Codex-first product defaults

Codex sets the first implementation bar for:

- naming conventions
- example plugins
- scaffold defaults
- validation rules
- smoke tests
- CLI examples
- generated docs

Claude is ported later as a peer platform on the same architecture, not as the base model.

## Delivery Phases

### Phase 1. Freeze and clear the ground

- Keep current behavior only as regression fixtures.
- Write ADRs for runtime shape, descriptor model, unified capability policy, and transport model.
- Delete or quarantine roadmap-heavy docs that describe non-shipped APIs.
- Mark pre-redesign architecture docs as historical maintainer context only.

### Phase 2. Build the new engine with Codex first

- Implement descriptor system.
- Implement new core engine.
- Implement process transport.
- Port Codex fully onto the new architecture with no compatibility shims.
- Make Codex the default scaffold and documentation path.

### Phase 3. Descriptor-driven CLI on top of Codex

- Rewrite `cli` around real use cases such as `init`, `validate`, `capabilities`, and `install`.
- Make scaffold an adapter over template/file-system ports instead of mixed application logic plus `os` plus embed assumptions.
- Add `plugin-kit-ai validate` for manifest correctness, supported event set, unified capability support, and scaffold consistency.

### Phase 4. Port Claude as a peer platform

- Port Claude onto descriptors and the new engine.
- Delete the old Claude-hardcoded dispatcher and `ClaudeWireCodec`-centered design.
- Keep no compatibility facade around the previous runtime shape.

### Phase 5. Add remaining Tier-1 platforms

- Add Gemini, Copilot CLI, Cursor, and Windsurf through the same descriptor pipeline.
- A platform is not done until codec, manifest metadata, scaffold metadata, docs, validation, and smoke coverage exist.

### Phase 6. Add unified capabilities after proof

- Add the unified layer only after Codex, Claude, and at least one more platform are live.
- Unified capabilities must be proven by descriptor-backed semantic overlap across platforms.
- Prompt, agent, and other platform-unique workflows remain platform-specific.

### Phase 7. Add more transports without a second runtime

- Add daemon and hybrid transports on top of the same engine and descriptors.
- No second runtime stack is allowed.

## CLI And Installer Decisions

- `plugin-kit-ai init` becomes descriptor-driven and platform-aware.
- Default scaffold target is `codex`.
- Generated examples use Codex handlers first.
- README sections, SDK docs, scaffold templates, and capability tables should be generated or descriptor-verified.

Installer remains a separate subsystem, but environment decisions must move out of the use case. Target platform, path resolution, checksum verification, asset selection policy, and release source become explicit inputs or ports.

## Testing Requirements

- Golden decode/encode tests for every platform event and response mode.
- Descriptor contract tests asserting every event has runtime wiring, manifest metadata, scaffold metadata, docs coverage, and capability tags.
- Fuzz or property tests for codecs and response policies.
- Cross-platform unified tests for block/allow semantics, unsupported registration, exit-code behavior, and field mapping.
- Repotests for generated plugins across Tier-1 platforms: scaffold, build, `go test`, `go vet`, smoke execution, validate flow, install flow, and doc/example sync.
- Live tests remain opt-in and validate only platform and installer boundaries.

## Acceptance Bar

The rewrite is not complete until all of the following are true:

- the old hardcoded dispatcher is deleted
- Codex-first scaffold works end-to-end
- docs are descriptor-generated or descriptor-verified
- Codex is fully green on codec, scaffold, validate, smoke, and install flows
- Claude, Gemini, Copilot CLI, Cursor, and Windsurf can be added without changing core architecture

## Immediate ADR Topics

- runtime primitives and execution model
- descriptor schema and generation boundaries
- unified capability admission policy
- transport model and process contract
- package boundaries for `core`, `platforms`, `unified`, `transport`, and `generate`
