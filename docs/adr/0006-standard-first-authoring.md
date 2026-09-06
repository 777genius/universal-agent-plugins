# ADR 0006: Shared Standard-First Authoring

## Status

Accepted for bounded implementation, 2026-09-06, under the approved
[implementation plan](../STANDARD_FIRST_AUTHORING_ENGINE_IMPLEMENTATION_PLAN.md).
This decision extends [ADR 0005](./0005-standard-first-agent-plugins-installer.md);
it does not replace or relax installer invariants. Acceptance is architectural,
not a claim that the standard-first CLI is released.

## Context

The current `plugin-kit-ai` v1 authors `plugin/plugin.yaml`. Agent Plugins 1.0
uses root `plugin.json`, optional root `mcp.json`, and immediate
`skills/<name>/SKILL.md`. Reusing the legacy application behind a renamed
command would preserve the wrong authority and conceal format conversion.
Cobra commands also retain parent pointers, flag values, and streams, making
package globals unsuitable for two independent entrypoints.

## Decision

1. Root `plugin.json` is the sole portable source of identity and metadata.
   `mcp.json` and `skills/` are the portable component sources. No required
   sidecar, generated client projection, build file, or publication metadata
   may supplement, override, or repair portable fields.
2. Normal authoring accepts exactly `FormatIDAgentPluginsV1`. The OpenAI
   compatibility layout `.codex-plugin/plugin.json` is a separate explicit
   input format. It never supplies missing standard files. Legacy input is
   accepted only by a later explicit read-only importer at the canonical
   `<project>/plugin/plugin.yaml` path, writing to a new destination.
3. `plugin-kit-ai` and `agentplugins author` construct fresh commands from
   the same Go factories and call the same services in process. Neither invokes
   the other binary. Each command injects its own typed request/result runner;
   there is no application-wide backend interface or legacy manifest adapter.
4. Reuse bounded decoding and the lossless `PackageEnvelope` beside the standard
   domain. Keep mutable authoring roots, snapshot ownership, CAS plans, and
   archive selection outside the installer domain. Preserve unknown values and
   supported opaque extensions; never serialize the envelope into legacy types.
5. Separate normative conformance, host safety, installer policy, authoring
   hygiene, release/publication policy, target compatibility, and runtime
   evidence. A skipped invalid MCP document/server or Skill does not erase
   valid components and does not constitute full authoring conformance.
6. Embed exact schema and Agent Skills validation profile identities. Validation
   never fetches rules, launches an agent/model, reads client credentials, or
   executes package content. Runtime/network testing belongs to explicit later
   phases with disposable roots and bounded process cleanup.
7. Existing installer flags stay at the root. The adapter copies only supported
   `--format`, `--no-color`, `--dry-run`, and `--target` into a fresh value.
   Explicit `--scope`, `--accept-security-risk`, and `--security-details`, even
   explicit false/default values, fail before authoring decoding or effects.
   Unsupported explicit shared flags also fail. Help explains the positional
   package path, authoring doctor, and separate installer policy commands.
8. Equivalent commands preserve exit errors, JSON data/envelopes, diagnostics,
   and filesystem effects. Only product branding, invocation prefix, and product
   version may differ; successful future authoring payloads carry the same
   engine revision. Renderers own existing output contracts, and JSON mode
   emits one document without prompts. No new exit-code mapping is introduced.
9. Keep Go module identities `github.com/777genius/plugin-kit-ai/...`. A future
   rename requires a separate compatibility ADR and consumer inventory.

Dependency directions are enforced by the foundation import-boundary test:

```text
authoringcli -> command-specific ports and renderers
authoring services -> standard facts + author policy + low-level infrastructure
installer loader/policy -> standard facts (never authoring)
standard domain/conformance -> no Cobra, mutation, clients, or publication
legacy importer -> standard mutation plan (migration only)
standard authoring -X-> pluginmanifest/pluginmodel/app/publicationmodel/legacyimport
```

The [command and policy inventory](./0006-authoring-inventory.md) records
conversion decisions and the failure classification handoff. Host restrictions
must never be reported as normative schema failures. In particular, Windows
leaf IDs, archive symlink restrictions, and publication SemVer requirements are
stricter policy; a non-SemVer string is not itself an Agent Plugins 1.0 violation.
Unknown fields and non-object extension containers may load with diagnostics
while still failing schema conformance. Unknown namespaces stay opaque.

## Consequences

The bounded foundation adds factories, an explicit flag adapter, tests, this
ADR, and historical markers. It deliberately leaves v1 command wiring and
installer behavior intact, without moving hundreds of legacy files. The
existing main is already a small execution composition point; individual legacy
constructors remain until each replacement has a complete standard service.
New factory roots are internal seams, not public success-shaped placeholders.
No released main imports them yet. The author subtree is hidden when explicitly
mounted by internal integration code.

The first public PR must integrate a working `init -> validate -> inspect ->
test` E2E slice. Foundation-only changes may be committed for orchestration but
must not become that first public PR. The complete MVP/release gates still
require generated Skill and MCP packages, both entrypoints, isolated installer
planner checks, unchanged installer tests, and matching engine provenance.
Runtime, migration, preview, publication, and retirement retain their later
phase gates. Historical artifacts and v1 binaries remain immutable.

Factories allocate fresh Cobra trees and flag sets; callers must not cache a
command or share mutable configuration captures. Runner, decoder, and renderer
implementations must honor cancellation and stream/JSON contracts. The factory
resets parsed flags after its argument rejection or runner completion, including
runner/render errors. Cobra parsing errors and ancestor pre-run failures occur
outside that lifecycle: construct a fresh tree after those failures. Trees are
not safe for concurrent Execute calls; independent trees may execute concurrently
with concurrency-safe injected runners.

## Non-Goals

This foundation neither implements later services nor changes installer flag
placement, lifecycle, state, trust, cache, rollback, or providers. It neither
publishes a v2 binary nor deletes legacy code. It does not register deferred
commands, implement a second manifest, or establish an authoring runtime SDK.

## Rejected Alternatives

- Mechanical movement of hundreds of legacy files: obscures domain coupling
  and expands the review without a standard-first capability.
- A single reusable `*cobra.Command`: leaks parent pointers and mutable flags.
- Shelling out to v1 or generating temporary YAML: preserves the legacy engine
  and creates version, quoting, cancellation, and error drift.
- Relaxing the installer loader to get green authoring results: confuses
  conformance with security and weakens an established safety boundary.
- Replacing the current public root before the vertical slice: breaks working
  v1 behavior and advertises unfinished commands.
