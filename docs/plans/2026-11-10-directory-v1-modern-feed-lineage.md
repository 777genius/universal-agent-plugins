# Separate Directory v1 signing lineages: minimal CLI foundation

Date: 2026-11-10. Status: **Draft / implementation incomplete**. Companion to [Registry Pi #451](https://github.com/777genius/universal-agent-plugins-registry/pull/451).

## Owner decision and non-goals

Prioritize a modern UAP CLI (0.1.102+) that can eventually load a distinct, editor-approved Ed25519-signed **Directory schema 1** feed with Pi target grants. Existing 0.1.89 readers **must continue consuming the original feed unchanged**. Do not retrofit old binaries, add Directory v2, add feature flags, or treat a source `--dry-run` as signed authority. Do not infer approval for a second signing key or unprotected publication.

## Verified architectural problem

Before this change, `directoryFloorForClient` separated installed sequence high-water marks by `SnapshotSchema` alone. Two unrelated signed schema-1 feeds would share a rollback floor, despite requiring independent signing identities. Example: the old feed has sequence 43 and a new feed starts at 1. Either feed must not borrow or lower the other's floor. Production also uses a single `directory-v1-cache.json`, one embedded bootstrap, and an old-feed signing key. Changing only the URL is unsafe.

## Phase 1 in this Draft PR

- [x] Add an optional `DirectoryOrigin.FeedID` receipt. Empty/missing identity means the legacy feed, preserving existing serialized data and old reader behavior.
- [x] Make the trusted `directoryv1.Client` carry a feed identity in memory (never obtained from a remote pointer, environment variable, or package manifest).
- [x] Scope installed schema-1 sequence floors by **both schema and feed identity**; a client with the old empty identity retains its historical high-water mark.
- [x] Persist the authenticated client's feed identity when creating/refreshing signed Directory provenance, not when a direct local/Git source is installed.
- [x] Add regression tests for old sequence 43, modern sequence 5, unknown lineage zero, and independent schema-2 sequence 3.
- [ ] Complete exact-head Go unit/CLI tests and independent review before merging.

This does **not** enable a second feed, change the old production URL/key/cache, sign a policy, add a bypass, or advertise Pi availability. The new identity is intentionally unused by the shipped production factory until reviewed bootstrap material exists.

## Subsequent scoped delivery, not silently merged with Phase 1

1. **Trust and publication**: approve distinct modern v1 origin, public Ed25519 key identity, immutable signing ledger, protected promotion, and source-policy qualification. Registry #451 must prove 72/72 actual saved dependency eligibility in an **isolated signed modern test cohort**; current historical-feed 70/72 is a genuine denial for Marketing Kit -> HubSpot CRM, not an acceptable pass. Exclude two suspended scope cases. Preserve byte-identical old feed.
2. **CLI routing**: wire a compiled, authenticated modern `directoryv1.Client` with its own embedded signed bootstrap and `directory-modern-v1-cache.json`. Do not allow URL/env selection to switch signing identity, implicit trust migration, or fallback to legacy authority. Keep legacy receipts and cache readable separately.
3. **Installation lineage**: bind signed short-name selections, saved dependency locks, update/repair/revocation paths, and restored installations to their source feed. The `FeedID` field is a foundation, **not** a complete cross-feed lock migration. Reject cross-lineage substitution even when product/release names match.
4. **E2E**: released-CLI signed short-name `add/info/update/repair/remove` on real Pi in user/project on Linux/macOS/Windows. Negative tests: wrong key, same-sequence equivocation, stale sequence, cache loss, revoked dependency, unexpected OAuth requirement, altered digest, foreign file preservation, and legacy 0.1.89 reading the original warm/remote feed.
5. **Release and postmerge**: protected editor approval, signed publication and CDN inspection, independent signature/sequence verification, release a new CLI version, and verify the website's Pi selector from the **actual signed** modern snapshot. Disclose the minimum modern CLI version and old-client impact.

## Review conclusions / risks

Round A — security: feed identity is not trusted simply because it is present in persisted state; only a compiled, signature-verifying client can designate it. This change keeps authorization on the existing Ed25519 path and does not add a signer.

Round B — backwards compatibility: `omitempty` and empty legacy identity preserve existing persisted receipts and floor behavior. A separate identity must never be assigned to old installation records automatically.

Round C — rollout: **no modern feed activation until** a protected signed bootstrap, independent trust/cache, signed dependency resolution, and 3-OS E2E are proven. A test-only signer cannot become production trust.

Estimated follow-on work: scoped CLI routing/state enforcement **8/10 complexity, 9/10 security risk, ~450–1,000 Go LOC plus tests**; separate protected registry feed/promotion **8/10 complexity, 9/10 risk**. Prefer reuse of the existing directoryv1 client, cache, and Ed25519 verifier, not a second installer engine.
