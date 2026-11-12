# Pi Directory v1 — real pinned Marketing Kit saved closure, test-only signing

Date: 2026-11-12. Stacked after [UAP #870](https://github.com/777genius/universal-agent-plugins/pull/870). Related [Registry #451](https://github.com/777genius/universal-agent-plugins-registry/pull/451).

## Problem and scope

Registry #451 has proven 70/70 standalone direct-source Pi user/project lifecycles on each of Linux, macOS, Windows. Its **published legacy** saved plan still correctly refuses two Marketing Kit → HubSpot CRM Pi scopes because the signed legacy HubSpot release grants no Pi. UAP #870's 3/3 green test-only Ed25519 cohort covers the four-node graph using **synthetic package contents**. This follow-up tests **actual immutable public package bytes**, not copies or guessed commits, under a distinct synthetic signed-v1 Pi source policy.

The four frozen inputs come from the current Registry #451 source, exact Git revision + tree + manifest digests:
- Marketing Kit 0.1.0: `777genius/universal-agent-plugins-registry@32b6c48d071aceadf90a030c9d69477fe1a6a65a//plugins/marketing-kit`
- Notion 0.1.0: `777genius/universal-agent-plugins@2ddbb99dd190c1792b79904f9875e6322bccd243//plugins/notion`
- HubSpot CRM 0.1.0: same UAP revision, `plugins/hubspot-crm`
- Playwright Bridge 0.0.80: same Registry revision, `plugins/playwright`, release sequence 2

The complete expected tuples are in the Go test. Do not use a branch tip or substitute newer package bytes. The actual Marketing Kit's unchanged `uap.toml` supplies all three transitive edges; the test constructs no fake dependency manifest.

## Implementation boundary

- Reuse #870's deterministic **test-only** Ed25519 Directory v1 strict signature verifier, in-memory verified client and isolated `FeedID`.
- Use production isolated GitHub source acquisition for immutable full commit SHA and digest verification against the public source tree.
- Resolve saved Pi user/project graphs and check all four immutable locks; exercise the composed user saved install+remove and four owned receipts with real package bytes.
- Run the exact real-public-source test on fresh Linux/macOS/Windows GitHub-hosted environments, no account credentials or model/MCP process. A specific test-only environment variable gates public network acquisition; production has no new config or feature flag.
- Independently run `TestPiProjectSignedV1SavedFourPackageLifecycle`: a disposable local TLS Git repository, second unrelated deterministic **test-only** Ed25519 key and the production CLI scoped composer exercise public `install` → `deps sync --locked` → `remove` over Pi User and Project A, with owned four-node receipts, real native Skills/MCP filesystem materialization and Project B/foreign state preservation. No real Pi process or account activation is claimed.
- A test-only synthetic policy/evidence record is **not trusted production materialization evidence**. Do not promote it to the real Registry or advertise 72/72. Exact signed modern production authorizations must be proved separately by editor-protected publication and released CLI native E2E.

## Review cycles

1. **Cryptography/provenance:** distinct test signing key only, production `directoryv1.VerifyBundle`, no imported env trust or unsigned fallback; actual hashes remain authoritative.
2. **Graph/architecture:** no duplicate dependency engine, real package `uap.toml`, real immutable source loader and saved coordinator; complete Pi four-node lock, no cross-feed borrowed source.
3. **Safety/UX:** disposable filesystem, no provider token, avoid OAuth/runtime claims; no production scope changes. Error is FAIL, never reported as a bypass.
4. **Acceptance:** run exact-current-head 3-OS CI, Go Core fast and Required; fix source, signature and any host errors; independent review before merge. Stacked merge order #870 then this PR, keep Registry #451 Draft pending genuine signed 72/72.

Out of scope: publishing modern Directory v1, changing trust origin or keys, CLI routing, HTTP OAuth, runtime MCP calls, and retrofitting CLI 0.1.89.
