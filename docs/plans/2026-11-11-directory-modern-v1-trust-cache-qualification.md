# Signed Directory v1 dual-feed trust/cache test qualification

Date: 2026-11-11. Purpose: provide executable negative evidence for the modern-only Pi v1 lineage before any protected key, production endpoint, or routing is enabled. Complement merged #837/#844 (feed-scoped floor, state, and lock); do not duplicate those tests.

## In scope
- Use the existing test-only Ed25519 conformance fixture and real verified `directoryv1.Client` for two distinct signed schema-1 feeds.
- Parameterize **only the test fixture's seed**, retain the original deterministic legacy fixture unchanged.
- Verify separate trust keys/HTTPS origins/cache paths and positive authenticated remote+local reads for both independent lineages.
- Negative cross-key: the modern client's trust must not verify the legacy signed bootstrap or cache even if key IDs and schema/sequence are the same; the reverse must also fail.
- Positive isolation: a modern remote load must not rewrite the legacy cache, and a legacy publication must not authenticate in the modern cache. The test-only modern `FeedID` is nonempty; legacy remains empty.
- Use existing hardened `Client.Load`, `Embedded.Verify`, and `Cache.Load` APIs; no additional signature scheme, cache implementation, hidden runtime flag, or public API.

## Out of scope / required later
This is a negative **test fixture** only, not a new production signing key, trusted bootstrap, modern origin, editor approval, signed Pi policy, 72/72 saved Marketing Kit eligibility, OAuth, user-facing redirect, or released-CLI short-name E2E. No legacy source/CI/signing controls change. The actual protected modern feed needs a separate scoped, approved implementation and 3-OS qualification before activation.

## Review
- R1 cryptographic: ensure genuinely different test keys and do not confuse equal key IDs with equal trust.
- R2 cache/rollback: verify source boundary and no shared writable pathname, retain existing anti-rollback.
- R3 scope: tests only plus this plan; no production branch changes, no artificial positive Pi claim.
- Required CI: Go Core fast, Required, package manager and static checks on exact head; reviewer comments resolved before merge.
