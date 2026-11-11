# Bind saved Directory v1 locks to their signed feed lineage

Date: 2026-11-11. Status: **Draft; validation pending**. Companion to [Registry Pi #451](https://github.com/777genius/universal-agent-plugins-registry/pull/451) and merged CLI foundation #837.

## Problem and scope

The CLI already isolates per-feed installation receipts and anti-rollback floors, but `uap.lock` retains only product, distribution, release, Git revision and digest. A future second Ed25519-signed v1 client could therefore reauthorize a legacy locked dependency under a different signing lineage if these values coincide. Source bytes alone are not the signature authority.

Make the smallest backward-compatible addition:

1. Persist optional `source.registry.feed_id` for newly acquired signed dependencies and top-level saved short-name installs, copied **only** from authenticated Directory provenance.
2. Require a restored lock's feed identity to match the configured trusted `DirectoryClient` **before** a network/cache lookup or source materialization. Switching source/feed remains an explicit operation.
3. Empty/missing `feed_id` remains the historical legacy v1 feed; do not migrate or reinterpret existing locks.
4. Regression-test modern and historical lock serialization and both cross-lineage directions.

## Boundaries and gates

- No new endpoint, Ed25519 key, signer, bootstrap, URL override, feed routing, Directory v2 or feature flag.
- No changes to old snapshots, signing ledger, source bytes or immutable dependency digests.
- This only closes a **lock lineage** boundary. It does not itself certify Pi, change signed policy, or turn Registry #451's real 70/72 into 72/72.
- Before merge: exact-HEAD Go tests, formatting, negative lifecycle tests and independent review. After integration: released CLI, signed short-name, user/project and three-OS acceptance in the separate modern-feed rollout.
- Complexity **3/10**, reliability target **9/10 after CI**, risk of legacy regressions **3/10**, implementation approximately **30 Go LOC plus tests**. Keep this PR Draft on any unexpected failure.

## Two-pass review

**Security:** `feed_id` is not user-authorized trust metadata. The trusted Directory client supplies the current identity and still verifies Ed25519 on its own; mismatched locks fail closed before fetching, without silent cross-feed fallback.

**Compatibility:** an omitted field retains its exact historical JSON form and empty legacy identity. No changes to the short-name default, existing cache, or installation ownership.
