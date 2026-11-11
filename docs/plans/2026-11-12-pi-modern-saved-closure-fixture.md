# Pi modern-v1 saved dependency closure: isolated source/lock qualification

Date: 2026-11-12. Related: Registry [#451](https://github.com/777genius/universal-agent-plugins-registry/pull/451), UAP [#868](https://github.com/777genius/universal-agent-plugins/pull/868).

## Purpose and boundary

The released CLI's historical signed feed has **no Pi target for HubSpot CRM**. Its Marketing Kit saved user/project plan fails honestly at 70/72; that negative result is not a flaky CI job. Direct `--no-save` Pi installs cannot substitute for a saved four-package graph.

This PR adds a **test-only preverified** modern feed-lineage fixture covering the four exact package roles (Marketing Kit, Notion, HubSpot CRM, Playwright Bridge), a real disposable Git history with immutable source/tree/manifest pins, and the production saved dependency resolver and locked acquisition on both User and Project. Both positive complete four-node plans and negative HubSpot-Pi-grant denial are asserted before any production mutation. All four pins must retain the same isolated test `FeedID`.

## Scope and review gates

- In scope: Go tests only, exact dependency graph, user/project targeting, immutable lock/source verification, child policy denial, no user environment or real credentials.
- Out of scope: production signing keys, publishing a feed, invented production evidence, a new routing flag, a CLI release, real native Pi activation, external OAuth, and claiming signed 72/72.
- Security review: no test feed identity can be obtained from untrusted package bytes; no fallback to the legacy signing identity; refused child cannot produce an installed receipt.
- Architecture review: reuse the existing Directory/lock/acquisition/installer seams rather than adding an installer or a schema.
- Scope review: no runtime/source changes; keep changes limited to one test and this plan.

The real signed-v1 Ed25519/cross-key/cache tests remain separately covered by #868. Follow-up acceptance must combine a disposable independently signed modern-v1 **test cohort** with the **real native managed saved** four-package installation on Linux/macOS/Windows, then an editor-approved production signer/bootstrap/origin/ledger, released modern routing, and post-publish 72/72 receipts. Neither this preverified fixture nor signed test-only fixtures may become publication evidence.

## Execution

Run `go test ./cli/internal/agentpluginscli -run '^TestPiModernFixture' -count=1 -v`, then exact-head Core fast, Required and package-manager CI. Resolve all failures and review before merge; do not use a green isolated source/lock fixture as a production eligibility claim.
