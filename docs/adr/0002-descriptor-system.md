# ADR 0002: Descriptor System

## Status

Accepted

## Context

The current repository duplicates platform contract knowledge across code, docs, templates, and tests. Runtime wiring, README matrices, scaffold content, and validation rules are edited independently. This creates drift and makes unsupported APIs look more real than the code behind them.

The rewrite needs one source of truth for per-platform event contracts and the outputs derived from those contracts.

## Decision

Descriptor granularity is one hand-authored descriptor per platform/event pair.

Each descriptor owns:

- wire schema
- decode and encode binding
- exit semantics
- block/allow behavior
- manifest metadata
- scaffold metadata
- unified capability tags
- docs snippet metadata
- validation metadata

Descriptors are authored by humans. Generated outputs are derived from descriptors and are not edited as peer sources of truth.

Required generated outputs:

- typed registrars
- runtime wiring tables
- docs snippets and capability tables
- scaffold data
- coverage matrix
- validation rules

No long-term parallel hand-maintained matrix is allowed across runtime/docs/templates/tests once the descriptor system lands.

Generator output is derived from descriptors, not from README prose, example code, or template duplication.

## Consequences

- Adding or changing a supported platform event becomes a descriptor-first change.
- Docs, validation, scaffold, and runtime wiring stay aligned by construction instead of by review discipline.
- Platform support claims become auditable because every support claim must trace back to a descriptor.
- Generated outputs become disposable artifacts; descriptors remain the primary maintained contract.

## Non-Goals

- Generating every line of platform implementation code.
- Eliminating hand-authored codecs or response logic where platform semantics are genuinely custom.
- Using descriptors as a public user-editable configuration surface in Phase 2.

## Rejected Alternatives

- Derive descriptors from runtime code after codecs and handlers already exist.
  That makes runtime code the hidden source of truth and keeps docs/scaffold drift as a downstream problem.

- Keep separate handwritten runtime tables, scaffold metadata, and README matrices.
  This is the current failure mode. It makes support claims cheap to write and expensive to verify.

- Use one descriptor per platform instead of per event.
  That produces coarse records that are harder to validate, harder to generate from, and too easy to leave partially implemented.
