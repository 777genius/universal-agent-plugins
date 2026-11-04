# BUG-038 — ambiguous product alias selected another eligible signed GitHub release

## Exact real reproduction

[Live 30-plugin OpenCode CI](https://github.com/777genius/universal-agent-plugins/actions/runs/41845401854), original [JSON ZIP artifact 13338844327](https://github.com/777genius/universal-agent-plugins/actions/runs/41845401854/artifacts/13338844327), branch source `18e4c7dee5959bad6cfb9695a593c5406b1c7dde`. `opencode-plugin.json` has 30 separate source rows: **20 PASS / nine package-installed-native-unqualified / one incomplete-native-proof**. The one incomplete row is source `github`, identity `github`, index 8. Real `agentplugins add` exited zero; UAP `list` exited zero, and `opencode/opencode.json` contains a native identity-bearing projection. The row is non-PASS because it was bound to the wrong immutable *signed distribution*.

Published [Directory snapshot 43](https://github.com/777genius/universal-agent-plugins-registry/blob/directory-publication-ledger/registry/schemas/1/snapshots/00000000000000000043.json) names product `github` with default distribution `github/github`. That official distribution's active target list includes Codex/Cursor/Kiro, **not OpenCode**. A separate active `777genius/github-bridge` distribution, release sequence 2, explicitly supports OpenCode and has immutable revision `ae72693705e9567d43dca63351fcf63147884f4a`. Actual OpenCode `list` returned precisely that bridge distribution, revision, sequence and tree/manifest SHA-256. The frozen positive fixture incorrectly required `github/github` irrespective of target, so its evidence oracle rejected a legitimate different eligible release. This is a **test source selector ambiguity**, not a demonstrated production installer defect.

## Narrow implementation plan

1. In the **30-source positive fixture only**, replace ambiguous product alias `github` with explicit published distribution ID `777genius/github-bridge` (same declared manifest identity `github`). Use immutable release 2's published repository/path/SHA and tree/manifest hashes. Keep exactly 21 pinned signed packages, nine independent direct sources, 30 standalone Skills and 15 target IDs.
2. Extend only the existing manual-manifest identity mapping and the namespaced-distribution case in `scripts/live-install-audit.py`. Keep actual `matching_installations` proof tied to the *same* signed distribution, target client, release sequence, resolved revision and both immutable digests; never match on display name alone.
3. Add a no-network negative regression: official `github/github`, wrong revision/sequence/digest or wrong client must not satisfy the explicitly pinned bridge. Re-run full 30x30 live source ZIPs on final branch HEAD; no historical PASS is carried forward. User-facing default alias resolution and Registry policies remain **unchanged**.

## Two independent critical perspectives (author self-review, not a completed external review)

- **Policy/semantics critic:** Using the bridge as an explicitly named fixture does not mean the product default should change or that the official GitHub MCP server is unsupported globally. The official product default is eligible for other clients. This patch tests **one declared distribution** consistently across clients; it does not claim equivalent function, auth, activation, or MCP tool execution.
- **Security/evidence critic:** No fallback to a self-consistent `agentplugins list`, no invented source commit, no weakened LintAI/cgroup, no auto-promoted prepared status. Release source/digests come from the published immutable Directory; the negative test must reject alternate distribution and spoofed target. If the bridge is not eligible for a target, preserve the explicit non-PASS classification.

## Status

Fixture choice and strict source/target regression corrected in PR #698. In a second source-integrity review, the namespaced selector was found to exempt the directory `product_id` entirely. Tightened the bridge case to require `product_id=github` and added a forged-product regression, while keeping the existing chrome-devtools behavior unchanged. This is a source-identity guard, not evidence of a successful final-head installation. Still requires exact-new-head full live CI and independent security review before merge. No product adapter, Registry, feature flag or policy change.
