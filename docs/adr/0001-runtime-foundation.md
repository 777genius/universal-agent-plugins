# ADR 0001: Runtime Foundation

## Status

Accepted

## Context

The current shipped SDK runtime is Claude-only. `plugin-kit-ai.App` wires `ports.ClaudeWireCodec`, the dispatcher switches on three Claude command-hook names, and core/runtime concerns are mixed with Claude-shaped event contracts.

The rewrite needs a runtime foundation that does not treat any single platform as the architectural center, while still keeping platform-specific APIs as the public contract.

## Decision

The VNext runtime foundation uses these cross-cutting primitives:

- `PlatformID`
- `EventID`
- `Envelope`
- `Codec`
- `Handler`
- `ResponsePolicy`
- `Middleware`
- `Transport`
- `ExecutionMode`

The runtime core is responsible only for:

- identifying the incoming platform/event
- decoding payloads through a platform/event codec binding
- applying middleware
- invoking the registered typed handler
- encoding the response
- applying platform-specific response and exit semantics

The runtime core keeps only cross-cutting concepts:

- plugin metadata
- lifecycle
- diagnostics
- middleware
- event identity
- result envelopes

Platform-specific event structs and wire DTOs live only in platform packages.

The stable public contract is platform-first. Registration is explicit per platform, and each registrar exposes only events that actually exist for that platform.

The unified API is a secondary facade and does not define the shape of the core runtime.

No core package may import any platform package.

Codex is the first platform used to validate this runtime foundation, but the runtime itself must not encode Codex-specific assumptions either.

## Consequences

- The pre-redesign `plugin-kit-ai.App` shape is historical context, not a compatibility anchor.
- The current switch-based dispatcher and `ClaudeWireCodec`-centered core are replaced, not generalized in place.
- Core logic can be tested without importing any concrete platform event types.
- Platform packages own their own event structs, wire codecs, manifest metadata, and response rules.
- Future platforms can be added without changing the runtime core if descriptors and codecs are sufficient.

## Non-Goals

- Preserving `plugin-kit-ai.New()`, `OnStop()`, `OnPreToolUse()`, `OnUserPromptSubmit()`, or the pre-redesign `Run()` shape as compatibility facades.
- Defining the full Go package tree or naming every internal file ahead of implementation.
- Introducing a unified event model that erases platform-specific semantics.

## Rejected Alternatives

- Keep `App` as a compatibility facade over the new runtime.
  This would preserve Claude-shaped assumptions at the public boundary and would constrain the runtime around old lifecycle and registration semantics.

- Make unified registration the primary public contract.
  This would make the least stable layer define the architecture and would force the runtime to optimize for partial semantic overlap instead of real platform contracts.

- Continue evolving the current dispatcher by adding more `switch` cases and codec methods.
  That path scales operationally but not architecturally. It preserves platform-specific coupling in the core and duplicates contract definitions across runtime, docs, and scaffold.
