# Pi modern Directory v1: pinned trust construction (2026-11-13)

Status: Draft / **not production routing or publication**. Companion to Registry #451 and UAP #900–#903.

## Minimal contract

Provide one reusable CLI composition-root constructor for an independently signed modern schema-1 Directory client. It accepts only already-reviewed, compiled public trust material (origin, FeedID, Ed25519 public key identity and embedded signed bootstrap). Require strict signature verification **before** constructing a client. Use the existing verifier, hardened HTTP transport and cache; do not implement a new installer, signature algorithm or schema.

Refuse empty/noncanonical feed identity, legacy origin or public key (including renamed keys), malformed HTTPS origins, an unsigned or wrong-key bootstrap, and missing state roots. Use a separate `directory-modern-v1-cache.json` with an independent feed high-water mark. Do **not** read trust material from environment variables; never fall back to the legacy signer.

This constructor is intentionally **not connected** to the released CLI. An editor-approved modern issuer/key/origin/bootstrap and reviewed Pi policy do not exist yet. Tests use only disposable Ed25519 signing fixtures; the ordinary legacy client retains its exact source and readers.

## Three review rounds

1. **Cryptographic/trust boundary:** signature must validate using the supplied independent Ed25519 key; reject legacy trust even under a different key ID, missing bootstrap and cross-signed replacements.
2. **Compatibility/state:** no changes to `newDirectoryClient`, the historical signed feed, old cache or persisted receipts. Verify distinct cache and that loading a modern fixture cannot modify the legacy cache or accept an environment-selected issuer.
3. **Scope/failure modes:** no runtime flags, signer credentials, new schema, generic feed registry or additional installer flow. Construct-only on invalid config, no filesystem writes. Verify fast Go tests and exact-HEAD Required/Core checks, review original CI logs before merge.

Estimate: ~80 production Go LOC, ~130 Go test LOC; scope complexity 3/10, trust boundary risk 7/10, confidence 7/10 pending exact-head CI and independent review.

## Remaining acceptance (not satisfied here)

Real independently reviewed source/materialization receipts for all Pi grants; owner/editor-approved modern signing key, dedicated protected origin/ledger/CAS, source and policy approvals; compiled immutable bootstrap and trusted per-target routing across read/info/add/update/repair/remove and user/project; original 3-OS genuine **production signed** 72/72 saved-install E2E; released CLI, protected CDN and website parity. Synthetic saved tests #900/#902/#903 are not publication authority. Never switch the legacy 0.1.89 feed or promote test keys.
