# ADR 0003: Unified Capability Policy

## Status

Accepted

## Context

The current docs describe a large unified API surface that is not implemented. More importantly, cross-platform overlap is usually partial and semantically uneven. If unified abstractions become the primary contract, the runtime will be shaped by the least precise layer.

The rewrite needs a strict admission policy for unified capabilities so that unified APIs remain honest and secondary.

## Decision

The unified layer is a separate package and a separate registration surface from platform registrars.

Unified capabilities are admitted only when semantic overlap is proven across at least three live platforms on the new runtime.

The burden of proof is on semantic overlap, not on field-name similarity. A capability may be unified only when these conditions hold:

- the trigger meaning is materially the same across the candidate platforms
- block/allow semantics can be expressed without hidden downgrade
- response and exit behavior can be defined honestly
- the mapped fields have stable cross-platform meaning

Unsupported unified registrations fail immediately at registration time with descriptor-backed errors.

The unified layer is never allowed to silently downgrade, partially fake, or register a hook that cannot be honored on the selected platform set.

Prompt, agent, notification-like, and other platform-unique workflows remain platform-specific unless later evidence proves a real shared capability.

## Consequences

- Platform-specific APIs stay the stable contract users can rely on.
- Unified APIs stay smaller, more honest, and easier to document.
- Cross-platform tests can validate unified behavior against explicit capability mappings instead of ad hoc translations.
- Unsupported combinations fail early and visibly.

## Non-Goals

- Maximizing unified API surface area.
- Providing a fallback abstraction for every platform event.
- Hiding platform differences that matter operationally.

## Rejected Alternatives

- Make unified the primary contract and map platform specifics under it.
  This pushes semantic mismatch into the core architecture and encourages fake overlap.

- Allow best-effort or partial unified registration with warnings.
  That creates support ambiguity and makes runtime behavior depend on hidden downgrade rules.

- Admit unified capabilities as soon as two platforms seem similar enough.
  Two-platform overlap is too weak a signal and makes premature abstractions likely.
