# Unified Microsoft APM risk-hardening follow-up — 2026-11-02

## One PR, four independent invariants

This draft **replaces PRs #670–#673**. It is stacked on #667 until the already-qualified base lands in main; afterwards retarget to main. Keep the original PRs open only until every original changed file is accounted for in this unified branch.

### Security — portable paths
- Reject Win32 COM¹/²/³ and LPT¹/²/³ aliases in one existing pathpolicy map, including nested paths and suffixes. Preserve other valid NFC Unicode.
- Prove policy, normalized source subpaths, packagedigest, actual ZIP/TAR extraction and valid ZIP/TAR cases on four OS runners.

### Security — native Git
- Keep safe.bareRepository=explicit and credential.interactive=never in existing command-scoped options alongside current SSH/TLS/protocol controls. Preserve agent/user credential helpers.
- Prove actual git subprocess precedence and native SSH transport, cross-platform Git tests.

### Safety — user/project environments
- Real public CLI for user, project A, project B; lock only one must preserve other scope's lock, manifest, inode and timestamps.
- Refuse contradictory scope and nonexistent project without writes; no-op lock stable.

### Safety — portable lock JSON
- Refuse duplicate object keys (root and nested; including JSON unicode-escaped equivalents) and invalid UTF-8 before pin materialization.
- Prove real locked install refuses tampering without network/receipt/native changes. Add the case to the exact E2E case catalogue *in addition to* #667's three new cases.

## CI and review qualification
1. Source of truth remains the existing layers: pathpolicy, native Git transport, conformance JSON grammar, CLI environment lock, live installer.
2. No VS Code work, extra parser, new state store, feature flag or new crosscutting layer. Only scoped changes and evidence.
3. One existing four-OS package-manager workflow runs the combined supplemental tests and new catalog case. Existing source transport native SSH E2E stays unchanged.
4. Critical reviews: source security/bypass, state/rollback/lifecycle, SOLID/Clean Architecture/DRY; fix all concrete observations.
5. Promote from Draft only after source review and base #667 merge. Require run+pass and full_qualification evidence on final HEAD in Linux, Windows, macOS arm64 and Intel; Source transports, Required, CodeQL and applicable CI must pass before squash merge.
6. Post-merge reverify main, avoid changing accepted ADRs. Never conflate source-only review with an independent model critic or a passing run.
