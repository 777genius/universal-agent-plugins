# Pi full 72-scope independently keyed signed TEST-only cohort

Date: 2026-11-13. This stacked PR builds on #900 and merged #873; it is not a production signer, eligibility grant or released CLI routing.

## Minimal implementation

- Reuse the frozen, SHA-256-pinned 36-source Directory v1 snapshot and the independently keyed `piUpstreamTestOnlyClient` introduced in #900. Its two upstream materialization evidence records are **synthetic and isolated**; the original immutable zero-evidence test snapshot is not rewritten.
- Exercise all 36 real pinned Git sources × User/Project through the real scoped saved `install`, `deps sync --locked`, `remove` composer, with the existing native Pi MCP assertion for upstream roots and foreign scope preservation. Marketing Kit must produce the entire four-package dependency closure. Abort on skipped subtests, incomplete receipts, wrong feed or immutable root source.
- Add one dedicated 3-OS / 3-shard workflow and independent original-receipt aggregator. Bind every JSON receipt to exact test HEAD, original source digest, OS, shard and the 72 expected cases per OS. Explicitly and unconditionally state `synthetic_evidence=true`, `production_eligibility=false`, `production_published=false`.

## Independent review rounds

1. **Security:** Ed25519 test key, origin, feed/cache are distinct from historical trust and never exported to product routing. The artificial upstream grants cannot become publisher materialization evidence.
2. **Correctness:** All 216 test-only saved lifecycle cases must execute against exact Git source bytes, with full Marketing Kit saved dependency count and foreign User/Project scope preservation. No dry-run or `--no-save` substitution.
3. **CI/provenance:** Nine original positive jobs and an exact-head aggregator; queued, skipped, filtered callbacks or denied cases never count as passing. Review original logs and reports on Linux/macOS/Windows.
4. **Scope:** Only a test, dedicated workflow and this plan; no legacy signed reader, private key, production approval, v2, hidden flag, or feed publication change.

Even a green 72/72 **TEST-only** result does not qualify the protected modern-v1 publication: real upstream publisher evidence and approvals, production signer/origin/ledger/bootstrap/CAS/floor, released trusted CLI routing, and actual 3-OS published saved E2E remain mandatory.
