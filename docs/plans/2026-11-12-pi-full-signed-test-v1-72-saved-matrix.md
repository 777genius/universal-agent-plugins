# Pi modern Directory v1: 72 saved lifecycle source qualifications (TEST-ONLY)

Date: 2026-11-12. Base: [UAP #871](https://github.com/777genius/universal-agent-plugins/pull/871). Registry source: [Draft #451](https://github.com/777genius/universal-agent-plugins-registry/pull/451), exact immutable head `32b6c48d071aceadf90a030c9d69477fe1a6a65a`.

## Why this narrow addition

The earlier full native Pi E2E proves **70/70 standalone direct-source --no-save** lifecycles per OS, while the real published CLI 0.1.102 signed legacy planner genuinely refuses **Marketing Kit user/project (70/72)**. UAP #871 independently proves the four *real* Marketing Kit source digests and a second signed TEST-ONLY disposable Pi User/Project native file lifecycle. Neither proves all 36 real active releases in the same signed saved path.

This PR adds exactly one isolated source snapshot fixture, one Go integration test and one 3-OS, 3-shard workflow with an independent aggregator. No runtime, trust, CLI routing, website or Registry production code is changed.

## Immutable test inputs

- `cli/cmd/agentplugins/testdata/pi-full-modern-v1-test-only-snapshot.json` is generated once from the Registry #451 exact source and freezes **29 products, 36 active distributions/releases**. It excludes the suspended distributions; those cannot be counted as passed.
- Every local package source gets only that exact Registry commit SHA when its canonical source revision is null. Every already-pinned external source retains its original SHA, path, tree digest, manifest digest and component set.
- The original authenticated-source release policies are NOT modified in the Registry. Only this source fixture replaces policy targets with a synthetic Pi v1 user-scoped grant and preserves each existing package's `required` / `not_required` authentication classification. No invented project-scope v1 grant or `unknown => not_required` coercion.
- The Go test signs fixture bytes using a deterministic **test-only Ed25519 seed** held only in `_test.go`; verifies the exact bytes with production `directoryv1.VerifyBundle` and a distinct in-memory `FeedID` + embedded bootstrap. The source has no test-authorized real materialization evidence and is not a sign-ready production artifact.

## E2E acceptance

- The real CLI scoped composer and installer perform `install distribution --target pi`, `deps sync --locked`, `remove` separately in User and Project, using real immutable public GitHub package sources (no patched acquisition, no `--no-save`). Expected 1 owned receipt for standalone packages, **4 receipts for Marketing Kit with Notion, HubSpot CRM and Playwright Bridge**.
- Each case verifies exact test-only signing lineage on every persisted installation; the second unrelated Project B's foreign native configuration is unchanged after all operations. No native Pi process, login, OAuth, model session, browser call or MCP `tools/call` is executed; activation is NOT claimed.
- The 9 GitHub jobs (Ubuntu 24.04, macOS 15, Windows 2025 × shards 0–2) each cover 12 distinct distributions × 2 scopes and may emit a receipt **only after 24 actual passing subtests**. No optimistic report on a failing shard.
- Independent aggregator reads exactly 9 original JSON artifacts; verifies the pinned snapshot SHA256, exact original Registry source commit, OS/shard identity, nonproduction marker, 24 distinct cases per shard and **216 distinct OS/case proofs (72 on each OS)**. This is *signed TEST-ONLY source+installer* E2E; it must NOT be called production 72/72.
- A negative failure remains a genuine FAIL. Do not rerun queued jobs or weaken expected receipts, source integrity, ACL, snapshots, signatures or native scope isolation to force green.

## Review, scope, risks

1. **Architecture:** reuses production schema v1 verifier, HTTP Git acquisition, CLI scoped composer, installer, state and receipt inspection; no new installer or schema v2.
2. **Security:** only in-memory TEST signing key; distinct trust origin/cache/FeedID; exact public source identities; zero user tokens; no privileged production publish.
3. **Completeness:** 36 active × 2 scopes, 3 OS; independent aggregation with exact source/source-digest cross-check and suspended exclusions.
4. **Remaining production gates:** independently reviewed real Pi host activation/account evidence, protected editor-approved modern Ed25519 key, branch/ledger/origin/CAS, authentic signed 72/72, modern released CLI default feed routing, legacy reader 0.1.89 and CDN/site post-publish checks.

**Risk:** the full test may find genuine source, collision or Project implementation bugs; keep Draft, inspect and fix code rather than reclassify failures. Separate protected signer/approval cannot be inferred from a successful synthetic test fixture.
