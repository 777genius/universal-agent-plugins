# ADR 0009: Unified UAP dependency manifest

## Status

Accepted (2026-10-18). Implementation and executable release qualification are
separate from this decision. Extends ADR 0005 for dependency management after
its original v0.1 scope; preserves that record's historical boundaries.

## Context

UAP already has root intent, an exact graph lock, a verified package store and
per-client installation transactions. Package requirements need one authoritative
declaration that ships with the immutable artifact. Environment and package
requirements share source syntax, but only the selected environment controls
target assignments and exact graph pins.

## Decision

Use optional root `uap.toml` beside mandatory standard `plugin.json`. One
`[dependencies]` section describes direct mandatory requirements in both the
user environment and a published package. One shared decoder supplies two
context checks: a root needs target assignments; a package exports dependencies
and ignores structurally valid author target assignments. Published registry
requirements use canonical source IDs. Future project scope can use this format
without enabling project installation in this change.

The UAP manager reads requirements from verified snapshot bytes. The portable
standard decoder preserves JSON extensions without executing UAP dependency
semantics. Presence of the retired JSON dependency key is an admission error,
including alongside TOML. There is no dual read, conversion step or fallback.

`plugin.json` alone defines standard identity, metadata and components.
Agent Plugins [manifest rules](https://agent-plugins.org/specification#5-manifest)
prohibit another file from supplementing or overriding those core fields.
The [authoring guide](https://agent-plugins.org/plugin-authors/build-an-agent-plugin#package-boundaries)
places installation and distribution outside the portable contract. Accordingly,
this sidecar defines UAP manager behavior and makes no claim that other Agent
Plugins clients understand its dependencies. Existing namespaced runtime/command
extensions retain their own contracts.

The selected root lock remains the sole exact graph authority; nested locks
cannot influence it. TOML and nested locks remain ordinary package bytes covered
by the existing tree digest. Declaration keys join locked nodes by canonical
source identity; stable node aliases continue to own receipts and bindings.

## Consequences

- Authors and users learn one dependency syntax without duplicating standard
  metadata or maintaining generated manifests.
- Managed install, replay and restore must verify the same full closure; a
  missing description capability cannot be interpreted as a leaf package.
- Root-only targets protect consumer installation authority. A dependency edge
  does not imply process startup order, readiness or universal component support.
- JSON-only historical dependency pins need an explicit new source revision;
  unsupported metadata does not block receipt-authorized cleanup or existing
  transaction recovery.
- A binary unaware of package TOML cannot safely reproduce a new dependency
  graph. Rollback must preserve exact files, store and receipts, with pending
  recovery completed using the appropriate binary.

## Non-Goals

Project activation, dev groups, new client adapters, a second resolver, an
environment-wide transaction journal, a new publisher and release publication.

## Rejected Alternatives

- Keep requirements in JSON extensions as a second source: precedence would
  obscure the single declaration contract.
- Separate package and environment dependency sections: both express the same
  direct requirement relation; only target and lock authority differ.
- Generate a namespaced manifest during packaging: adds an unnecessary build
  step and another representation whose consistency must be verified.
- Infer declarations from lock edges: cannot detect a removed or forged edge.

Implementation and acceptance details live in the
[delivery plan](../plans/uap-unified-dependency-manifest.md).
