# Cursor/Qoder no-auth post-merge E2E consolidation

## Goal and measured baseline

Following merged #793, `.github/workflows/cursor-qoder-mcp-preflight.yml` starts two separate Linux jobs for every eligible PR and main push. Each job runs identical receipt-contract tests, installs Go 1.26.11, builds the same source installer and MCP probe, and then performs its **different** pinned Cursor or Qoder native lifecycle. Postmerge run [43631843040](https://github.com/777genius/universal-agent-plugins/actions/runs/43631843040) had two independent jobs queued with no result. Source execution in the earlier [43617742434](https://github.com/777genius/universal-agent-plugins/actions/runs/43617742434) took about a minute per client; most elapsed time was GitHub runner queue, not the client commands.

The same native checks are required on PR and after merge. Do not claim that a queued run is successful or that source-only MCP inventory proves model-driven `tools/call`.

## Optimization (bounded scope)

1. Collapse the two matrix jobs into **one** hosted Linux job for Cursor **and** Qoder, running the contracts and `setup-go` only once.
2. Add an explicitly no-credentials, two-client mode to the existing source runner. It first checks the exact checkout and builds **one** UAP installer and MCP probe using pinned Go; then copies the verified build bytes into **two fresh, independent private client workspaces**. Each client retains separate HOME, XDG, Git repositories A/B/outside, nonce, native executable acquisition/verification, MCP listing and ownership checks, install/removal, post-remove absence and foreign-byte negatives.
3. Fail the combined job if **either** client fails. Publish a single artifact containing **both separately named** `cursor/source-only-preflight.json` and `qoder/source-only-preflight.json` only on whole-job success. Both original per-client receipts must keep `native_tools_call_qualified=false`, `real_model_qualified=false`, `vendor_authentication_qualified=false`.
4. Make the existing native workflow apply to **any same-repository PR touching its owned files**, not only merged #793. Preserve narrow path filters for main pushes; no unrelated cross-repo/fork secret access. Keep only the latest relevant main-push run pending, replacing obsolete main validation. Leave optional credentialed model-call workflow unchanged.
5. Add deterministic regression tests: build-once / both-client dispatch, separate output roots, stale/tampered shared binaries rejected, proof of no use of batch mode for credentialed calls, and failures not silently marked as success.

## Cost and risk

Expected hosted-runner allocations per eligible event: **2 → 1**; Go toolchain setups: **2 → 1**; source installer/probe build pairs: **2 → 1**; contract runs: **2 → 1**. The two real native vendor lifecycles remain fully executed, so no unproven coverage is removed. This reduces runner resource consumption but cannot make a GitHub-hosted queue start sooner. Wall-clock runtime might rise slightly relative to two parallel jobs on idle runners.

Estimated production delta: ~40–100 lines Python; ~30–70 tests; workflow ~15–30 lines. Complexity 3/10, reliability target 9/10, confidence before CI 7/10.

## Critical review checks

- **Isolation:** sharing compiled bytes is safe; sharing client HOME, vendor binary, MCP state, or project directories is not. The shared copy must refuse symlink substitution and mismatched source hashes; copy only into fresh owned paths.
- **Evidence:** a successful first client and failed second client must fail the whole job and upload **no** combined PASS artifact; never treat an absent/skipped child as completed.
- **Source pin:** verify exact checkout/clean tree *before* building and again in each client; only in-process source-built bytes may be reused, never arbitrary external executable paths or an untrusted artifact cache.
- **Coverage:** batch mode is opt-in to credential-free preflight only; retain prior single-client authenticated runner semantics.
- **CI:** actual pinned native Cursor and Qoder installation/list/removal in combined job, contracts, applicable required checks, review, merge, and the resulting main-push combined job must PASS. Prior success is historical, not current-head qualification.

## Out of scope

No paid model sessions, native `tools/call` claim, cross-platform expansions, public installer changes, release, general Required CI redesign, or arbitrary extra workflows.
