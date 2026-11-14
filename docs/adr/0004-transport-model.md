# ADR 0004: Transport Model

## Status

Accepted

## Context

The current shipped runtime is process-per-hook through stdin/stdout. Historical roadmap text also described daemon and hybrid execution modes as if they were part of the intended core contract.

The rewrite needs transport decisions that keep one runtime architecture and prevent daemon support from becoming a second stack with separate semantics.

## Decision

The first transport for the rewrite is process transport.

Daemon and hybrid modes are later transports layered on the same engine, descriptors, handler model, and response policy.

There is no second runtime stack for daemon mode.

Transport-specific concerns must be modeled as explicit inputs or ports, including:

- target platform
- path resolution
- process spawning
- socket or HTTP listener setup
- checksum or release source selection where transport/install boundaries interact
- environment lookup required for transport behavior

Use cases must not read ambient runtime environment directly when those reads affect transport or installation decisions.

Process transport defines the baseline correctness model. Later transports must preserve the same descriptor-driven semantics and differ only in invocation mechanism and lifecycle management.

## Consequences

- Process transport can be implemented and validated first without deferring architectural decisions.
- Daemon and hybrid additions later reuse the same engine and descriptor contracts.
- Tests can prove semantic equivalence across transports instead of validating two runtimes.
- Installer and runtime boundary decisions become easier to isolate because environment-sensitive behavior is pulled into explicit ports and parameters.

## Non-Goals

- Designing full daemon lifecycle behavior in Phase 1.
- Choosing specific socket types, HTTP frameworks, or backgrounding policy now.
- Encoding transport selection into the stable platform-specific handler API.

## Rejected Alternatives

- Build a separate daemon runtime optimized for long-lived plugins.
  This would duplicate semantics, split tests, and make support drift likely.

- Let daemon mode own a different handler API than process mode.
  That would make transport a public contract concern instead of an execution concern.

- Keep transport behavior implicit through ambient environment reads in use cases.
  That hides control flow, weakens testability, and makes install/runtime behavior harder to reason about.
